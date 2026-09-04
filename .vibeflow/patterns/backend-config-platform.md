---
tags: [go, config, env-vars, redis, rate-limiting, platform]
modules: [backend/internal/platform/]
applies_to: [configs, services]
confidence: inferred
---
# Pattern: Env-var config + Redis fixed-window rate limiting

<!-- vibeflow:auto:start -->
## What
`internal/platform/config.Load(lookup Lookup)` is a pure function from a `func(key string) (string, bool)` lookup (so tests inject a fake map, never touching the real environment) to a `Config` struct, applying safe local-dev defaults for everything and erroring only on malformed (not missing) values. `internal/platform/cache.RedisRateLimiter` implements a simple INCR+EXPIRE fixed-window counter used for both login rate limiting and presence WS update-frame rate limiting.

## Where
`backend/internal/platform/config/config.go`, `backend/internal/platform/cache/ratelimiter.go`, consumed from `backend/cmd/server/main.go` and `backend/cmd/presence-server/main.go`.

## The Pattern
```go
type Lookup func(key string) (string, bool)

func Load(lookup Lookup) (Config, error) {
	cfg := Config{
		HTTPAddr: getOr(lookup, "HTTP_ADDR", ":8080"),
		...
	}
	var err error
	if cfg.RedisDB, err = getIntOr(lookup, "REDIS_DB", 0); err != nil {
		return Config{}, err
	}
	...
	return cfg, nil
}
```
Every secret-shaped field (`JWTEd25519SeedHex`, `PasswordPepperHex`) defaults to empty with a runtime `log.Warn` if unset (never a hardcoded fallback secret) — see `cmd/server/main.go`'s `jwtKeyPair`. `CORSAllowedOrigins` explicitly rejects `"*"` at parse time (`getCSVOrigins`) rather than allowing it and relying on a later check.

Rate limiter: INCR the key, EXPIRE only on the first hit (`n == 1`) to arm the window, compare against `limit`.
```go
func (r *RedisRateLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, time.Duration, error) {
	n, err := r.client.Incr(ctx, key).Result()
	...
	if n == 1 {
		r.client.Expire(ctx, key, window)
	}
	if n <= int64(limit) {
		return true, 0, nil
	}
	...
}
```

## Rules
- Config loading never panics or calls `os.Exit` — always returns `(Config, error)` so `main.go` decides how to fail.
- A genuinely missing env var always falls back to a safe default; only a *present but malformed* value (bad int/duration) is an error.
- Never hardcode a production secret as a Go default — secrets default to empty + a warning log, forcing explicit provisioning outside dev.
- The fixed-window rate limiter is a documented, accepted trade-off (allows up to ~2x limit across a window boundary) — not a bug; a sliding-window log is called out as future work for the most abuse-sensitive endpoints (login).

## Examples from this codebase
File: `backend/internal/platform/config/config.go:78-127`
File: `backend/internal/platform/cache/ratelimiter.go:29-58`
<!-- vibeflow:auto:end -->

## Anti-patterns
None found.
