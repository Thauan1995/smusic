package presence

import (
	"context"
	"time"
)

// RateLimiter is the same shape as internal/platform/middleware.RateLimiter
// and is satisfied as-is by internal/platform/cache.RedisRateLimiter — per
// the task's explicit hint, this package reuses that Fatia 1 primitive
// instead of inventing a second rate-limiting implementation. Declared
// locally (not imported from middleware) so this package doesn't need to
// depend on the HTTP-layer middleware package for an unrelated concept.
type RateLimiter interface {
	Allow(ctx context.Context, key string, limit int, window time.Duration) (allowed bool, retryAfter time.Duration, err error)
}

// Anti-triangulation limits, security.md §1.2/§1.8:
//   - "no máximo 1 consulta de proximidade por par de usuários a cada 30
//     segundos" — PairQueryLimit/PairQueryWindow.
//   - "no máximo 200 consultas totais por usuário por dia" —
//     DailyQueryLimit/DailyQueryWindow.
const (
	PairQueryLimit  = 1
	PairQueryWindow = 30 * time.Second

	DailyQueryLimit  = 200
	DailyQueryWindow = 24 * time.Hour
)

// pairKey is directional (requester -> target), not a sorted/symmetric
// pair key: the harm this limit defends against is a specific requester
// repeatedly observing a specific target to shrink the target's jitter
// noise by averaging. The reverse direction (target querying requester) is
// an entirely separate budget, since it's a different target being
// protected.
func pairKey(requesterID, targetID string) string {
	return "presence:rl:pair:" + requesterID + ":" + targetID
}

func dailyKey(requesterID string) string {
	return "presence:rl:daily:" + requesterID
}

// updateFrameKey is the presence-service's "borda" per-user rate limit on
// inbound update frames themselves (backend-go.md §3's three-layer
// backpressure, layer 3: "Rate limiting por usuário na borda... limita
// updates/segundo por client_id antes mesmo de chegar ao pipeline"). This
// is independent of the query-anti-triangulation limits above — it exists
// to protect the ingest pipeline from a single misbehaving/buggy client,
// not to protect a target's location privacy.
func updateFrameKey(userID string) string {
	return "presence:rl:updates:" + userID
}

const (
	// UpdateFrameLimit/Window: generous relative to the recommended 20-30s
	// client heartbeat cadence (backend-go.md §4), but present so a
	// buggy/malicious client spamming the WS can't flood the ingest
	// channel — the first line of the three-layer backpressure design.
	UpdateFrameLimit  = 12
	UpdateFrameWindow = time.Minute
)
