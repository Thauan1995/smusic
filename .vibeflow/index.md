# Project: smusic

> Analyzed: 2026-09-04
> Stack: Backend — Go 1.25 monolith modular (`chi`, `pgx/v5`+Postgres 16, `go-redis`+Redis 7, Ed25519 JWT, Argon2id) + separate `presence-server` process (WS). Frontend — Dart/Flutter, Melos monorepo (19 packages), Riverpod, `dio`, `just_audio`, `geolocator`. Deploy — Docker Compose + Caddy (DuckDNS DNS-01 TLS) on a home-lab VPS.
> Type: Two-sided product monorepo (Go API backend + Flutter web/mobile client) implementing a music-streaming platform with a real-time proximity-based social discovery feature as its core differentiator.
> Suggested budget: ≤ 8 files per task

## Structure

The repo was built from an explicit 4-document architecture-planning phase (`docs/architecture/`: `00-overview.md`, `backend-go.md`, `data-architecture.md`, `frontend-flutter.md`, `security.md`), each written by a distinct "specialist" persona (Go backend, Flutter frontend, data architecture, security) per the project's founding brief, then implemented in two tracked slices ("Fatia 1" = base: auth/catalog/library/playback; "Fatia 2" = social-proximity discovery). Every deviation from the planning docs is documented inline in code and consolidated in `backend/README.md`'s "Desvios da spec" section and `frontend/README.md`.

```
backend/            Go monolith (cmd/server :8080) + separate cmd/presence-server (:8081, WS only)
  internal/<domain>/ domain.go · repo.go (interfaces) · service.go · postgres/ · api/
  migrations/         golang-migrate, numbered .up/.down.sql
frontend/            Melos workspace: packages/{core,domain,data,presentation}/<feature> + app/{smusic_app_shared,smusic_mobile,smusic_web}
docs/architecture/   the 4 planning specs + 00-overview.md decision log (Auditor status, coverage policy)
deploy/              docker-compose.prod.yml + Caddyfile (production, VPS/home-lab)
scripts/             local run automation (run_local.sh, path_proxy.py for local presence-server routing)
```

## Structural Units

- **backend/internal/auth** — signup/login/refresh/logout, Argon2id + pepper, Ed25519 JWT + rotated opaque refresh tokens, OAuth stub, MFA stub (`NoopChallenger` — see Known Issues).
- **backend/internal/catalog** — artists/albums/tracks CRUD + `pg_trgm` search, cursor-paginated.
- **backend/internal/library** — playlists (fractional `position`) + saved-tracks ("Músicas Curtidas").
- **backend/internal/playback** — session state (play/pause/seek/next/queue) in Redis only; `LocalResolver` fakes CDN-signed URLs (real `media-edge-service`/HLS/CDN is out of scope for this slice).
- **backend/internal/presence** — the differentiator: proximity discovery. `Hub` (WS data plane, `cmd/presence-server`) + `SettingsService`/REST control plane (`cmd/server`) implement `security.md` §1's full privacy model (opt-in, 4 distance buckets, ±75m jitter, mutual-radius intersection, silent block, reveal levels, anti-triangulation rate limits, append-only audit log via DB triggers).
- **backend/internal/platform** — config, clock, idgen (UUIDv7), dbx (pg pool), cache (Redis + rate limiter), logging (`slog`), middleware (auth, rate limit).
- **frontend/packages/core** — `core_platform` (audio engine, location provider), `core_networking` (`dio` `ApiClient` + interceptors, `ReconnectingWebSocketClient`), `core_design_system`.
- **frontend/packages/{domain,data,presentation}/{auth,library,player,social_proximity}** — one vertical slice per feature, strict layer dependency direction enforced by `frontend/tool/check_layer_deps.sh`.
- **frontend/app/{smusic_app_shared,smusic_mobile,smusic_web}** — shared root widget + two verified-thin entrypoints (100% shared UI/business logic, no platform forks — see `frontend-layered-architecture.md`).

## Pattern Registry

<!-- vibeflow:patterns:start -->
patterns:
  - file: patterns/backend-module-layout.md
    tags: [go, module-layout, layering, architecture, ddd]
    modules: [backend/internal/auth/, backend/internal/catalog/, backend/internal/library/, backend/internal/playback/, backend/internal/presence/]
  - file: patterns/backend-error-handling.md
    tags: [go, error-handling, sentinel-errors, http-status-mapping]
    modules: [backend/internal/auth/, backend/internal/presence/, backend/internal/catalog/, backend/internal/library/, backend/internal/playback/]
  - file: patterns/backend-http-handlers.md
    tags: [go, http, chi, handlers, rest]
    modules: [backend/internal/auth/api/, backend/internal/catalog/api/, backend/internal/library/api/, backend/internal/playback/api/, backend/internal/presence/api/]
  - file: patterns/backend-testing.md
    tags: [go, testing, testify, fakes, coverage, unit-tests]
    modules: [backend/internal/]
  - file: patterns/backend-concurrency-presence.md
    tags: [go, concurrency, goroutines, channels, backpressure, websocket, presence]
    modules: [backend/internal/presence/, backend/cmd/presence-server/]
  - file: patterns/backend-config-platform.md
    tags: [go, config, env-vars, redis, rate-limiting, platform]
    modules: [backend/internal/platform/]
  - file: patterns/frontend-layered-architecture.md
    tags: [flutter, dart, melos, monorepo, layering, architecture]
    modules: [frontend/packages/, frontend/app/, frontend/tool/]
  - file: patterns/frontend-state-management.md
    tags: [flutter, dart, riverpod, state-management, notifiers]
    modules: [frontend/packages/domain/, frontend/packages/presentation/]
  - file: patterns/frontend-networking.md
    tags: [dio, http, interceptors, retry, auth, api-client]
    modules: [frontend/packages/core/core_networking/]
  - file: patterns/frontend-websocket-realtime.md
    tags: [websocket, realtime, reconnect, backoff, presence]
    modules: [frontend/packages/core/core_networking/, frontend/packages/data/social_proximity_data/]
  - file: patterns/frontend-proximity-privacy-ui.md
    tags: [privacy, location, permissions, opt-in, social-proximity]
    modules: [frontend/packages/domain/social_proximity_domain/, frontend/packages/data/social_proximity_data/, frontend/packages/presentation/social_proximity_ui/]
  - file: patterns/frontend-audio-playback.md
    tags: [audio, just-audio, playback, gapless, prefetch]
    modules: [frontend/packages/core/core_platform/, frontend/packages/data/player_data/]
  - file: patterns/frontend-testing.md
    tags: [testing, coverage, flutter-test, dart-test, mocking]
    modules: [frontend/packages/, frontend/app/]
  - file: patterns/frontend-design-system.md
    tags: [flutter, dart, design-system, theming, color, icons, spacing, skeleton, ui]
    modules: [frontend/packages/core/core_design_system/, frontend/packages/presentation/]
<!-- vibeflow:patterns:end -->

## Pattern Docs Available

- [backend-module-layout.md](patterns/backend-module-layout.md) — domain/repo/service/postgres/api layering, one directory per bounded context.
- [backend-error-handling.md](patterns/backend-error-handling.md) — sentinel errors + one centralized `writeXError` mapper per module.
- [backend-http-handlers.md](patterns/backend-http-handlers.md) — thin chi handlers: decode → service call → map response/error.
- [backend-testing.md](patterns/backend-testing.md) — testify + in-memory fakes for service-layer unit tests.
- [backend-concurrency-presence.md](patterns/backend-concurrency-presence.md) — bounded-channel worker pool + 3-layer backpressure in `presence.Hub`.
- [backend-config-platform.md](patterns/backend-config-platform.md) — env-var config loading, Redis wiring, rate limiter.
- [frontend-layered-architecture.md](patterns/frontend-layered-architecture.md) — Melos core/domain/data/presentation layering, enforced by script.
- [frontend-state-management.md](patterns/frontend-state-management.md) — Riverpod `AsyncNotifier` + DI via provider overrides.
- [frontend-networking.md](patterns/frontend-networking.md) — shared `dio` `ApiClient` + auth/retry interceptors.
- [frontend-websocket-realtime.md](patterns/frontend-websocket-realtime.md) — `ReconnectingWebSocketClient` (backoff + jitter, generic).
- [frontend-proximity-privacy-ui.md](patterns/frontend-proximity-privacy-ui.md) — opt-in consent/value-screen flow gating the proximity feature.
- [frontend-audio-playback.md](patterns/frontend-audio-playback.md) — `NativeAudioEngine`/`just_audio` abstraction (gapless engine wiring incomplete — see Known Issues).
- [frontend-testing.md](patterns/frontend-testing.md) — per-package `flutter test`/`dart test` + coverage via Melos.
- [frontend-design-system.md](patterns/frontend-design-system.md) — color/spacing/icon/skeleton tokens and shared widgets; documents 3 gaps found in a 2026-09-04 UI/UX audit (see specs below).

## Key Files

- `backend/cmd/server/main.go` — DI wiring for the monolith (auth/catalog/library/playback + presence REST control plane), excluded from coverage target by policy.
- `backend/cmd/presence-server/main.go` — separate WS-only process, mounts only `/v1/presence/connect`.
- `backend/internal/presence/hub.go` — the concurrency core of the differentiator feature.
- `backend/internal/presence/bucket.go` — distance-bucket + jitter privacy math (security.md §1.2 implementation).
- `docs/architecture/00-overview.md` — cross-specialist decision log; §2 defines the project's actual (adjusted) test-coverage policy.
- `docs/architecture/security.md` — the security contract; §1 is the proximity privacy model, §5's STRIDE table and §6's "Crítico" definition govern the Auditor's zero-critical-vuln stop condition.
- `backend/README.md` — "Desvios da spec e justificativas": the authoritative list of implementation deviations from the 4 planning docs.
- `deploy/Caddyfile` / `deploy/docker-compose.prod.yml` — production topology (see Known Issues: routing bug found live).
- `frontend/melos.yaml` / `frontend/tool/check_layer_deps.sh` — workspace + layering enforcement.
- `frontend/app/smusic_mobile/lib/main.dart` / `frontend/app/smusic_web/lib/main.dart` — the two (verified-thin, near-identical) platform entrypoints.

## Dependencies (critical only)

- `pgx/v5` — Postgres driver, chosen for native-type support over `database/sql`+`lib/pq`.
- `go-redis/v9` — presence geo-index (`GEOADD`/`GEOSEARCH`), playback session state, rate limiting.
- `golang-jwt/jwt/v5` + Ed25519 — access tokens; `golang.org/x/crypto/argon2` — password hashing.
- `gorilla/websocket` — presence WS transport (spec's `backend-go.md` proposed `nhooyr.io/websocket` as an alternative; gorilla was chosen instead — not flagged as a deviation in the README, worth a one-line note if audited literally against the doc).
- Frontend: `riverpod` (state), `dio` (networking), `just_audio` (playback), `geolocator` (location) — per `frontend-flutter.md`.
- No gRPC anywhere despite `backend-go.md` §2 specifying it as the internal source-of-truth contract — documented deviation #1 in `backend/README.md`.

## Known Issues / Tech Debt

1. **[Live, reproduced] Production routing bug**: `deploy/Caddyfile` forwards *all* of `/v1/presence/*` to `presence-server:8081`, which mounts only `/v1/presence/connect` (WS). The REST privacy-control endpoints (`GET/PUT /v1/presence/settings`, `POST/DELETE /v1/presence/consent`, `/pause`, `/resume`, `/blocks/{user_id}`) live on `cmd/server:8080` and are therefore **unreachable in production** — confirmed live against `smusic-dev.duckdns.org` (`GET /v1/presence/settings` → 404). See `.vibeflow/specs/fix-presence-rest-routing.md`.
2. ~~Backend test coverage is 72.5%~~ **Resolved 2026-09-04** — `.vibeflow/specs/backend-integration-test-coverage.md` implemented: every `internal/*/postgres/*.go` repository (`auth`, `catalog`, `library`, `presence`, `auth/mfa`) now has a real integration-tier test (`testcontainers-go`, `make test-integration`) exercising every exported method against a real, ephemeral Postgres — see `backend/README.md`'s "Integration tests" section. `internal/presence/ws/handler.go`'s `handleInbound`/`bearerToken` are now at 100% in the unit tier too. Remaining unit-tier `coverage.out` number (73.0%) is expected to stay below 100%: the `postgres/*.go` packages are deliberately `coverage:ignore`'d there (measured by the separate integration tier instead), same as `cmd/*/main.go`'s wiring exclusion — both are documented, reviewed exclusions per `00-overview.md` §2's policy, not silent gaps.
3. **Frontend coverage is effectively at policy target**: every one of the 19 packages is at 100% line coverage except the two app entrypoints (`smusic_mobile`/`smusic_web`, 31.6%, 12/38 lines) — consistent with the documented main.go/wiring exclusion, but not yet explicitly justified in a comment the way the backend's is.
4. ~~Gapless playback is not actually wired~~ **Resolved 2026-09-04** — `.vibeflow/specs/gapless-playback-engine.md` implemented: `load()` now seeds a real `ja.ConcatenatingAudioSource` (via the extracted, unit-tested `buildInitialAudioSource`), so `setNextSource()`'s guard is genuinely true at runtime. See `frontend-audio-playback.md`'s updated pattern.
5. **No CI/CD pipeline exists** (`.github/workflows` and equivalent absent). `security.md` §4/§6 makes SAST (gosec/semgrep), secret scanning (gitleaks), dependency scanning (`govulncheck`, Trivy, SBOM), and DAST (OWASP ZAP) *mandatory CI gates* for the "zero critical vulnerabilities" stop condition — none of these tools appear anywhere in the repo (Makefile, scripts, or docs) outside of the spec prose itself. This is the single largest gap against the founding prompt's stop condition. See `.vibeflow/specs/security-ci-gates.md`.
6. **MFA is still a no-op** (`internal/auth/mfa.NoopChallenger`) even though Fatia 2 (proximity) is now implemented. `security.md` §2 mandates TOTP MFA as a hard requirement to *enable the proximity feature* — this requirement is currently unenforced anywhere in the codebase. See `.vibeflow/specs/mfa-for-proximity-consent.md`.
7. ~~Catalog write endpoints have no role/ownership check~~ **Resolved 2026-09-04** — `.vibeflow/specs/catalog-write-authorization.md` implemented: `POST /v1/catalog/{artists,albums,tracks}` now require the `catalog_curator` role (`middleware.RequireRole`, `users.role` column, `auth.Service.HasRole`). No admin UI; role is granted with a manual `UPDATE`, per the spec's own anti-scope.
8. **No real audio/CDN delivery, no gRPC, no `media-edge-service`** — all explicitly documented, intentional scope cuts for this slice (not bugs), but they are the reason the "match/exceed Spotify/YouTube Music playback performance" comparison in the founding prompt cannot yet be evaluated against real infrastructure; only bucket-privacy math and architecture can be audited today, not the actual streaming latency targets in `backend-go.md` §6.
9. **`frontend/app/smusic_web/integration_test/real_backend_e2e_test.dart`** has never run to completion (documented sandbox/Chrome-debug blocker, confirmed twice independently per `00-overview.md` §3) — needs a CI machine with a real Chrome debug target before public launch.
10. No `.cursorrules`/`CLAUDE.md`/`.clinerules`/`.github/copilot-instructions.md` found in this repo — `docs/architecture/*.md` served as the sole (and unusually rich) source of project rules for this analysis.
