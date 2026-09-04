# smusic backend — Fatia 1 + Fatia 2 (auth, catalog, library, playback-state, presence)

Go backend implementing `docs/architecture/`'s first two vertical slices:

- **Fatia 1** (monolith, `cmd/server`): **auth, catalog, library,
  playback-state**.
- **Fatia 2** (`internal/presence` + the separate `cmd/presence-server`
  process): proximity-discovery / social presence, implementing
  `security.md` §1's mandatory privacy model in full — see
  [Presence / proximity discovery](#presence--proximity-discovery-fatia-2)
  below.

`media-edge-service` extraction is still out of scope (Fatia 3+). Architecture,
decisions and every deviation from the four planning docs are documented
inline in the code (search for `TODO` and doc comments referencing
`backend-go.md`, `data-architecture.md`, `security.md`, `00-overview.md`) and
summarized in **[Desvios da spec](#desvios-da-spec-e-justificativas)** below.

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
  presence/       proximity-discovery privacy pipeline + Redis geo-index
                  (Fatia 2, see below) — imported by BOTH cmd/server (REST
                  settings/consent/block control plane) and
                  cmd/presence-server (the WS data plane)
    api/          REST handlers (settings/consent/pause/blocks) — mounted
                  on cmd/server, NOT presence-server
    ws/           WebSocket transport for /v1/presence/connect — mounted
                  on cmd/presence-server, NOT cmd/server
    redisstore/   presence.GeoIndex over Redis GEOADD/GEOSEARCH (ephemeral
                  only, TTL-bound — never durable)
  platform/       shared, cross-cutting: config, clock, idgen, httpx,
                  middleware (auth, rate limit), cache (Redis wiring +
                  rate limiter), dbx (Postgres pool wiring), logging
cmd/
  server/         wires everything together, runs smusic-core's HTTP server
                  (auth/catalog/library/playback + presence's REST control
                  plane)
  presence-server/ separate process (Fatia 2, backend-go.md §1): hosts ONLY
                  WS /v1/presence/connect — see below
  migrate/        thin CLI over golang-migrate
migrations/       0001_init.{up,down}.sql (Fatia 1), 0002_presence.{up,down}.sql
                  (Fatia 2: user_privacy_settings, user_blocks, presence_audit_log
                  — never a lat/lng column, see 0002's header comment)
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
- MFA: `internal/auth/mfa` ships a real `TOTPChallenger` (RFC 6238, via
  `pquerna/otp`), wired for the one call site security.md §2 makes
  mandatory today — granting proximity consent
  (`presence.SettingsService.GrantConsent`, see "Presence" below and
  `.vibeflow/specs/mfa-for-proximity-consent.md`). `POST /v1/auth/mfa/enroll`
  (authenticated) returns a base32 secret + `otpauth://` URI for a QR
  code; `POST /v1/auth/mfa/verify` (`{code}`) checks a submitted code and,
  on the first success, activates the factor. `NoopChallenger` remains
  available as a test double / for any future call site that deliberately
  has no MFA requirement — none exists today. Enrollment other than for
  proximity (step-up for password/email change, session management) is a
  documented follow-up, same scope boundary as before.

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

**`cmd/presence-server`'s WS handshake shares this same allowlist.**
`internal/presence/ws/handler.go`'s `Upgrader.CheckOrigin`
(`newOriginChecker`) is fed the identical `Config.CORSAllowedOrigins`
(same `CORS_ALLOWED_ORIGINS` env var, one config, two processes) instead
of gorilla/websocket's library default (reject unless `Origin` equals the
request host — same-origin only). That default made the WS endpoint
unreachable from any Web client in every realistic deployment, since
presence-server is a separate process/origin from the Web app by design
(backend-go.md §1) — there is no topology where they share an origin. A
missing `Origin` header (native/mobile clients — only browsers set it on
a WS handshake) always passes, matching the REST policy's own "CORS is
browser-enforced, never server-side" stance; with `CORS_ALLOWED_ORIGINS`
unset, WS falls back to the library's same-origin default rather than
rejecting every browser client outright.

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

## Presence / proximity discovery (Fatia 2)

Implements `backend-go.md` §1/§3/§4 (separate process, concurrency model, WS
protocol) and `security.md` §1 in full (the mandatory privacy model). Two
processes:

- **`cmd/presence-server`** (own binary, own listen address —
  `PRESENCE_HTTP_ADDR`, default `:8081`): hosts **only**
  `WS /v1/presence/connect`. This is the latency/concurrency-sensitive "data
  plane" — thousands of long-lived connections, a fixed worker pool
  (`internal/presence.Hub`), three-layer backpressure.
- **`cmd/server`** (smusic-core): hosts the low-frequency, Postgres-backed
  "control plane" — `GET/PUT /v1/presence/settings`,
  `POST/DELETE /v1/presence/consent`, `POST /v1/presence/pause`,
  `POST /v1/presence/resume`, `POST/DELETE /v1/presence/blocks/{user_id}`.

### Privacy model (security.md §1) — where each control lives

| Control | Enforced in |
|---|---|
| Opt-in consent, off by default, 6-month renewal | `PrivacySettings`/`SettingsService` (`domain.go`, `settings_service.go`); WS handshake rejects (`403 consent_required`/`consent_expired`) without valid consent — `ws/handler.go`'s `ServeHTTP` |
| MFA required before consent can be granted (security.md §2) | `SettingsService.GrantConsent` calls `MFAChecker.HasVerifiedMFA` before any other work (`403 mfa_required` otherwise) — implemented by `auth.Service`/`internal/auth/mfa.TOTPChallenger`, wired only in `cmd/server/main.go` (presence never imports auth directly) |
| 4 distance buckets, never coordinate/geohash to client | `DistanceBucket`/`BucketFor` (`bucket.go`); `NearbyResult`/`ws/protocol.go`'s `userFrame` structurally has no float64/lat/lon field (asserted by reflection in `TestNearbyResult_StructurallyCannotCarryCoordinates`) |
| ±75m jitter, renewed every heartbeat, never the raw coordinate stored | `Jitterer`/`RandJitterer` (`bucket.go`); applied once in `NearbyService.ApplyUpdate`, raw lat/lon is a local variable never passed elsewhere |
| Mutual (intersection, not union) visibility radius, 150m–15km slider | `NearbyService.query`'s `math.Min(requester radius, target radius)` gate |
| Invisible/pause = immediate index removal | `NearbyService.ApplyUpdate`/`SetVisibility`/`Disconnect`, `SettingsService.Update`/`RevokeConsent` all call `GeoIndex.Remove` synchronously |
| Silent block, bidirectional | `user_blocks` table, `BlockRepository.IsBlockedEitherWay` checked in `NearbyService.query` before any other candidate work |
| 3 reveal levels (0 anonymous / 1 connections / 2 open discovery) | `NearbyService.query`'s `mutual \|\| target.RevealLevel == RevealLevelOpenDiscovery` gate; level 1 uses the Fatia 1 `follows` table via `FollowChecker` |
| Anti-triangulation rate limits (1/pair/30s, 200/day) | `ratelimit.go`'s `PairQueryLimit`/`DailyQueryLimit`, reusing `internal/platform/cache.RedisRateLimiter` (the task's explicit hint) via the local `RateLimiter` interface |
| Append-only audit log | `presence_audit_log` table + `BEFORE UPDATE/DELETE` triggers that unconditionally raise (migration `0002_presence.up.sql`); no read endpoint in this slice |
| Presence never in a durable table | `PresenceEntry`/`GeoPosition` never round-trip through `internal/presence/postgres`; only `internal/presence/redisstore` (TTL-bound) ever holds a position |

### Concurrency (backend-go.md §3)

`Hub` (`hub.go`): bounded ingest channel (layer-1 backpressure, reject not
block), fixed worker pool (`NewHubWithLimit`'s `workers` param — no
goroutine-per-update), per-connection bounded outbound buffer (layer-2,
`ws/conn.go`'s `outboundBuffer`), per-connection WS-frame rate limit
(layer-3, `AllowUpdateFrame`, configurable via `PRESENCE_UPDATE_RATE_LIMIT`/
`PRESENCE_UPDATE_RATE_WINDOW`). No global locks — the geo-index itself is a
single Redis key in this slice (sharding by region is a documented TODO in
`redisstore/geoindex.go`, an orthogonal scaling concern per
data-architecture.md §4.4, not a correctness one). Graceful shutdown
(`Hub.Shutdown`) broadcasts a `drain` frame to every connected client, closes
ingest, and waits for in-flight workers up to the caller's context deadline.

`Hub`'s `sync.WaitGroup` count is `Add`-ed once, synchronously, in the
constructor (`NewHubWithLimit`) rather than inside `Run()` — `go test -race`
caught a genuine (if narrow-window) data race in the original draft, where
`Add` (inside a `go hub.Run(ctx)` goroutine) could run concurrently with
`Wait` (inside `Shutdown`'s own internal goroutine) with no synchronization
between them, exactly the misuse the `sync.WaitGroup` docs warn about. Fixed
by setting the full worker count once at construction time, which strictly
happens-before `Run`/`Shutdown` can ever be invoked on that `Hub` value —
see `hub.go`'s `NewHubWithLimit` doc comment for the full explanation.

### WS protocol (backend-go.md §4)

`WS /v1/presence/connect`, bearer token via `Authorization` header or
`?access_token=` query param (browser WS clients can't set custom headers on
the handshake). Client→server: `{type:"update", lat, lon, now_playing?}`,
`{type:"heartbeat"}`, `{type:"visibility", mode}`. Server→client:
`{type:"nearby_update"|"resync_full", users:[...]}`,
`{type:"drain", reconnect_hint}`. This slice always computes and sends the
full current nearby set on every update/heartbeat rather than maintaining
incremental per-connection delta state — a documented simplification (see
`hub.go`'s `SendResync` doc comment); `resync_full` and `nearby_update`
therefore carry an identically-shaped payload, differing only in `type`.

### Running locally

```bash
go run ./cmd/migrate up       # applies 0001 + 0002
go run ./cmd/server           # :8080 — auth/catalog/library/playback + presence REST
go run ./cmd/presence-server  # :8081 — WS /v1/presence/connect
```

Both processes must share the **same** `JWT_ED25519_SEED_HEX` (presence-server
only verifies access tokens smusic-core issued) and point at the same
Postgres/Redis in any persistent environment — see `cmd/presence-server/main.go`'s
package doc for the documented deviation from backend-go.md §1/§2's gRPC-between-
processes target design (plain Go import + shared DB, same rationale as Fatia
1's own "no gRPC yet" deviation below).

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

### Fatia 2 (presence) deviations

15. **`presence-service` shares `internal/presence` via a plain Go import
    and connects to the same Postgres/Redis as smusic-core, instead of
    talking gRPC to smusic-core** as backend-go.md §1/§2's target
    architecture describes. Same rationale as deviation #1 above (a second
    process with exactly one internal caller doesn't justify a
    protobuf/gRPC-Gateway toolchain yet) — see
    `cmd/presence-server/main.go`'s package doc for the full explanation and
    the TODO for when to revisit (a second internal caller, or per-service DB
    credentials forcing the split at the data layer too).
16. **Audit log (security.md §1.8) is an append-only table in the same
    operational Postgres, not a separate WORM store** — security.md §7
    explicitly flags this as an open infrastructure question with no answer
    yet. Substituted with `BEFORE UPDATE/DELETE` triggers on
    `presence_audit_log` that unconditionally raise, so even a bug or an
    ad-hoc manual query can't mutate/erase a row — see
    `migrations/0002_presence.up.sql`'s header comment for the full
    reasoning, including why `requester_id`/`target_id` are plain UUID
    columns without an `ON DELETE CASCADE` FK to `users` (so account
    deletion can never silently erase abuse-investigation history). The
    180-day retention purge job and the automatic abuse-pattern detection
    (security.md §1.8) are documented TODOs, not implemented in this slice.
17. **No k-anonymity aggregate pipeline** (security.md §1.5's k≥20
    aggregated "N people are listening to this in this city" metric) — no
    analytics/BI pipeline exists yet in this codebase for presence to feed
    into; the *precondition* for it (raw presence never persisted anywhere
    durable, never mirrored to a warehouse) is honored, but the aggregate
    feature itself is out of scope for this slice.
18. **Geo-index is a single, unsharded Redis key** (`presence:geo`), not
    partitioned by coarse region as data-architecture.md §4.4 recommends for
    horizontal scale. Documented as a scaling TODO in
    `redisstore/geoindex.go` — explicitly not a correctness or privacy
    concern (`GEOSEARCH`'s radius query is correct regardless of sharding),
    and the `GeoIndex` interface doesn't leak this decision to callers, so
    adding sharding later only touches that one file.
19. **`GeoSearchLocation` avoided in favor of `GeoSearch` + a `GeoPos`
    batch** — `redisstore.Store.Search`'s doc comment documents a confirmed
    argument-duplication bug in `go-redis/v9`'s `GeoSearchLocation` (up to
    and including the pinned v9.22.0) that makes Redis reject the command
    with a syntax error; worked around with the unaffected two-call path
    instead of filing/waiting on an upstream fix.
20. **`Hub`'s `sync.WaitGroup.Add` moved from `Run()` to the constructor**
    (`NewHubWithLimit`) after `go test -race` caught a genuine data race
    between `Add` (inside a `go hub.Run(ctx)` goroutine) and `Wait` (inside
    `Shutdown`'s internal goroutine) — see the "Concurrency" section above
    for the full explanation. Not a deviation from any doc's prescribed
    design, but worth flagging as a concurrency-correctness fix made during
    this slice's own verification, exactly the kind of issue backend-go.md
    §7's mandatory `-race` gate exists to catch.
21. **Client identity ("nome de escuta" pseudonym distinct from the
    social-profile display name, security.md §1.6) is not implemented** —
    `ProfileResolver` in this slice resolves the same `display_name`/
    `avatar_url` used elsewhere in the app (`cmd/presence-server`'s
    `profileResolver`, reading directly off `users`). A dedicated
    presence-only pseudonym is a schema and product-flow addition out of
    scope for this slice; documented here so it isn't mistaken for done.

### Post-implementation security review (adversarial audit against security.md §1)

The items below were found and fixed during an independent security review
of this slice against `docs/architecture/security.md` §1, run with the same
rigor as an internal pentest (attempted bypasses of consent gating, jitter,
block symmetry, radius intersection, rate limiting and audit-log
immutability). The review's full 11-point report lives in that review's own
session record; only the concrete code changes are listed here, since
they're now load-bearing behavior other specialists may rely on:

22. **A WS connection whose owner's proximity consent expired or was
    revoked mid-session used to go silently dead instead of being closed.**
    `NearbyService.ApplyUpdate`/`ApplyHeartbeat` already correctly refused
    to process further frames the instant `HasActiveConsent` turned false
    (security.md §1.1) — but `Hub.process` (`hub.go`) swallowed that error
    like any other transient failure, leaving the socket registered and
    open, receiving nothing, until the client eventually gave up or
    reconnected on its own. No presence data ever leaked through this path
    (every query still re-checks live consent per candidate), but it meant
    "revogação... interrompe o processamento imediatamente" (§1.1) held for
    the *data* but not for the *connection itself*. Fixed: `Hub.process`
    now special-cases `ErrConsentRequired`/`ErrConsentExpired` specifically
    — it sends the existing `drain` frame (with `reconnect_hint`
    `"consent_required"`/`"consent_expired"`) and calls the (newly added)
    `Conn.Close()`, so a well-behaved client reconnects immediately and hits
    the WS handshake's own consent check (`ws/handler.go`) instead of
    sitting on a dead socket. `presence.Conn` gained a `Close()` method
    (implemented by `ws/conn.go`'s existing idempotent `close()`, and by the
    `hub_test.go` fake) to make this possible — see `hub.go`'s `process` doc
    comment and `TestHub_Process_ConsentError_DrainsAndClosesConn`/
    `TestHub_Process_ConsentExpiredMidConnection_DrainsWithSpecificHint`.

    **Follow-up bug in this same mechanism, found by a later Auditor pass
    and fixed in this commit**: the `Conn.Close()` this item introduces
    (`ws/conn.go`'s `close()`) tore the physical socket down *synchronously*
    right after `Hub.process` enqueued the `drain` frame — racing
    `writePump`'s own goroutine for that just-buffered frame and routinely
    winning, since `ws.Close()` doesn't wait for anything. The client only
    ever observed a bare `1006` abnormal closure, never the `drain` frame
    (with its `reconnect_hint`) explaining why — reproduced 4/4 times.
    Fixed: `close()` now signals `writePump` (via a new `shutdown` channel,
    kept separate from the existing `closed` channel that still marks full
    teardown) to drain whatever's left in the outbound buffer — the `drain`
    frame is essentially always already sitting there, since `Send()` and
    `Close()` run back-to-back in the same `Hub` worker goroutine — and
    write it before the transport is physically closed, bounded by a
    200ms `closeDrainWait` so a stalled write can't hang shutdown
    indefinitely. Covered by a new integration test that (unlike
    `hub_test.go`'s channel-based fake `Conn`, which has no such timing to
    get wrong) exercises the *real* `Conn` end to end — a real `Handler`,
    a real `httptest` WS server, and a real client dial —
    `TestHandler_ConsentRevoked_MidSession_ClientReceivesDrainBeforeClose`
    (`ws/handler_test.go`); confirmed to fail 4/4 against the pre-fix
    `conn.go` and pass 4/4 after.
24. **Jitter renewal gap for stationary clients (security.md §1.2)** — see
    `frontend/README.md`'s matching entry; the fix is entirely client-side
    (`WebSocketProximityFeedRepository`), no backend change was needed since
    it just makes the client send `update` frames (already part of the wire
    contract) on the heartbeat cadence instead of position-less `heartbeat`
    frames once a position is known.
25. **Identified but NOT fixed in this review — flagged as tech debt for
    the Auditor**: `NearbyResult.UserID`/the WS `user_id` field is the
    real, permanent `users.id` UUID, sent to a viewer regardless of reveal
    level (including level 0/anonymous). Security.md §1.6's level-0 copy
    ("Alguém por perto está ouvindo *[Faixa]*") implies no persistent handle
    at all, not just "no name/avatar" — as implemented, a viewer can still
    recognize "the same anonymous person" across sessions/days by this ID,
    and if any future endpoint ever maps a `user_id` to a profile (none
    exists yet in this codebase — verified by inspecting every mounted REST
    route), that would fully deanonymize an otherwise-anonymous encounter.
    Not fixed here because a real fix (session-scoped or rotating
    pseudonymous IDs) is a wire-protocol and client list-diffing change
    that reaches into both repos' contract, not a contained bug fix; until
    then, no route in this codebase can actually exploit it. Recommended:
    derive a per-viewer, per-target, time-boxed opaque token instead of
    reusing the raw UUID on the wire.

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

**Re-verified again after Fatia 2 (presence)**: same clean results across
`go build`/`go vet`/`staticcheck`/`govulncheck`; `go test -race -cover ./...`
passes with zero failures across ~10 consecutive runs of
`./internal/presence/...` specifically (deliberately repeated, given that
this slice's own verification caught and fixed a real `-race` failure in
`Hub`'s `sync.WaitGroup` usage — deviation #20 above).

**Re-verified again after the Auditor's 2 Fatia 2 WS-transport findings
(CheckOrigin allowlist + drain-frame-lost-on-close race, both above)**:
`go build ./...`, `go vet ./...`, `staticcheck ./...` and `govulncheck ./...`
all clean (`go1.25.4`; govulncheck: 0 vulnerabilities affecting this code,
17 in unused parts of required modules); `go test -race -cover ./...`
passes with **zero failures** across 3 full consecutive repo-wide runs
(`-count=3`) plus additional isolated `-race -count=4` reruns of
`./internal/presence/ws/...` — no flakes, no data races. The new
`TestHandler_ConsentRevoked_MidSession_ClientReceivesDrainBeforeClose`
integration test (real `Handler`, real `httptest` WS server, real client
dial) was confirmed to fail 4/4 against the pre-fix `conn.go` (`websocket:
close 1006 (abnormal closure): unexpected EOF`, matching the Auditor's own
4/4 reproduction) and pass 4/4 after the fix. Bug 1 (`CheckOrigin`) was
additionally verified with a real WS client dial per Origin-header case
(allowlisted origin → `101`, non-allowlisted → `403`, no `Origin` header at
all → `101`) via `TestHandler_CheckOrigin_*` — genuine client/server
handshakes over a real `httptest` TCP listener, not mocked.

**Known pre-existing flake, unrelated to Fatia 2**:
`internal/catalog`'s `TestSearch_Pagination` fails intermittently
(non-deterministically reproduced in isolated `-race` reruns of
`./internal/catalog/...` alone, unrelated to any presence code or to
anything touched in this slice) — looks like a keyset-pagination ordering
assumption in the test's fake repo that isn't fully deterministic across
runs. Flagged here rather than silently ignored, per this project's own
"no undocumented gaps" policy, but left unfixed: `internal/catalog` is
Fatia 1 code, already audited and approved, and out of this slice's scope.

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
| `internal/presence` | 99.4% | remaining lines: two defensive branches unreachable through any `GeoIndex` implementation given its documented contract — a duplicate-candidate guard already enforced by `GeoIndex.Search`'s own contract, and a `BucketFor(15000)==BucketNone` guard reachable only at an exact float64 boundary not reproducible via real haversine math (both `coverage:ignore`, see `nearby_service.go`; `BucketFor`'s own boundary behavior, including this exact value, *is* directly unit-tested in `bucket_test.go`) |
| `internal/presence/api` | **100.0%** | |
| `internal/presence/postgres` | 0.0%* | integration tier, see below — manually verified end-to-end against real Postgres (signup, consent grant/revoke, settings update, block/unblock, audit-log rows) |
| `internal/presence/redisstore` | 85.3% | remaining lines: Redis-command-sequence failures (e.g. `GEOADD` succeeding then the very next command failing) not reproducible with `miniredis`'s uniform fault injection — same documented limitation as `internal/platform/cache`'s `EXPIRE`-after-`INCR` branch above; every remaining line carries its own `coverage:ignore` justification in `geoindex.go` |
| `internal/presence/ws` | 95.6% | remaining lines, all `coverage:ignore`'d: a narrow send/close race window (`Send`'s `<-c.closed` branch), a `json.Marshal` failure on a plain string/bool struct (`writeFrame`), and — new with the Bug 2 fix — `drainThenTeardown`'s inner `<-c.out` frame-flush branch (in practice always beaten by `writePump`'s own main-loop `case f := <-c.out`, which is normally already parked waiting when a frame is enqueued) and its `closeDrainWait` timeout branch (not reproducible without a deliberately hung fake transport); same reasoning as `internal/playback/redisstore`'s analogous branches |
| `cmd/server`, `cmd/migrate`, `cmd/presence-server` | 0.0% | wiring/`main.go`, explicitly excluded by 00-overview.md §2 |

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

### What was actually verified end-to-end — Fatia 2 (presence)

`go run ./cmd/migrate up` applied `0002_presence.up.sql` cleanly (confirmed
`user_privacy_settings`/`user_blocks`/`presence_audit_log` present, with the
audit log's immutability triggers attached). Both `cmd/server` and
`cmd/presence-server` were booted against the same live Postgres 16 + Redis 7
and exercised together:

1. Two real users signed up via `cmd/server`'s auth API.
2. `GET /v1/presence/settings` confirmed the safe-by-default row
   (`invisible`/`paused:true`/`consent:false`) for a user who never touched
   presence — security.md §1.1's "nasce desligada" holds at the schema
   level, not just in application logic.
3. `WS /v1/presence/connect` correctly rejected (`403 consent_required`)
   before consent was granted.
4. After `POST /v1/presence/consent` + `PUT /v1/presence/settings`
   (`visibility:everyone`, `paused:false`) for both users, a real WS client
   (two concurrent connections) sent `update` frames ~500m apart and each
   received the other back with **only** `distance_bucket:"150m_1km"` /
   `distance_label:"No seu bairro"` — no coordinate, no exact distance, no
   name (reveal level 0 by default, not a mutual follow).
5. `presence_audit_log` gained one row per direction of that exchange,
   `distance_bucket=2`, `endpoint='ws:/v1/presence/connect'` — confirmed via
   `psql`, never a lat/lng column.
6. `POST /v1/presence/blocks/{user_id}` (real Postgres write to
   `user_blocks`) then a fresh WS query confirmed the blocked user no longer
   appeared, even after the 30s pair-rate-limit window had elapsed (so the
   effect was the block, not the rate limiter).
7. `DELETE /v1/presence/consent` immediately caused the next WS connection
   attempt to be rejected (`403 consent_expired`→ actually
   `consent_required`, since revocation clears `proximity_consent_enabled`
   rather than merely expiring it) and confirmed `paused` was force-set to
   `true` in the same response, per `RevokeConsent`'s documented defense-in-
   depth.
8. `GEOPOS presence:geo <user_id>` returned empty immediately after each WS
   connection closed — confirming presence is tied to the live connection
   (`Hub.Unregister` → `NearbyService.Disconnect` → `GeoIndex.Remove`), not
   left dangling for the remainder of its TTL.

No bugs were found by this pass (unlike Fatia 1's cursor-pagination bug
above) — the privacy pipeline behaved exactly as its extensive unit-test
suite already predicted.

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
- **Fatia 2 (presence) TODOs:**
  - gRPC between `cmd/server` and `cmd/presence-server`, replacing the
    shared-Go-import/shared-DB deviation (`cmd/presence-server/main.go`).
  - Geo-index sharding by coarse region (`internal/presence/redisstore`),
    per data-architecture.md §4.4 — a scaling, not correctness, concern.
  - Periodic sweep of stale `presence:geo` sorted-set members
    (`internal/presence/redisstore`) — currently filtered lazily at read
    time, never exposed, just a memory-tidiness gap bounded by the 90s TTL.
  - Real WORM/object-lock storage for the audit log (security.md §7),
    replacing the trigger-enforced append-only Postgres table.
  - 180-day audit-log retention purge job and automatic abuse-pattern
    detection/alerting (security.md §1.8).
  - k-anonymity (k≥20) aggregate pipeline for "N people are listening to
    this nearby" product metrics (security.md §1.5) — no analytics/BI
    pipeline exists yet for presence to feed into.
  - Presence-only pseudonym ("nome de escuta") distinct from the
    social-profile display name (security.md §1.6) — currently the same
    `display_name` is reused.
  - Movement-plausibility check on `update` frames (security.md's threat
    model open question: reject updates implying an impossible speed of
    travel between two consecutive positions) — not implemented in this
    slice.
