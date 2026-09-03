# smusic backend — Fatia 1 (auth, catalog, library, playback-state)

Go monolith implementing the first vertical slice of smusic per
`docs/architecture/`: **auth, catalog, library, playback-state** — no
proximity/presence feature, no `presence-service`/`media-edge-service`
extraction (those are Fatia 2). Architecture, decisions and every deviation
from the four planning docs are documented inline in the code (search for
`TODO` and doc comments referencing `backend-go.md`, `data-architecture.md`,
`security.md`, `00-overview.md`) and summarized in **[Desvios da
spec](#desvios-da-spec-e-justificativas)** below.

## Stack

- Go **≥ 1.25.14** (see "Known Go-toolchain vulnerabilities" below —
  `go.mod`'s `go` directive pins this; `GOTOOLCHAIN=auto` (the Go default)
  auto-downloads a matching toolchain if the locally installed `go` is
  older, so CI/deploy always builds with a patched toolchain even if the
  local machine hasn't been upgraded)
- **Postgres 16** (via `pgx/v5`) — users, catalog, playlists, history (see
  `migrations/`)
- **Redis 7** (via `go-redis/v9`) — playback session state, login rate
  limiting
- **chi** — HTTP router
- **golang-migrate** — schema migrations (plain numbered `.up.sql`/`.down.sql`
  files; chosen over `atlas` because this project doesn't need
  declarative-schema diffing yet, and plain SQL is easier to review in a PR)
- **Argon2id** (`golang.org/x/crypto/argon2`) — password hashing
- **Ed25519 JWT** (`golang-jwt/jwt/v5`) — access tokens
- **testify** + **miniredis** — testing

## Running locally

```bash
cp .env.example .env   # edit if needed; defaults work with docker-compose
docker-compose up -d   # Postgres + Redis
export $(cat .env | grep -v '^#' | xargs)   # or use direnv/dotenv of choice
go run ./cmd/migrate up
go run ./cmd/server
```

The server listens on `:8080` (`HTTP_ADDR`). `GET /healthz` returns `200 ok`.

> **Known environment issue, not a project defect:** this repo was built in
> an environment where the installed `docker-compose` (v1.29.2, Python) is
> broken against the local Docker daemon's API version
> (`Not supported URL scheme http+docker` — a known `docker-py`/`requests`
> version conflict) and the `docker compose` v2 plugin isn't installed. The
> `docker-compose.yml` in this repo is valid and was verified by running
> Postgres/Redis directly via `docker run` with the same images/ports it
> declares (see "What was actually verified" below). If you hit the same
> error, install the `docker compose` v2 plugin or fix the `docker-compose`
> v1 Python environment; it's unrelated to this codebase.

### Makefile shortcuts

```bash
make up            # docker-compose up -d
make migrate-up    # go run ./cmd/migrate up
make run           # go run ./cmd/server
make test-race     # go test -race ./...
make cover         # coverage report
make vet lint      # go vet + staticcheck (if installed)
```

## Configuration

All settings are environment variables with safe local-dev defaults; see
`internal/platform/config/config.go` for the full list and
`.env.example` for a template. Notable ones:

| Var | Default | Notes |
|---|---|---|
| `DATABASE_URL` | `postgres://smusic:smusic@localhost:5432/smusic?sslmode=disable` | |
| `REDIS_ADDR` | `localhost:6379` | |
| `JWT_ED25519_SEED_HEX` | unset → ephemeral key generated at boot | **Set this in any persistent environment** — an ephemeral key means every restart invalidates all access tokens. Generate with `openssl rand -hex 32`. Production key management via Vault/KMS is a documented TODO (security.md §3). |
| `PASSWORD_PEPPER_HEX` | unset → no pepper | Same as above: set via `openssl rand -hex 32` outside dev. |
| `MEDIA_BASE_URL` / `MEDIA_SIGNING_KEY` | local `/media` handler | See "Media / playback" below. |
| `LOGIN_RATE_LIMIT_PER_MINUTE` | `10` | Per-IP, applied to signup+login only. |
| `CORS_ALLOWED_ORIGINS` | unset → CORS disabled | Comma-separated browser origins allowed to call this API cross-origin (e.g. `http://localhost:5173,https://app.smusic.example`). See "CORS" below. |

## Module layout

```
internal/
  <domain>/
    domain.go     entities + sentinel errors
    repo.go       repository interfaces (the only way another module may
                  reach this one's data — never direct table access)
    service.go    business logic — depends only on interfaces, unit-tested
                  with in-memory fakes, no real Postgres/Redis needed
    postgres/     real repository implementation (pgx) — integration tier
    api/          thin chi handlers: decode → call service → map response
  auth/           signup, login, refresh, logout, OAuth stub, MFA stub
  catalog/        artists/albums/tracks CRUD + pg_trgm search
  library/        playlists + favorites ("Músicas Curtidas")
  playback/       session state (play/pause/seek/next/queue), Redis-backed
  platform/       shared, cross-cutting: config, clock, idgen, httpx,
                  middleware (auth, rate limit), cache (Redis wiring +
                  rate limiter), dbx (Postgres pool wiring), logging
cmd/
  server/         wires everything together, runs the HTTP server
  migrate/        thin CLI over golang-migrate
migrations/       0001_init.{up,down}.sql
testdata/media/   placeholder local "CDN" test asset (see below)
```

Cross-module rule enforced throughout (backend-go.md §1): a module never
imports another module's `postgres` package or touches its tables. Where one
module needs a fact owned by another (e.g. "does this track exist?"), it
depends on a small interface (`library.TrackChecker`,
`playback.TrackChecker`) that `catalog.Service` happens to satisfy
structurally — wired together only in `cmd/server/main.go`.

## Auth (security.md §2)

- Argon2id, pepper via HMAC-SHA256 pre-hash (see
  `internal/auth/password/argon2.go` for the parameter/rationale doc).
- Access token: short-lived (`15m` default) JWT, Ed25519-signed.
- Refresh token: opaque, 32 random bytes, stored **hashed** (SHA-256) in
  Postgres, **rotated on every use**. Presenting an already-rotated
  (stale) refresh token is treated as theft: every refresh token for that
  user is revoked immediately (`ErrRefreshTokenReused`) — this resolves
  backend-go.md's open question on reuse detection.
- `POST /v1/auth/logout` revokes one refresh token (idempotent);
  `POST /v1/auth/logout-all` (authenticated) revokes every session — an
  addition beyond the sketched contract, needed for the "logout de todos os
  dispositivos" requirement in security.md §2.
- OAuth: `internal/auth/oauth` defines the `Verifier` interface and ships a
  `StubVerifier` that returns `ErrNotImplemented`. The full signup/login
  flow (find-or-create by provider+subject, identity linking, session
  issuance) is wired end-to-end in `auth.Service.LoginWithOAuth` — swapping
  in real Google/Apple JWKS verification is the only change needed.
- MFA: `internal/auth/mfa` ships the `Challenger` interface and a
  `NoopChallenger`, per the task's explicit instruction (no feature in this
  slice needs step-up auth yet — proximity, which does, is Fatia 2).

## CORS

Added after an independent audit noted that no CORS policy existed at all
— every prior verification of this API was via `curl`, which isn't
subject to the browser's same-origin policy, so a browser-hosted client
(the Flutter **web** target) had never actually been proven able to call
this API cross-origin. `net/http` + `chi` add no CORS headers on their
own, so with nothing configured every cross-origin browser request was
silently blocked by the browser (server logs would show nothing wrong;
the request would just never leave the browser as a usable response).

**Policy** (`cmd/server/main.go`'s `buildRouter`, via `github.com/go-chi/cors`):

- **`AllowedOrigins`**: an explicit allowlist from `CORS_ALLOWED_ORIGINS`
  (comma-separated), parsed in `internal/platform/config`. **Empty by
  default** — CORS is opt-in, not opt-out: with the var unset, no
  cross-origin browser request is allowed (server-to-server calls, `curl`,
  mobile app HTTP clients, etc. are entirely unaffected — CORS is a
  browser-enforced restriction on `fetch`/`XHR`, never a server-side
  one). Set it explicitly per environment, e.g. for local Flutter web dev:
  `CORS_ALLOWED_ORIGINS=http://localhost:PORT` (match whatever port
  `flutter run -d chrome` picked, or pin one with `--web-port`).
  **`*` is rejected at config-load time** (`config.Load` returns an
  error) — the audit's instruction to never wildcard, kept even though
  (see `AllowCredentials` below) it isn't strictly forced by
  cookie/credential use today: it's cheap defense-in-depth against a
  future cookie-based flow being added without anyone revisiting this
  file.
- **`AllowCredentials: false`**. Verified by reading
  `internal/auth/api/handlers.go` and
  `internal/platform/middleware/auth.go`: this API sets **no cookies**
  anywhere — access and refresh tokens are issued in the JSON response
  body (`authResponse`) and the client sends the access token back as
  `Authorization: Bearer <token>` (checked in
  `middleware.RequireAuth`). A plain custom header does not require
  `fetch(..., {credentials: 'include'})` the way cookies/HTTP-auth/TLS
  certs do, so the browser's CORS "credentialed request" mode — which is
  the thing that would forbid a wildcard origin outright — doesn't apply
  here. If a cookie-based flow (e.g. an httpOnly refresh-token cookie) is
  added later, this must flip to `true` **and** `AllowedOrigins` must stay
  a strict non-wildcard allowlist (already the case).
- **`AllowedMethods`**: `GET, POST, PUT, PATCH, DELETE, OPTIONS` — covers
  every verb actually used across auth/catalog/library/playback's routes.
- **`AllowedHeaders`**: `Accept, Content-Type, Authorization` — the only
  headers any client sends today (`Authorization` for bearer auth,
  `Content-Type` for the JSON bodies `httpx.DecodeJSON` expects).
- **`MaxAge: 300`** — caches preflight (`OPTIONS`) responses for 5
  minutes, a conventional default that cuts preflight round-trips without
  stale-caching a policy that's expected to change per-deploy.

## Catalog (data-architecture.md §1.2, §5.4)

Minimal CRUD for artists/albums/tracks (enough to populate and list) plus
search. Search uses **Postgres `pg_trgm`** as the fallback engine — a
dedicated search engine (Meilisearch, per data-architecture.md §5.4) is a
documented TODO in `internal/catalog/domain.go`'s package doc; swapping it
in only requires a new `TrackRepository`/`AlbumRepository`/
`ArtistRepository` implementation, the service layer's contract doesn't
change. Search is **cursor-paginated** (keyset, not offset), ordered by
`(title/name, id)` ascending.

## Library (data-architecture.md §1.3)

Playlists (create/list, add/remove/list tracks) and favorites
(save/unsave/list). Playlist ordering uses fractional positioning
(`position NUMERIC`, gap of 1024) so inserting anywhere never requires
reindexing the rest of the playlist. Only the playlist owner may mutate it
in this slice; collaborative-playlist editing by non-owners is a documented
TODO (`ErrNotOwner`/visibility check lives in `library.Service`).

## Playback (backend-go.md §4-§5)

Session state (`track_id`, `position_ms`, `is_playing`, `queue`) lives
**only in Redis** (`internal/playback/redisstore`), never Postgres — per
backend-go.md §5, losing it is an acceptable worst case (client resyncs).

**Media delivery is faked for this slice, on purpose** (explicit task
instruction): `internal/playback/media.LocalResolver` signs a URL
(HMAC-SHA256 over track+expiry, same shape a real CDN-signed URL would
have) pointing at `GET /media/sample.mp3`, served locally by
`cmd/server` from `testdata/media/`. Swapping in a real
`media-edge-service` + CDN later only means implementing
`playback.MediaURLResolver` differently — the interface and every caller
stay the same. See `internal/playback/media/resolver.go`'s package doc for
the full TODO.

## Desvios da spec e justificativas

Every deviation is also documented inline at its exact location; this is
the consolidated list.

1. **REST/JSON via `chi` only, no gRPC/gRPC-Gateway.** backend-go.md §2
   specifies gRPC as the source-of-truth contract with gRPC-Gateway
   generating the REST surface, for the eventual multi-process topology
   (`presence-service`, `media-edge-service`). Fatia 1 is explicitly a
   single-process monolith with no internal service-to-service calls yet —
   introducing a gRPC/protobuf toolchain with no second process to talk to
   would be pure overhead. Endpoints match backend-go.md §4's REST shape
   directly.
2. **`refresh_tokens` table added** — not in data-architecture.md's table
   list, but required by security.md §2's revocable-opaque-refresh-token
   model. See `migrations/0001_init.up.sql`'s header comment.
3. **`tracks.audio_asset_id` FK dropped.** data-architecture.md §1.2's prose
   mentions both `tracks.audio_asset_id` and `track_audio_assets.track_id`,
   which is circular. Implemented only the latter (the real 1:N,
   "many assets per track" — the actual relationship data-architecture.md
   itself explains). The playback-facing "which asset to use" question is
   answered by `quality_tier` at query time, not a hardcoded default FK.
4. **Auth write endpoints extended:** `POST /v1/auth/logout-all` and
   full-flow OAuth login (`{oauth_provider, oauth_token}` on the existing
   signup/login endpoints) aren't in backend-go.md §4's sketch table but
   are needed to satisfy security.md §2's session-revocation and OAuth
   requirements respectively.
5. **Catalog write endpoints added:** backend-go.md §4 only sketches
   catalog's *read* surface (search, get track/album). The task explicitly
   asks for "CRUD mínimo... o suficiente para popular e listar", so
   `POST /v1/catalog/{artists,albums,tracks}` were added, gated behind
   authentication (any authenticated user) as a minimal guard — a real
   admin/ingest role is a TODO (role-based authz doesn't exist yet).
6. **Library endpoints added:** `GET /v1/library/me/playlists/{id}/tracks`
   (listing a playlist's tracks wasn't in the original sketch, but the task
   explicitly asks for "adicionar/remover faixa, listar") and
   `DELETE /v1/library/me/saved-tracks/{track_id}` (unsave — a save
   endpoint without its inverse isn't a complete favorites feature).
7. **Refresh response includes a new refresh token**, not just a new access
   token as backend-go.md §4's sketch shows — required by security.md §2's
   explicit rotation-on-every-use policy; the client must receive the new
   token to keep rotating.
8. **`POST /v1/playback/sessions` returns `{session_id}` only**, not
   `{session_id, playback_url_manifest}` — an HLS manifest belongs to the
   real media-edge-service/CDN pipeline (out of scope; see "Playback"
   above). The client fetches a `stream_url` per track via `.../play`
   instead.
9. **Search ranking is alphabetical + `pg_trgm` similarity filter, not
   popularity-ranked.** data-architecture.md §5.4 already earmarks a
   dedicated search engine for real relevance ranking; building
   popularity-aware keyset pagination on top of a fallback engine slated
   for replacement wasn't worth the complexity here.
10. **`RealIP` middleware deliberately not used** (see `cmd/server/main.go`).
    chi's `RealIP` trusts `X-Forwarded-For`/`X-Real-IP` unconditionally,
    which lets a client spoof its rate-limit identity unless the deployment
    first allowlists trusted proxy IPs (not configured in this slice, and
    `staticcheck` flags the middleware itself as vulnerable to exactly this
    — GHSA-3fxj-6jh8-hvhx). Rate limiting keys on the raw TCP peer address
    instead; adopting a trusted-proxy-aware `RealIP` is a TODO for when a
    concrete LB/proxy topology exists.
11. **Rate limiting is a fixed window (`INCR`+`EXPIRE`), not the
    sliding-window log** backend-go.md §5 calls out as the right choice
    specifically for login. Documented as a TODO in
    `internal/platform/cache/ratelimiter.go`; fixed-window is simpler and
    was judged adequate for this slice (worst case ~2x the nominal limit
    across a window boundary).
12. **No gRPC/OpenTelemetry/Prometheus wiring.** backend-go.md §5
    prescribes Prometheus metrics and OpenTelemetry tracing as general
    architecture; with a single process and no second service to trace
    calls across yet, this was deprioritized against actually finishing
    the four required modules end-to-end. `slog` structured logging (§5's
    other observability requirement) *is* implemented
    (`internal/platform/logging`).
13. **`play_events` partition-creation job not implemented.** The table is
    range-partitioned by `played_at` (data-architecture.md §5.1) with a
    `DEFAULT` partition catching everything, so writes never fail; a
    scheduled job to pre-create monthly partitions (and drop old ones per
    retention policy, data-architecture.md §6.2) is a TODO, not a
    correctness issue.
14. **`plans`, `subscriptions`, `family_plan_members`, `follows`,
    `user_devices` (beyond auth's upsert-on-login use), `library_albums`,
    `library_artists`** exist as **schema only** (migration present, per
    the task's "mantenha as tabelas e relacionamentos centrais"
    instruction) with no service/API layer — billing and social-follow
    endpoints aren't part of this slice's contract (backend-go.md §4, as
    scoped to auth/catalog/library/playback).

## Testes

```bash
go build ./...
go vet ./...
go test -race -cover ./...
staticcheck ./...       # go install honnef.co/go/tools/cmd/staticcheck@latest
govulncheck ./...       # go install golang.org/x/vuln/cmd/govulncheck@latest
```

**Real results from this repo** (not estimated, re-verified after the CORS
+ toolchain-version fix below, running `go1.25.14` — auto-downloaded per
`go.mod`'s `go 1.25.14` directive since the machine that ran this had an
older `go1.25.4` installed): `go build ./...` and `go vet ./...` are
clean; `go test -race -cover ./...` passes with **zero failures**;
`staticcheck ./...` reports zero issues; `govulncheck ./...` reports
**zero** vulnerabilities affecting this code (see below — was 18 before
the fix).

### Cobertura por pacote (política de 00-overview.md §2)

The policy: 100% on hand-written business logic, excluding wiring/`main.go`
and documented-impossible defensive branches, each exclusion justified
in-code (`coverage:ignore` comments) rather than silently.

| Pacote | Cobertura | Nota |
|---|---|---|
| `internal/auth` | **100.0%** | |
| `internal/auth/api` | **100.0%** | |
| `internal/auth/mfa` | **100.0%** | |
| `internal/auth/oauth` | **100.0%** | |
| `internal/auth/password` | 97.1% | remaining line: `crypto/rand.Read` failure (`coverage:ignore`, not reproducible hermetically) |
| `internal/auth/token` | 94.1% | remaining line: same crypto/rand justification in `refresh.go`'s generator |
| `internal/auth/postgres` | 0.0%* | integration tier, see below |
| `internal/catalog` | **100.0%** | |
| `internal/catalog/api` | **100.0%** | |
| `internal/catalog/postgres` | 0.0%* | integration tier |
| `internal/library` | **100.0%** | |
| `internal/library/api` | **100.0%** | |
| `internal/library/postgres` | 0.0%* | integration tier |
| `internal/playback` | **100.0%** | |
| `internal/playback/api` | **100.0%** | |
| `internal/playback/media` | **100.0%** | |
| `internal/playback/redisstore` | 95.0% | remaining line: `json.Marshal` failure on a plain struct (`coverage:ignore`) — unlike the other `postgres/` packages, this one *is* unit-tested (with `miniredis`), see its package doc |
| `internal/platform/cache` | 93.3% | remaining line: Redis failing on `EXPIRE` right after a successful `INCR` (`coverage:ignore`, no per-command fault injection in miniredis) |
| `internal/platform/clock` | **100.0%** | |
| `internal/platform/config` | **100.0%** | |
| `internal/platform/dbx` | 0.0%* | thin pool-wiring, integration tier |
| `internal/platform/httpx` | **100.0%** | |
| `internal/platform/idgen` | 85.7% | remaining line: `uuid.NewV7`'s internal `crypto/rand` failure fallback (`coverage:ignore`) |
| `internal/platform/logging` | **100.0%** | |
| `internal/platform/middleware` | **100.0%** | |
| `cmd/server`, `cmd/migrate` | 0.0% | wiring/`main.go`, explicitly excluded by 00-overview.md §2 |

\* **`*/postgres` packages (and `platform/dbx`) are 0% in the unit-test run
by design**, not a gap: backend-go.md §7's testing pyramid puts real-database
code in the *integration* tier (`testcontainers-go` against a real Postgres),
not the unit tier — these files are pure SQL/pgx plumbing with no branching
business logic (the business logic they front — validation, ownership
checks, rotation, position math, dispatch — lives in each module's
`service.go` and *is* at 100%, tested with in-memory fakes). This is
documented at the top of every `postgres` package file, per
00-overview.md §2's "cada exclusão precisa de justificativa explícita".
Each `postgres` package was also manually integration-verified against a
real, disposable Postgres + Redis (see below) — including catching and
fixing a real bug that the unit-level fakes couldn't have caught (see next
section).

### What was actually verified end-to-end (not simulated)

Since this environment's `docker-compose` CLI is broken (see above), the
same Postgres 16 / Redis 7 images `docker-compose.yml` declares were run
directly via `docker run` to verify the full stack for real:

1. `go run ./cmd/migrate up` — applied cleanly, created all 22 tables +
   `schema_migrations`.
2. `go run ./cmd/server` — booted, connected to both Postgres and Redis,
   started listening.
3. Full HTTP flows exercised with `curl` against the live server: signup →
   login → `/me` → create artist/track → **search** (this caught a real
   bug, see below) → create playlist → add/list tracks → save/list
   favorites → create playback session → enqueue → `next` → `play` →
   `state` → refresh-token rotation → logout → refresh-after-logout
   correctly rejected (`refresh_token_revoked`) → unauthenticated request
   correctly rejected (`401`) → nonexistent session correctly rejected
   (`404`).
4. **Bug found and fixed by this verification, not by the unit suite:**
   `catalog`'s and `library`'s keyset-pagination cursor queries compared an
   empty-string sentinel against a Postgres `uuid` column
   (`(title, id) > ($3, $4)` with `$4 = ''`), which Postgres rejects
   (`invalid input syntax for type uuid: ""`) — the in-memory fakes used
   for unit tests don't enforce column types, so this only surfaced against
   real Postgres. Fixed by only appending the cursor predicate when a
   cursor is actually present, in all four affected queries
   (`catalog/postgres`'s three `Search` methods and
   `library/postgres`'s `LibraryTrackRepo.List`). Re-verified working
   (including second-page pagination) after the fix.

### Go-toolchain vulnerabilities — fixed via `go.mod`'s `go` directive

**Before** (`go.mod` pinned `go 1.25.4`, forcing that toolchain via
`GOTOOLCHAIN=go1.25.4` to reproduce the original finding):
`govulncheck ./...` reported **18 reachable vulnerabilities, all 18 in the
Go standard library itself** — `crypto/tls` (6: GO-2026-6090, GO-2026-5856,
GO-2026-4870, GO-2026-4340, GO-2026-4337), `crypto/x509` (5: GO-2026-5037,
GO-2026-4947, GO-2026-4946, GO-2025-4175, GO-2025-4155), `net/url` (2:
GO-2026-4601, GO-2026-4341), `net/http` (1: GO-2026-6089), `net` (1:
GO-2026-4971), `os` (1: GO-2026-4602), `encoding/xml` (1: GO-2026-6088),
`encoding/asn1` (1: GO-2026-5972). Every one's "Fixed in" version is
`go1.25.5` through `go1.25.13` — none in this project's own code or in a
third-party dependency it actually calls (that run separately reported
7+27 vulnerabilities in imported/required packages `govulncheck` says
"your code doesn't appear to call" — informational, not actionable, and
unaffected by this fix). Exit code `3` (vulnerabilities found).

**Fix**: bumped `go.mod`'s `go` directive from `1.25.4` to **`1.25.14`**
(the newest available `1.25.x` patch at the time of this fix — one past
the `go1.25.13` the audit asked for, and past every "Fixed in" version
above). Since Go 1.21, the `go` directive is a *minimum required
toolchain*, not documentation: with `GOTOOLCHAIN=auto` (the Go default,
unmodified here), any `go` invocation on a machine whose installed
toolchain is older than the directive **transparently downloads and uses
the matching toolchain from the module proxy** before doing anything else
— confirmed on this machine, whose installed `go` was `go1.25.4`:

```
$ go version
go version go1.25.4 linux/amd64
$ go build ./...     # go.mod now says `go 1.25.14`
go: downloading go1.25.14 (linux/amd64)
$ go version
go version go1.25.14 linux/amd64
```

This is exactly the property the task asked for: **the `go.mod` forces
the correct version in CI/deploy** (and in any dev environment with
network access to the module proxy) **regardless of what's installed
locally** — no separate "upgrade Go on the machine" step is required, and
nobody can silently build against the vulnerable `1.25.4` unless they
override `GOTOOLCHAIN` to pin it back down (which is what was done,
deliberately, to reproduce the "before" numbers above).

**After** (same repo, `go.mod` at `go 1.25.14`, no `GOTOOLCHAIN` override
— the toolchain auto-download above already happened by this point):

```
$ govulncheck ./...
=== Symbol Results ===

No vulnerabilities found.

Your code is affected by 0 vulnerabilities.
This scan also found 0 vulnerabilities in packages you import and 17
vulnerabilities in modules you require, but your code doesn't appear to
call these vulnerabilities.
```

Zero reachable vulnerabilities, exit code `0`. (The "17 in modules you
require" line is the same category of informational, not-actually-called
noise as the 7+27 figure before — unaffected by, and unrelated to, this
fix; not investigated further here as it's out of this audit's two
findings.)

## TODOs (grep-able as `TODO` in the code, consolidated here)

- Real Google/Apple OIDC verification (`internal/auth/oauth`).
- TOTP MFA implementation (`internal/auth/mfa`).
- Dedicated search engine (Meilisearch) replacing `pg_trgm`
  (`internal/catalog`).
- Real `media-edge-service` + CDN-signed URLs replacing
  `LocalResolver` (`internal/playback/media`).
- Production key management (JWT signing key, password pepper) via
  Vault/KMS instead of env vars (`internal/platform/config`).
- Sliding-window rate limiting for login specifically
  (`internal/platform/cache`).
- Trusted-proxy-aware `RealIP` once a concrete LB topology exists
  (`cmd/server`).
- Scheduled job to pre-create/drop `play_events` monthly partitions.
- Prometheus metrics / OpenTelemetry tracing (backend-go.md §5).
- Collaborative playlist editing by non-owners (`internal/library`).
- Role-based authz for catalog write endpoints (currently: any
  authenticated user).
- Integration test suite (`testcontainers-go`) for the `*/postgres`
  packages, automated in CI — this slice verified them manually against
  live containers instead (see above).
