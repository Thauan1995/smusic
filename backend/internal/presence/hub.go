package presence

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// Frame is the transport-agnostic shape of a server->client message. The
// real WS frame encoding lives in internal/presence/ws; Hub only knows
// about this Go value, which is exactly why Hub is unit-testable without a
// real network connection (backend-go.md §7's "testabilidade por design").
type Frame struct {
	Type          string // "nearby_update" | "resync_full" | "drain"
	Users         []NearbyResult
	ReconnectHint string
}

const (
	FrameNearbyUpdate = "nearby_update"
	FrameResyncFull   = "resync_full"
	FrameDrain        = "drain"
)

// Conn abstracts one presence WebSocket connection for Hub. The real
// implementation (internal/presence/ws) wraps a gorilla/websocket
// connection with its own bounded outbound buffer/writer goroutine per
// backend-go.md §3 ("cada conexão tem sua própria goroutine de escrita com
// buffer pequeno"); tests use a channel-based fake.
type Conn interface {
	UserID() string
	// Send delivers f to this connection's outbound buffer. It must never
	// block: if the connection's own buffer is full (slow client/bad
	// network), Send returns an error and the caller (a Hub worker) drops
	// that one frame for that one client only — backend-go.md §3's
	// per-connection backpressure isolation ("aquele cliente
	// especificamente perde updates... nunca afeta outros clientes").
	Send(Frame) error
	// Close tears down the underlying transport (idempotent, safe to call
	// from any goroutine). Used by Hub.process to proactively evict a
	// connection whose owner's proximity consent is no longer valid
	// (security.md §1.1: consent revoked, or its 6-month renewal window
	// lapsed, while a WS connection was already open) — without this, such
	// a connection would simply stop receiving frames forever (since
	// NearbyService gates every read on live consent) but stay open at the
	// transport level, invisible to the client and wasting a server-side
	// connection slot until the client eventually gives up or reconnects
	// on its own. See process's consent-error branch.
	Close()
}

// Metrics exposes the counters backend-go.md §3/§5 call for as
// observability of backpressure ("saturação de canais internos... sinal
// direto de backpressure"). Exported as plain atomics rather than
// Prometheus per this slice's documented deviation (no metrics/tracing
// wiring yet, consistent with Fatia 1's README).
type Metrics struct {
	IngestDropped atomic.Int64
	FanoutDropped atomic.Int64
	FramesSent    atomic.Int64
}

type jobKind int

const (
	jobUpdate jobKind = iota
	jobHeartbeat
)

type ingestJob struct {
	kind    jobKind
	conn    Conn
	lat     float64
	lon     float64
	trackID string
}

// Hub is backend-go.md §3's ingest/fanout pipeline: a bounded channel
// (layer-1 backpressure — reject, never block, when full), consumed by a
// fixed-size worker pool (never goroutine-per-update), each worker calling
// into NearbyService and then attempting a non-blocking Conn.Send (layer-2
// backpressure — drop-and-count per connection on a full outbound buffer).
// Layer-3 backpressure (per-user rate limiting of inbound update frames) is
// applied by the caller (internal/presence/ws) before Enqueue is even
// called, per backend-go.md §3's "antes mesmo de chegar ao pipeline."
type Hub struct {
	nearby        *NearbyService
	presenceTTL   time.Duration
	workers       int
	updateLimiter RateLimiter // layer-3 backpressure, backend-go.md §3; may be nil (no per-user WS-frame limit)

	updateFrameLimit  int           // configurable layer-3 limit; falls back to UpdateFrameLimit if <= 0
	updateFrameWindow time.Duration // configurable layer-3 window; falls back to UpdateFrameWindow if <= 0

	ingest  chan ingestJob
	metrics *Metrics

	mu    sync.Mutex
	conns map[string]Conn // userID -> connection, for drain broadcast on shutdown

	wg sync.WaitGroup
}

// NewHub constructs a Hub. ingestBuffer is the bounded channel capacity
// (backend-go.md §3: "buffered, com capacidade dimensionada por carga");
// workers is the fixed worker-pool size. updateLimiter, if non-nil, caps
// how often a single connection's "update" frames are accepted
// (backend-go.md §3's layer-3 backpressure, "na borda... antes mesmo de
// chegar ao pipeline") — internal/presence/ws checks AllowUpdateFrame
// before ever calling EnqueueUpdate.
func NewHub(nearby *NearbyService, presenceTTL time.Duration, workers, ingestBuffer int, updateLimiter RateLimiter) *Hub {
	return NewHubWithLimit(nearby, presenceTTL, workers, ingestBuffer, updateLimiter, 0, 0)
}

// NewHubWithLimit is NewHub plus explicit, configurable layer-3 limit/window
// (cmd/presence-server wires PRESENCE_UPDATE_RATE_LIMIT/PRESENCE_UPDATE_RATE_WINDOW
// here). updateFrameLimit <= 0 or updateFrameWindow <= 0 falls back to the
// package defaults (UpdateFrameLimit/UpdateFrameWindow) — this keeps NewHub's
// existing zero-value-friendly behavior for every caller (tests included)
// that doesn't care about tuning this specific knob.
func NewHubWithLimit(nearby *NearbyService, presenceTTL time.Duration, workers, ingestBuffer int, updateLimiter RateLimiter, updateFrameLimit int, updateFrameWindow time.Duration) *Hub {
	if updateFrameLimit <= 0 {
		updateFrameLimit = UpdateFrameLimit
	}
	if updateFrameWindow <= 0 {
		updateFrameWindow = UpdateFrameWindow
	}
	h := &Hub{
		nearby:            nearby,
		presenceTTL:       presenceTTL,
		workers:           workers,
		updateLimiter:     updateLimiter,
		updateFrameLimit:  updateFrameLimit,
		updateFrameWindow: updateFrameWindow,
		ingest:            make(chan ingestJob, ingestBuffer),
		metrics:           &Metrics{},
		conns:             make(map[string]Conn),
	}
	// wg.Add(workers) happens HERE, synchronously, before NewHubWithLimit
	// ever returns h to a caller — not inside Run() — deliberately. Run()
	// is typically started via `go hub.Run(ctx)` (its own goroutine) and
	// Shutdown() calls wg.Wait() from a goroutine it spawns itself; if
	// wg.Add ran inside Run() instead, a caller invoking Shutdown()
	// concurrently with (or shortly after) `go hub.Run(ctx)` could race
	// Add against Wait on an initially-zero counter — the WaitGroup docs'
	// own documented misuse ("calls with a positive delta that start when
	// the counter is zero must happen before a Wait"), confirmed here by
	// `go test -race` failing on exactly this pattern before this fix.
	// Setting the full worker count once, in the constructor — which
	// strictly happens-before Run or Shutdown can ever be called on h — for
	// a Hub whose value never changes after construction (Hub.workers isn't
	// re-configurable) removes the race by construction instead of by
	// synchronizing around it.
	h.wg.Add(h.workers)
	return h
}

// AllowUpdateFrame reports whether userID may submit another "update" frame
// right now, per the layer-3 per-connection rate limit. A nil updateLimiter
// (not configured) always allows.
func (h *Hub) AllowUpdateFrame(ctx context.Context, userID string) (bool, error) {
	if h.updateLimiter == nil {
		return true, nil
	}
	allowed, _, err := h.updateLimiter.Allow(ctx, updateFrameKey(userID), h.updateFrameLimit, h.updateFrameWindow)
	return allowed, err
}

// Metrics returns the Hub's live counters.
func (h *Hub) Metrics() *Metrics { return h.metrics }

// Register adds conn to the drain-broadcast registry. Call on WS connect,
// after the consent check has passed.
func (h *Hub) Register(conn Conn) {
	h.mu.Lock()
	h.conns[conn.UserID()] = conn
	h.mu.Unlock()
}

// Unregister removes conn from the registry and removes its owner from the
// live geo index (security.md §1.5: presence is tied to an active
// connection). Call on WS disconnect (any reason).
func (h *Hub) Unregister(ctx context.Context, conn Conn) {
	h.mu.Lock()
	if h.conns[conn.UserID()] == conn {
		delete(h.conns, conn.UserID())
	}
	h.mu.Unlock()
	_ = h.nearby.Disconnect(ctx, conn.UserID())
}

// EnqueueUpdate submits a client "update" frame for asynchronous
// processing. It never blocks: if the ingest channel is full, it returns
// ErrIngestSaturated immediately (layer-1 backpressure) instead of
// blocking the calling connection's read goroutine.
func (h *Hub) EnqueueUpdate(conn Conn, lat, lon float64, trackID string) error {
	return h.enqueue(ingestJob{kind: jobUpdate, conn: conn, lat: lat, lon: lon, trackID: trackID})
}

// EnqueueHeartbeat submits a client "heartbeat" frame.
func (h *Hub) EnqueueHeartbeat(conn Conn) error {
	return h.enqueue(ingestJob{kind: jobHeartbeat, conn: conn})
}

func (h *Hub) enqueue(job ingestJob) error {
	select {
	case h.ingest <- job:
		return nil
	default:
		h.metrics.IngestDropped.Add(1)
		return ErrIngestSaturated
	}
}

// Run starts the fixed-size worker pool and blocks until ctx is canceled
// and every in-flight job has been drained, per backend-go.md §3's
// worker-pool pattern (never goroutine-per-update). The WaitGroup's full
// worker count was already added in NewHubWithLimit (see its doc comment)
// — Run only spawns the goroutines and waits for them to each call
// wg.Done() on exit.
func (h *Hub) Run(ctx context.Context) {
	for i := 0; i < h.workers; i++ {
		go h.worker(ctx)
	}
	h.wg.Wait()
}

func (h *Hub) worker(ctx context.Context) {
	defer h.wg.Done()
	for {
		select {
		case job, open := <-h.ingest:
			if !open {
				return
			}
			h.process(ctx, job)
		case <-ctx.Done():
			return
		}
	}
}

func (h *Hub) process(ctx context.Context, job ingestJob) {
	var (
		results []NearbyResult
		err     error
	)
	switch job.kind {
	case jobUpdate:
		results, err = h.nearby.ApplyUpdate(ctx, job.conn.UserID(), job.lat, job.lon, job.trackID, h.presenceTTL)
	case jobHeartbeat:
		results, err = h.nearby.ApplyHeartbeat(ctx, job.conn.UserID(), h.presenceTTL)
	}
	if err != nil {
		// security.md §1.1: "revogação... interrompe o processamento
		// imediatamente" — and the same immediacy is expected whether
		// consent was explicitly revoked (REST DELETE /v1/presence/consent,
		// possibly from a different device/session than this open WS) or
		// simply lapsed past its 6-month renewal due date while this
		// connection sat idle-but-open. NearbyService already refuses to
		// process (or expose this user to anyone else's query) the instant
		// either happens — but left as a plain swallowed error, the
		// connection itself would just go silent forever: no more frames,
		// but no signal to the client either, and the socket stays
		// registered until the client gives up or reconnects unprompted.
		// Proactively drain (reusing the same graceful-reconnect frame
		// backend-go.md §3 already defines) and close it here instead, so a
		// well-behaved client reconnects immediately, hits the WS
		// handshake's own consent check (ws/handler.go's ServeHTTP), and
		// surfaces a clear consent_required/consent_expired error to the
		// user rather than silently receiving nothing.
		if errors.Is(err, ErrConsentRequired) || errors.Is(err, ErrConsentExpired) {
			hint := "consent_required"
			if errors.Is(err, ErrConsentExpired) {
				hint = "consent_expired"
			}
			h.deliver(job.conn, Frame{Type: FrameDrain, ReconnectHint: hint})
			job.conn.Close()
			return
		}
		// Every other processing error for one user's update is never
		// fatal to the pipeline (backend-go.md §3's "best-effort,
		// now-state" model) — it's simply not delivered this round; the
		// next heartbeat corrects it. Errors surface via the connection's
		// own error handling in internal/presence/ws if Conn.Send itself
		// is what's failing; NearbyService errors here are swallowed by
		// design (no durable side effect was left in an inconsistent
		// state: every NearbyService write is either fully applied or not
		// attempted).
		return
	}
	h.deliver(job.conn, Frame{Type: FrameNearbyUpdate, Users: results})
}

// deliver attempts a non-blocking send to conn, counting drops
// (layer-2 backpressure) without ever blocking the worker.
func (h *Hub) deliver(conn Conn, f Frame) {
	if err := conn.Send(f); err != nil {
		h.metrics.FanoutDropped.Add(1)
		return
	}
	h.metrics.FramesSent.Add(1)
}

// SendResync delivers an initial "resync_full" frame right after a
// connection is registered (backend-go.md §4's resync-on-reconnect
// contract). Since this implementation always computes and sends the full
// current nearby set on every update/heartbeat (a documented simplification
// — see README's "Desvios da spec" — rather than maintaining incremental
// delta state per connection), "resync_full" and "nearby_update" carry an
// identically-shaped payload; only the frame Type differs, which is enough
// for a client SDK that branches on Type per the protocol.
func (h *Hub) SendResync(ctx context.Context, conn Conn, lat, lon float64, trackID string) error {
	results, err := h.nearby.ApplyUpdate(ctx, conn.UserID(), lat, lon, trackID, h.presenceTTL)
	if err != nil {
		return err
	}
	h.deliver(conn, Frame{Type: FrameResyncFull, Users: results})
	return nil
}

// Shutdown implements backend-go.md §3's graceful-shutdown contract:
// broadcast "drain" to every connected client (so client SDKs can
// proactively reconnect to another replica), stop accepting new ingest,
// and wait for in-flight jobs to finish or ctx's deadline to pass —
// whichever comes first.
func (h *Hub) Shutdown(ctx context.Context, reconnectHint string) error {
	h.mu.Lock()
	conns := make([]Conn, 0, len(h.conns))
	for _, c := range h.conns {
		conns = append(conns, c)
	}
	h.mu.Unlock()

	for _, c := range conns {
		h.deliver(c, Frame{Type: FrameDrain, ReconnectHint: reconnectHint})
	}

	close(h.ingest)

	done := make(chan struct{})
	go func() {
		h.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
