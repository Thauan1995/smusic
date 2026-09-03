package ws

import (
	"encoding/json"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	"smusic/backend/internal/presence"
)

var (
	errConnClosed         = errors.New("presence/ws: connection closed")
	errOutboundBufferFull = errors.New("presence/ws: outbound buffer full")
)

// outboundBuffer is the small per-connection send buffer backend-go.md §3
// calls for ("cada conexão tem sua própria goroutine de escrita com buffer
// pequeno"). When full, Send drops the frame for THIS connection only and
// reports the failure to the caller (Hub counts it as a fanout drop) —
// never blocks, per backend-go.md §3's per-connection backpressure
// isolation.
const outboundBuffer = 8

// wireWriteWait bounds how long a single frame write may take before the
// connection is considered dead — protects the write pump from a
// half-open TCP connection hanging forever.
const wireWriteWait = 10 * time.Second

// conn implements presence.Conn over a real gorilla/websocket connection.
type conn struct {
	userID string
	ws     *websocket.Conn
	log    *slog.Logger

	out       chan presence.Frame
	closed    chan struct{}
	closedVal atomic.Bool // checked synchronously by Send, see its doc comment
	once      sync.Once
}

func newConn(userID string, wsConn *websocket.Conn, log *slog.Logger) *conn {
	return &conn{
		userID: userID,
		ws:     wsConn,
		log:    log,
		out:    make(chan presence.Frame, outboundBuffer),
		closed: make(chan struct{}),
	}
}

// UserID implements presence.Conn.
func (c *conn) UserID() string { return c.userID }

// Send implements presence.Conn: non-blocking, drops on a full buffer or a
// closed connection. closedVal is checked synchronously FIRST, rather than
// relying solely on a `select` across c.out/c.closed: once c.out has spare
// capacity, `case c.out <- f` and `case <-c.closed` can BOTH be ready
// simultaneously, and Go's select picks among ready cases pseudo-randomly —
// so a plain two-case select would let Send occasionally still enqueue a
// frame after close() (flaky, observed under `go test -race`), instead of
// deterministically reporting the connection as closed.
func (c *conn) Send(f presence.Frame) error {
	if c.closedVal.Load() {
		return errConnClosed
	}
	select {
	case c.out <- f:
		return nil
	case <-c.closed:
		// coverage:ignore — reachable only in the narrow race window
		// between the closedVal.Load() check above returning false and
		// this select executing; the closedVal check above already
		// deterministically covers the "already closed" case exercised by
		// this package's tests (TestConn_Close_Idempotent). Kept as a
		// second layer of defense, not because it's expected to fire in
		// practice.
		return errConnClosed
	default:
		return errOutboundBufferFull
	}
}

// writePump owns the connection's write side exclusively (gorilla/websocket
// connections aren't safe for concurrent writers) — backend-go.md §3's "1
// goroutine de leitura + 1 de escrita por conexão" pattern. It exits when
// the connection is closed or a write fails. Deliberately never ranges
// over c.out directly (which would require closing it, racing concurrent
// Send calls into a send-on-closed-channel panic) — it selects on both
// c.out and c.closed instead, and c.closed is the only channel ever
// closed.
func (c *conn) writePump() {
	for {
		select {
		case f := <-c.out:
			b, err := json.Marshal(toOutboundFrame(f))
			if err != nil {
				// coverage:ignore — toOutboundFrame always returns a plain
				// struct of strings/slices with no unmarshalable field
				// type; this cannot fail in practice.
				continue
			}
			_ = c.ws.SetWriteDeadline(timeNow().Add(wireWriteWait))
			if err := c.ws.WriteMessage(websocket.TextMessage, b); err != nil {
				c.log.Warn("presence ws: write failed, closing connection", "user_id", c.userID, "err", err)
				c.close()
				return
			}
		case <-c.closed:
			return
		}
	}
}

// close idempotently tears down the connection. Safe to call from multiple
// goroutines/multiple times.
func (c *conn) close() {
	c.once.Do(func() {
		c.closedVal.Store(true)
		close(c.closed)
		_ = c.ws.Close()
	})
}

// Close implements presence.Conn: it lets Hub (a different package, working
// only through the Conn interface) proactively evict this connection, e.g.
// when its owner's proximity consent is revoked or expires mid-session (see
// Hub.process's consent-error branch). Just an exported alias for the
// existing idempotent close — kept as two names because close() is also
// called internally by writePump on a write failure, a distinct code path
// that doesn't need (and shouldn't require) going through the exported
// interface method.
func (c *conn) Close() { c.close() }

// timeNow is indirected only so this file doesn't need to depend on
// clock.Clock (a whole interface) just for a write-deadline timestamp,
// which has no bearing on any privacy/business-logic invariant under test.
var timeNow = time.Now
