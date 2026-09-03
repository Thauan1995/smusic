// Package cache wires the shared Redis client (backend-go.md §5: catalog
// cache, search result cache, rate limiting, and — via
// internal/playback/redisstore — playback session state) and provides a
// Redis-backed RateLimiter.
package cache

import (
	"context"

	"github.com/redis/go-redis/v9"
)

// NewClient builds a go-redis client from an address (host:port) and
// optional password/db. This is thin connection wiring with no branching
// logic of its own — coverage:ignore per 00-overview.md §2's carve-out for
// wiring code; its behavior is exercised transitively by every test that
// uses miniredis through the *redis.Client it returns the same way
// production code does.
func NewClient(addr, password string, db int) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
}

// Ping checks connectivity, used at startup so the process fails fast
// instead of serving traffic against a broken cache dependency.
func Ping(ctx context.Context, client *redis.Client) error {
	return client.Ping(ctx).Err()
}
