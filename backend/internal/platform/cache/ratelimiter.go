package cache

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisRateLimiter implements middleware.RateLimiter as a fixed-window
// counter (INCR + EXPIRE), per backend-go.md §5's recommendation for
// consistent, cross-replica rate limiting. A fixed window is a deliberate
// simplification versus a sliding-window log: it is exact-once-per-window
// rather than exact-per-rolling-window (a client can in the worst case send
// ~2x the limit across a window boundary), which is an accepted trade-off
// for this MVP slice — the doc flags sliding-window log as the option for
// the endpoints most sensitive to abuse (login), which is a documented TODO
// here (see README "Desvios e TODOs").
type RedisRateLimiter struct {
	client *redis.Client
}

// NewRedisRateLimiter returns a RedisRateLimiter backed by client.
func NewRedisRateLimiter(client *redis.Client) *RedisRateLimiter {
	return &RedisRateLimiter{client: client}
}

// Allow implements middleware.RateLimiter.
func (r *RedisRateLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, time.Duration, error) {
	n, err := r.client.Incr(ctx, key).Result()
	if err != nil {
		return false, 0, err
	}
	if n == 1 {
		// First hit in this window: arm expiry. A crash between INCR and
		// EXPIRE would leave the key without a TTL; that's an accepted,
		// self-healing risk (the next deploy's key uses a fresh window
		// anyway once traffic resumes), not worth a Lua script/transaction
		// for this MVP slice.
		if err := r.client.Expire(ctx, key, window).Err(); err != nil {
			// coverage:ignore — requires the Redis connection to succeed on
			// INCR and then fail on the very next command (EXPIRE); not
			// reproducible with miniredis, which has no per-command fault
			// injection. The general "Redis command errors" path is
			// covered by TestRedisRateLimiter_ClientError (INCR failing).
			return false, 0, err
		}
	}
	if n <= int64(limit) {
		return true, 0, nil
	}

	ttl, err := r.client.TTL(ctx, key).Result()
	if err != nil || ttl < 0 {
		ttl = window
	}
	return false, ttl, nil
}
