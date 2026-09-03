package middleware

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"smusic/backend/internal/platform/httpx"
)

// RateLimiter answers "is one more request under key allowed within the
// current window". security.md §4/backend-go.md §5 call for Redis-backed
// fixed-window counters (INCR+EXPIRE) so the limit is consistent across
// replicas; see cache/redis_ratelimiter.go for that implementation. This
// interface lets the HTTP-layer middleware below be unit-tested without a
// real Redis.
type RateLimiter interface {
	// Allow reports whether the caller identified by key may proceed,
	// given at most limit calls per window. retryAfter is populated (best
	// effort) when the call is denied.
	Allow(ctx context.Context, key string, limit int, window time.Duration) (allowed bool, retryAfter time.Duration, err error)
}

// KeyFunc derives the rate-limit bucket key for a request (e.g. client IP
// for unauthenticated endpoints, user ID for authenticated ones).
type KeyFunc func(r *http.Request) string

// RateLimit returns middleware enforcing at most limit requests per window
// per KeyFunc-derived key. On denial it responds 429 with a Retry-After
// header (security.md §4: "nunca falha silenciosa"). If the limiter itself
// errors (e.g. Redis unavailable), the request is allowed through — a
// rate-limiter outage must never take down the critical path
// (backend-go.md §3's "best-effort" philosophy applied to this middleware).
func RateLimit(rl RateLimiter, keyFn KeyFunc, limit int, window time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := keyFn(r)
			allowed, retryAfter, err := rl.Allow(r.Context(), key, limit, window)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}
			if !allowed {
				w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())))
				httpx.WriteError(w, http.StatusTooManyRequests, "rate_limited", "too many requests, slow down")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ClientIPKey is a KeyFunc for unauthenticated endpoints (e.g. login,
// signup) that keys the bucket by remote IP, prefixed so it can share a
// Redis keyspace with other limiters without colliding.
func ClientIPKey(prefix string) KeyFunc {
	return func(r *http.Request) string {
		host := r.RemoteAddr
		if i := lastColon(host); i >= 0 {
			host = host[:i]
		}
		return prefix + ":" + host
	}
}

func lastColon(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == ':' {
			return i
		}
	}
	return -1
}
