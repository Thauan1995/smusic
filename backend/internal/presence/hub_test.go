package presence

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeConn implements Conn for Hub tests. Setting full=true makes Send
// always fail (simulating a saturated outbound buffer) so layer-2
// backpressure can be exercised deterministically.
type fakeConn struct {
	userID string
	mu     sync.Mutex
	frames []Frame
	full   bool
}

func newFakeConn(userID string) *fakeConn { return &fakeConn{userID: userID} }

func (c *fakeConn) UserID() string { return c.userID }

func (c *fakeConn) Send(f Frame) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.full {
		return errFakeConnFull
	}
	c.frames = append(c.frames, f)
	return nil
}

func (c *fakeConn) received() []Frame {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]Frame, len(c.frames))
	copy(out, c.frames)
	return out
}

var errFakeConnFull = assertErr("fake conn outbound buffer full")

type assertErr string

func (e assertErr) Error() string { return string(e) }

func newTestHub(t *testing.T, workers, ingestBuffer int, updateLimiter RateLimiter) (*Hub, *nearbyDeps) {
	t.Helper()
	nearby, d := newTestNearbyService(t, FixedJitterer{})
	hub := NewHub(nearby, time.Minute, workers, ingestBuffer, updateLimiter)
	return hub, d
}

func TestHub_EnqueueUpdate_ProcessedAndDelivered(t *testing.T) {
	hub, d := newTestHub(t, 2, 8, nil)
	d.settings.set(consentedSettings("a", d.clock))
	d.settings.set(consentedSettings("b", d.clock))
	require.NoError(t, d.geo.Upsert(context.Background(), PresenceEntry{UserID: "b", Position: origin}, time.Minute))

	ctx, cancel := context.WithCancel(context.Background())
	go hub.Run(ctx)
	defer cancel()

	c := newFakeConn("a")
	require.NoError(t, hub.EnqueueUpdate(c, originLat, originLon, ""))

	require.Eventually(t, func() bool { return len(c.received()) > 0 }, time.Second, time.Millisecond)
	frames := c.received()
	require.Len(t, frames, 1)
	assert.Equal(t, FrameNearbyUpdate, frames[0].Type)
	require.Len(t, frames[0].Users, 1)
	assert.Equal(t, "b", frames[0].Users[0].UserID)
}

func TestHub_EnqueueHeartbeat_Processed(t *testing.T) {
	hub, d := newTestHub(t, 2, 8, nil)
	d.settings.set(consentedSettings("a", d.clock))
	require.NoError(t, d.geo.Upsert(context.Background(), PresenceEntry{UserID: "a", Position: origin}, time.Minute))

	ctx, cancel := context.WithCancel(context.Background())
	go hub.Run(ctx)
	defer cancel()

	c := newFakeConn("a")
	require.NoError(t, hub.EnqueueHeartbeat(c))
	require.Eventually(t, func() bool { return hub.Metrics().FramesSent.Load() > 0 }, time.Second, time.Millisecond)
}

// TestHub_Enqueue_BackpressureLayer1_IngestSaturated verifies backend-go.md
// §3's required property: once the bounded ingest channel is full, Enqueue
// rejects explicitly and immediately instead of blocking.
func TestHub_Enqueue_BackpressureLayer1_IngestSaturated(t *testing.T) {
	nearby, d := newTestNearbyService(t, FixedJitterer{})
	d.settings.set(consentedSettings("a", d.clock))
	// 0 workers: nothing ever drains the channel, so it fills deterministically.
	hub := NewHub(nearby, time.Minute, 0, 1, nil)

	c := newFakeConn("a")
	require.NoError(t, hub.EnqueueUpdate(c, originLat, originLon, ""), "first update should fit in the buffer")

	err := hub.EnqueueUpdate(c, originLat, originLon, "")
	assert.ErrorIs(t, err, ErrIngestSaturated)
	assert.EqualValues(t, 1, hub.Metrics().IngestDropped.Load())
}

// TestHub_Deliver_BackpressureLayer2_FanoutDropped verifies a single slow
// connection's full outbound buffer only drops that connection's frame
// (counted in metrics) without blocking or crashing the worker.
func TestHub_Deliver_BackpressureLayer2_FanoutDropped(t *testing.T) {
	hub, d := newTestHub(t, 1, 8, nil)
	d.settings.set(consentedSettings("a", d.clock))

	ctx, cancel := context.WithCancel(context.Background())
	go hub.Run(ctx)
	defer cancel()

	c := newFakeConn("a")
	c.full = true
	require.NoError(t, hub.EnqueueUpdate(c, originLat, originLon, ""))

	require.Eventually(t, func() bool { return hub.Metrics().FanoutDropped.Load() > 0 }, time.Second, time.Millisecond)
	assert.Empty(t, c.received())
}

// TestHub_AllowUpdateFrame_Layer3Backpressure verifies the per-user WS
// update-frame rate limiter (layer 3) can be consulted independently of
// enqueueing, per backend-go.md §3's "antes mesmo de chegar ao pipeline".
func TestHub_AllowUpdateFrame_Layer3Backpressure(t *testing.T) {
	rl := newFakeRateLimiter()
	hub, _ := newTestHub(t, 1, 8, rl)

	allowed, err := hub.AllowUpdateFrame(context.Background(), "a")
	require.NoError(t, err)
	assert.True(t, allowed)

	rl.denyKeys[updateFrameKey("a")] = true
	allowed, err = hub.AllowUpdateFrame(context.Background(), "a")
	require.NoError(t, err)
	assert.False(t, allowed)
}

func TestHub_AllowUpdateFrame_NilLimiterAlwaysAllows(t *testing.T) {
	hub, _ := newTestHub(t, 1, 8, nil)
	allowed, err := hub.AllowUpdateFrame(context.Background(), "a")
	require.NoError(t, err)
	assert.True(t, allowed)
}

// TestNewHubWithLimit_ZeroFallsBackToPackageDefaults verifies
// NewHubWithLimit(..., 0, 0) behaves exactly like NewHub — the
// PRESENCE_UPDATE_RATE_LIMIT/PRESENCE_UPDATE_RATE_WINDOW knobs are optional
// tuning, not a required override.
func TestNewHubWithLimit_ZeroFallsBackToPackageDefaults(t *testing.T) {
	rl := newFakeRateLimiter()
	nearby, _ := newTestNearbyService(t, FixedJitterer{})
	hub := NewHubWithLimit(nearby, time.Minute, 1, 8, rl, 0, 0)
	assert.Equal(t, UpdateFrameLimit, hub.updateFrameLimit)
	assert.Equal(t, UpdateFrameWindow, hub.updateFrameWindow)
}

// TestNewHubWithLimit_ExplicitOverride verifies an explicit, positive
// limit/window is honored instead of the package defaults (cmd/presence-server
// wires PresenceUpdateRateLimit/PresenceUpdateRateWindow here).
func TestNewHubWithLimit_ExplicitOverride(t *testing.T) {
	rl := newFakeRateLimiter()
	nearby, _ := newTestNearbyService(t, FixedJitterer{})
	hub := NewHubWithLimit(nearby, time.Minute, 1, 8, rl, 5, 10*time.Second)
	assert.Equal(t, 5, hub.updateFrameLimit)
	assert.Equal(t, 10*time.Second, hub.updateFrameWindow)

	allowed, err := hub.AllowUpdateFrame(context.Background(), "a")
	require.NoError(t, err)
	assert.True(t, allowed)
	assert.Equal(t, 5, rl.lastLimit)
	assert.Equal(t, 10*time.Second, rl.lastWindow)
}

func TestHub_RegisterUnregister(t *testing.T) {
	// 0 workers deliberately: Run() is never started in this test (it only
	// exercises Register/Shutdown's drain-broadcast, not job processing),
	// and NewHubWithLimit's wg.Add(workers) (see its doc comment) would
	// otherwise leave Shutdown's internal wg.Wait() blocked forever waiting
	// for worker goroutines that are never spawned.
	hub, _ := newTestHub(t, 0, 8, nil)
	c := newFakeConn("a")
	hub.Register(c)

	err := hub.Shutdown(context.Background(), "bye")
	require.NoError(t, err)
	require.Len(t, c.received(), 1)
	assert.Equal(t, FrameDrain, c.received()[0].Type)
	assert.Equal(t, "bye", c.received()[0].ReconnectHint)
}

func TestHub_Unregister_RemovesFromIndexAndRegistry(t *testing.T) {
	// 0 workers: see TestHub_RegisterUnregister's comment above -- Run() is
	// never started here either.
	hub, d := newTestHub(t, 0, 8, nil)
	require.NoError(t, d.geo.Upsert(context.Background(), PresenceEntry{UserID: "a", Position: origin}, time.Minute))
	c := newFakeConn("a")
	hub.Register(c)

	hub.Unregister(context.Background(), c)

	_, ok, _ := d.geo.Touch(context.Background(), "a", time.Minute)
	assert.False(t, ok, "disconnecting must remove the user from the live index")

	// Shutdown after Unregister should not attempt to drain c again.
	require.NoError(t, hub.Shutdown(context.Background(), ""))
	assert.Empty(t, c.received())
}

func TestHub_SendResync(t *testing.T) {
	hub, d := newTestHub(t, 1, 8, nil)
	d.settings.set(consentedSettings("a", d.clock))
	d.settings.set(consentedSettings("b", d.clock))
	require.NoError(t, d.geo.Upsert(context.Background(), PresenceEntry{UserID: "b", Position: origin}, time.Minute))

	c := newFakeConn("a")
	require.NoError(t, hub.SendResync(context.Background(), c, originLat, originLon, ""))

	frames := c.received()
	require.Len(t, frames, 1)
	assert.Equal(t, FrameResyncFull, frames[0].Type)
	require.Len(t, frames[0].Users, 1)
}

func TestHub_SendResync_PropagatesError(t *testing.T) {
	hub, _ := newTestHub(t, 1, 8, nil) // "a" has no consent -> ApplyUpdate errors
	c := newFakeConn("a")
	err := hub.SendResync(context.Background(), c, originLat, originLon, "")
	assert.ErrorIs(t, err, ErrConsentRequired)
}

// TestHub_Shutdown_WaitsForInFlightWork checks Shutdown's contract: it
// closes ingest, waits for workers, and returns nil once drained within
// the deadline.
func TestHub_Shutdown_WaitsForInFlightWork(t *testing.T) {
	hub, d := newTestHub(t, 2, 8, nil)
	d.settings.set(consentedSettings("a", d.clock))

	ctx, cancel := context.WithCancel(context.Background())
	go hub.Run(ctx)
	defer cancel()

	c := newFakeConn("a")
	require.NoError(t, hub.EnqueueUpdate(c, originLat, originLon, ""))

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	assert.NoError(t, hub.Shutdown(shutdownCtx, ""))
}

// TestHub_Shutdown_DeadlineExceeded simulates a worker that never finishes
// (by holding the Hub's internal WaitGroup open directly, since Run isn't
// started here) and asserts Shutdown returns ctx's error once the deadline
// passes, rather than hanging forever.
func TestHub_Shutdown_DeadlineExceeded(t *testing.T) {
	hub, _ := newTestHub(t, 0, 1, nil)
	hub.wg.Add(1)
	t.Cleanup(hub.wg.Done)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	err := hub.Shutdown(shutdownCtx, "")
	assert.Error(t, err)
}

// TestHub_ConcurrentEnqueue_NoRaces stresses the pipeline with many
// concurrent producers; run with -race in CI to catch data races in the
// worker pool / connection registry.
func TestHub_ConcurrentEnqueue_NoRaces(t *testing.T) {
	hub, d := newTestHub(t, 8, 256, nil)
	d.settings.set(consentedSettings("a", d.clock))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c := newFakeConn("a")
			hub.Register(c)
			_ = hub.EnqueueUpdate(c, originLat, originLon, "")
			_ = hub.EnqueueHeartbeat(c)
			hub.Unregister(context.Background(), c)
		}()
	}
	wg.Wait()
}

func TestHub_Process_ErrorSwallowed_NoDelivery(t *testing.T) {
	hub, _ := newTestHub(t, 1, 8, nil) // "a" has no consent -> ApplyUpdate errors, nothing delivered

	ctx, cancel := context.WithCancel(context.Background())
	go hub.Run(ctx)
	defer cancel()

	c := newFakeConn("a")
	require.NoError(t, hub.EnqueueUpdate(c, originLat, originLon, ""))

	time.Sleep(20 * time.Millisecond)
	assert.Empty(t, c.received())
}
