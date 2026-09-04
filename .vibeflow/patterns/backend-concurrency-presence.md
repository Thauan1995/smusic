---
tags: [go, concurrency, goroutines, channels, backpressure, websocket, presence]
modules: [backend/internal/presence/, backend/cmd/presence-server/]
applies_to: [services, handlers]
confidence: inferred
---
# Pattern: Layered backpressure worker pool (presence ingest pipeline)

<!-- vibeflow:auto:start -->
## What
`internal/presence.Hub` is the concurrency core of the social-proximity feature (the product's stated competitive differentiator) and is genuinely idiomatic Go concurrency, not superficial goroutine-per-request code: a fixed-size worker pool reading off a single bounded channel, three explicit layers of backpressure (reject-never-block at every layer), non-blocking per-connection sends, and a documented `sync.WaitGroup` construction-ordering fix for a real `go test -race` failure.

## Where
`backend/internal/presence/hub.go` (the pipeline), `backend/internal/presence/ws/` (WS transport calling into it), `backend/cmd/presence-server/main.go` (process wiring + graceful shutdown).

## The Pattern
Three backpressure layers, in order, each documented at its own boundary:
1. **Layer 3** (`internal/presence/ws`, before `Enqueue` is even called): per-connection rate limit on inbound "update" frames via `Hub.AllowUpdateFrame` → `RateLimiter.Allow`.
2. **Layer 1** (`Hub.enqueue`): bounded channel, non-blocking `select`/`default` — a full ingest channel returns `ErrIngestSaturated` immediately rather than blocking the caller's read goroutine.
   ```go
   func (h *Hub) enqueue(job ingestJob) error {
       select {
       case h.ingest <- job:
           return nil
       default:
           h.metrics.IngestDropped.Add(1)
           return ErrIngestSaturated
       }
   }
   ```
3. **Layer 2** (`Hub.deliver`, called from each worker): non-blocking `Conn.Send`; on a full per-connection outbound buffer, only that one client drops the frame, isolated from every other connection.

Fixed worker pool (never goroutine-per-update):
```go
func (h *Hub) Run(ctx context.Context) {
	for i := 0; i < h.workers; i++ {
		go h.worker(ctx)
	}
	h.wg.Wait()
}
```
`wg.Add(h.workers)` happens synchronously in the constructor (`NewHubWithLimit`), not inside `Run()` — documented as a deliberate fix for a genuine `go test -race` failure where `Shutdown()` calling `wg.Wait()` could race a `wg.Add` that hadn't happened yet if `Run()` was started concurrently.

Graceful shutdown broadcasts a `drain` frame to every live connection, stops accepting new ingest (`close(h.ingest)`), and waits for in-flight jobs up to a context deadline — `Hub.Shutdown(ctx, reconnectHint)`.

Consent revoked/expired mid-connection is handled proactively: `Hub.process` detects `ErrConsentRequired`/`ErrConsentExpired` from the service call and evicts the connection (sends `drain` + `Conn.Close()`) instead of silently going quiet — so a well-behaved client reconnects and gets a clear error rather than a socket that just stops receiving frames.

## Rules
- Never block a worker or the calling goroutine on a full channel/buffer — always `select`/`default` + drop-and-count.
- Metrics are plain `atomic.Int64` counters (`IngestDropped`, `FanoutDropped`, `FramesSent`), not a metrics library — consistent with the project's documented "no Prometheus/tracing yet" deviation.
- `Hub` is unit-tested via a `Conn` interface + channel-based fake, no real network connection required (`backend-go.md` §7 "testabilidade por design").
- Any future concurrent pipeline in this codebase should follow the same shape: bounded channel → fixed worker pool → non-blocking per-target delivery → explicit metrics for every drop point.

## Examples from this codebase
File: `backend/internal/presence/hub.go:141-158` (WaitGroup construction-ordering fix, full rationale in the doc comment)
File: `backend/internal/presence/hub.go:245-295` (`process` — consent-revocation eviction)
File: `backend/internal/presence/hub_test.go` (751 lines exercising the pipeline with real goroutines)
<!-- vibeflow:auto:end -->

## Anti-patterns
None found — this is the most carefully engineered part of the backend. The one caveat: `internal/presence/ws/handler.go`'s `handleInbound`/`bearerToken` (the code that feeds `Hub.Enqueue*` from the actual WebSocket) is at 0% test coverage (see `backend-testing.md`'s Anti-patterns) — the pipeline itself is well-tested, but its real entry point is not.
