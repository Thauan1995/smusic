# Performance baseline — first measured run (2026-09-04)

Produced by `.vibeflow/specs/performance-benchmark-harness.md`'s harness
(`backend/loadtest/{http,presence}`). This is the **first time** any of
`backend-go.md` §6's numeric targets have been measured against a real,
running instance of this backend — previously they were architecture
commitments only, never validated.

**Where this ran**: a local stack on the same machine this session's
sandbox runs on (Postgres 16 + Redis 7 in Docker, `cmd/server` +
`cmd/presence-server` via `go run`, all on loopback). **Not** the public
`smusic-dev.duckdns.org` deployment — running an unbounded load test
against a home-lab machine that's also serving real traffic risked
degrading the live service or looking like a DDoS to anyone monitoring
it, so this first pass deliberately stayed local. Re-run against the real
deployment only with the user's explicit go-ahead (see the harness's own
risk note).

**Hardware caveat**: numbers below reflect this specific local-loopback
run (no real network latency, no CDN, no real audio bytes), not a
representative production environment. Read them as "the harness works
and these are the current numbers, on this hardware" — not as a final
verdict on the architecture's ability to hit these targets at scale on
real infrastructure.

## HTTP (`backend/loadtest/http`, 30 req/s for 10s each)

| Metric | Target (backend-go.md §6) | Measured p50 | Measured p95 | Result |
|---|---|---|---|---|
| Catalog search | ≤100ms p50 / ≤300ms p95 | 1.5ms | 1.7ms | PASS |
| Playback play (server-side portion) | ≤150ms p50 / ≤400ms p95 (proxy target — §6 has no direct number for this call alone) | 3.8ms | 4.5ms | PASS |
| Playback seek | ≤150ms p50 / ≤400ms p95 | 1.9ms | 2.3ms | PASS |

All three pass by a wide margin — expected on loopback with no real
network hop, no CDN, and a tiny (single-track) local catalog. This
confirms the harness works correctly and the server-side request path
itself isn't the bottleneck; it says nothing about the "time to first
audio" target as a whole, which depends heavily on the still-out-of-scope
real CDN/media-edge-service (`backend/README.md`'s "Playback" section —
`LocalResolver` serves a static test file, not real adaptive-bitrate
audio).

## Presence fanout (`backend/loadtest/presence`)

| N (concurrent WS clients) | Target | Measured (single sample) | Result |
|---|---|---|---|
| 10 | ≤2s p95 | 367µs | PASS |
| 50 | ≤2s p95 | 239µs | PASS |
| 200 | ≤2s p95 | 320µs | PASS |

**Methodology note (read before trusting these as "p95")**: each N above
is a *single* trigger event, not a statistical sample — the harness has
each already-connected "watcher" client heartbeat every 200ms and records
the first heartbeat response, from any watcher, that reflects one
"trigger" client's fresh update. A true p95 would require repeating the
trigger many times per N and computing a percentile across those
samples, which this first pass didn't do (documented as a real
limitation, not glossed over — see this spec's Anti-scope, which only
required "the harness working correctly and producing a real number,"
not full percentile rigor). The sub-millisecond results are consistent
with a heartbeat cycle that happened to already be in-flight when the
trigger fired, not proof that fanout is reliably sub-millisecond under
sustained load — re-running with multiple trigger rounds per N is a
natural follow-up if more rigorous numbers are needed later.

**≥5,000 concurrent connections / ≥2,000 updates/s per replica** (the
other backend-go.md §6 presence target) was **not attempted** here —
explicitly out of this pass's scope (a single home-lab machine was never
going to hit that regardless of the harness's correctness; see the
spec's anti-scope) and would also multiply the login-rate-limiter
friction noted below by another order of magnitude.

## Operational finding: login rate limiting makes multi-hundred-client local load tests non-trivial

Every simulated client in `backend/loadtest/presence` calls
`POST /v1/auth/signup` from the same source IP (the load-test process
itself). `security.md` §4's per-IP login rate limit
(`LOGIN_RATE_LIMIT_PER_MINUTE`, default 10/min) — correctly — throttles
this, since a real attacker mass-creating accounts from one IP is exactly
what this control exists to slow down. This first local run raised
`LOGIN_RATE_LIMIT_PER_MINUTE` on the *local test instance only* to get
past this for measurement purposes. **This must never be done against a
real deployment** — doing so would defeat a real security control rather
than work around a load-testing inconvenience. If N-in-the-hundreds
local load testing becomes a recurring need, the harness should instead
either accept pre-provisioned test accounts (created once, reused across
runs) or space out signups to respect the configured rate limit, rather
than requiring the target instance's security posture to be weakened.

## How to re-run

```bash
# from backend/, against a locally running smusic-core + presence-server:
go run ./loadtest/http -base-url http://localhost:8080 -database-url "$DATABASE_URL"
go run ./loadtest/presence -api-base-url http://localhost:8080 -ws-base-url ws://localhost:8081 -clients 10,50,200
```

See `backend/loadtest/{http,presence}/main.go` for flags (rate, duration,
client counts, custom target URLs — including, with explicit
authorization, the public deployment).
