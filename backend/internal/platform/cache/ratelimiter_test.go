package cache

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func newTestClient(t *testing.T) (*redis.Client, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return client, mr
}

func TestRedisRateLimiter_AllowsUpToLimit(t *testing.T) {
	client, _ := newTestClient(t)
	rl := NewRedisRateLimiter(client)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		allowed, _, err := rl.Allow(ctx, "k", 3, time.Minute)
		require.NoError(t, err)
		require.True(t, allowed, "call %d should be allowed", i)
	}

	allowed, retryAfter, err := rl.Allow(ctx, "k", 3, time.Minute)
	require.NoError(t, err)
	require.False(t, allowed)
	require.Greater(t, retryAfter, time.Duration(0))
}

func TestRedisRateLimiter_WindowResets(t *testing.T) {
	client, mr := newTestClient(t)
	rl := NewRedisRateLimiter(client)
	ctx := context.Background()

	allowed, _, err := rl.Allow(ctx, "k", 1, time.Second)
	require.NoError(t, err)
	require.True(t, allowed)

	allowed, _, err = rl.Allow(ctx, "k", 1, time.Second)
	require.NoError(t, err)
	require.False(t, allowed)

	mr.FastForward(2 * time.Second)

	allowed, _, err = rl.Allow(ctx, "k", 1, time.Second)
	require.NoError(t, err)
	require.True(t, allowed, "window should have reset after TTL expiry")
}

func TestRedisRateLimiter_SeparateKeysIndependent(t *testing.T) {
	client, _ := newTestClient(t)
	rl := NewRedisRateLimiter(client)
	ctx := context.Background()

	allowed, _, err := rl.Allow(ctx, "a", 1, time.Minute)
	require.NoError(t, err)
	require.True(t, allowed)

	allowed, _, err = rl.Allow(ctx, "b", 1, time.Minute)
	require.NoError(t, err)
	require.True(t, allowed, "different key must have its own counter")
}

func TestRedisRateLimiter_TTLFallbackWhenNoExpiry(t *testing.T) {
	client, _ := newTestClient(t)
	rl := NewRedisRateLimiter(client)
	ctx := context.Background()

	// Seed the counter without a TTL, simulating the documented accepted
	// race (a crash between INCR and EXPIRE on the very first hit).
	require.NoError(t, client.Set(ctx, "k", 5, 0).Err())

	allowed, retryAfter, err := rl.Allow(ctx, "k", 1, 2*time.Second)
	require.NoError(t, err)
	require.False(t, allowed)
	require.Equal(t, 2*time.Second, retryAfter, "must fall back to window when the key has no TTL")
}

func TestRedisRateLimiter_ClientError(t *testing.T) {
	client, mr := newTestClient(t)
	rl := NewRedisRateLimiter(client)
	mr.Close()

	_, _, err := rl.Allow(context.Background(), "k", 1, time.Minute)
	require.Error(t, err)
}

func TestPing(t *testing.T) {
	client, _ := newTestClient(t)
	require.NoError(t, Ping(context.Background(), client))
}

func TestNewClient(t *testing.T) {
	mr := miniredis.RunT(t)
	c := NewClient(mr.Addr(), "", 0)
	t.Cleanup(func() { _ = c.Close() })
	require.NoError(t, Ping(context.Background(), c))
}
