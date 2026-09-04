---
tags: [go, testing, testify, fakes, coverage, unit-tests]
modules: [backend/internal/]
applies_to: [tests]
confidence: inferred
---
# Pattern: Hand-written fakes + testify, hermetic unit tests

<!-- vibeflow:auto:start -->
## What
Business logic (`service.go`) is unit-tested against hand-written in-memory fakes implementing the module's `repo.go` interfaces — never a real Postgres/Redis — using `testify/assert` + `testify/require`. `internal/platform/clock.Frozen` gives deterministic time; `internal/platform/idgen.NewSequential("id")` gives deterministic, inspectable IDs in tests (vs. `idgen.UUIDv7` in production). Redis-backed adapters (`presence/redisstore`, `playback/redisstore`, rate limiters) are tested against `miniredis` instead of fakes, since they need to exercise real Redis command semantics (GEOADD/GEOSEARCH, INCR/EXPIRE races). Postgres adapters (`*/postgres/repo.go`) are excluded from the unit-coverage target and covered separately by build-tagged `integration` tests against a real database.

## Where
Every `*_test.go` colocated with the file it tests, same package (white-box testing, not `_test` package suffix). `backend/internal/presence/hub_test.go` and `nearby_service_test.go` are the largest/most instructive (751 and 391 lines) — they test the concurrency pipeline with real goroutines and channels, not mocked-out.

## The Pattern
```go
type deps struct {
	users         *fakeUserRepo
	...
	clock         *clock.Frozen
}

func newTestService(t *testing.T) (*Service, *deps) {
	t.Helper()
	clk := clock.NewFrozen(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	d := &deps{ users: newFakeUserRepo(), ..., clock: clk }
	svc := NewService(d.users, ..., clk, idgen.NewSequential("id"), refreshTTL)
	return svc, d
}

func TestSignUp_Success(t *testing.T) {
	svc, d := newTestService(t)
	result, err := svc.SignUp(context.Background(), SignUpInput{...})
	require.NoError(t, err)
	assert.NotEmpty(t, result.UserID)
	...
}
```
Coverage-exclusion convention: an inline `// coverage:ignore — <reason>` comment on any branch deliberately excluded from the 100%-coverage target, per `docs/architecture/00-overview.md` §2's rule that "every exclusion needs an explicit, reviewable justification, never silent." Typical reasons found: an error branch that requires a fault-injection capability `miniredis` doesn't support (e.g. "Redis succeeds on command A then fails on the immediately-following command B"), or a `json.Marshal` call on a type that structurally cannot fail to marshal.

## Rules
- `require` for setup/fatal assertions (stop the test), `assert` for the actual expectations (collect all failures).
- Table-driven tests for input-validation sweeps; one `Test<Method>_<Scenario>` function per behavior otherwise.
- Never mock `clock.Clock` with `time.Now()` in a test — always `clock.Frozen`, so time-dependent assertions (TTL expiry, consent renewal windows) are exact, not flaky.
- Any deliberately-uncovered line MUST carry a `// coverage:ignore — <reason>` comment; an exclusion without one is a policy violation per the project's own stated 100%-coverage bar.
- `internal/*/postgres/*_test.go` (real DB) are build-tagged `integration` and excluded from the default `go test ./...` / coverage run — run separately.

## Examples from this codebase
File: `backend/internal/auth/service_test.go:31-47`
File: `backend/internal/presence/redisstore/geoindex.go:78-95` (coverage:ignore convention, 3 occurrences)
File: `backend/internal/platform/cache/ratelimiter.go:40-47` (same convention)
<!-- vibeflow:auto:end -->

## Anti-patterns
- Total unit coverage is **72.5%** (`backend/coverage.out`, `go tool cover -func`), not the project's stated 100% target. `internal/presence/ws/handler.go`'s `handleInbound` and `bearerToken` functions are at **0%** — the WebSocket transport layer (the newest, most complex code in the repo) is untested. This is the single largest gap against the project's own stop-criterion ("cobertura de testes = 100%") and should be the top item in any coverage-closing spec.
