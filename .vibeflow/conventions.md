# Conventions — smusic

<!-- vibeflow:auto:start -->
## Sources

- `docs/architecture/00-overview.md`, `backend-go.md`, `data-architecture.md`, `frontend-flutter.md`, `security.md` — the 4 planning specs + cross-specialist decision log (via docs/architecture — these function as this project's CLAUDE.md/architecture rules; no `.cursorrules`/`CLAUDE.md`/`.clinerules` exist).
- `backend/README.md` §"Desvios da spec e justificativas" and `frontend/README.md` — the authoritative deviation log; treat these as amendments to the specs above, not violations.

## Backend (Go)

- **Module boundary rule (via `backend-go.md` §1, enforced in code)**: a module never imports another module's `postgres` package or touches its tables directly. Cross-module facts go through a small interface the other module's service satisfies structurally (e.g. `library.TrackChecker`), wired only in `cmd/server/main.go`. See `backend-module-layout.md`.
- **Layering inside every `internal/<domain>`**: `domain.go` (entities + sentinel errors) → `repo.go` (interfaces only) → `service.go` (business logic, unit-tested with fakes, zero real I/O) → `postgres/` (real pgx implementation, integration tier) → `api/` (thin chi handlers: decode → service → map response/error).
- **Dependency injection via interfaces only** (`data-architecture.md`/`backend-go.md` §7) — no singletons, no `init()` I/O side effects. `Clock` and `IDGenerator` (UUIDv7) are injected, never called directly (`time.Now()`/random IDs never appear inside domain logic).
- **No `panic` for control flow** — every error is a returned `error` value, tested via `errors.Is`/`errors.As` against sentinel errors (via `security.md`/`backend-go.md` §7). See `backend-error-handling.md`.
- **Errors**: one sentinel-error set per module in `domain.go`; one centralized `writeXError` switch per module maps errors → HTTP status. Existence-hiding is deliberate where relevant (e.g. login returns the same error for "no such user" and "wrong password").
- **Concurrency (via `backend-go.md` §3, verified genuinely idiomatic in `internal/presence/hub.go`)**: fixed-size worker pool, never goroutine-per-update; explicit non-blocking backpressure at every stage (reject, don't block, on a full channel); no global locks; `sync.WaitGroup.Add` happens once at construction time, never inside a separately-started goroutine (a real `go test -race` failure this fixed — see `hub.go`'s doc comment). See `backend-concurrency-presence.md`.
- **SQL**: always parameterized (`$1, $2, ...`); the only `fmt.Sprintf` usage found injects a positional placeholder *index*, never a value, into `ORDER BY ... LIMIT $N` clauses.
- **Testing**: `testing` + `testify/assert`, in-memory fakes for every I/O interface — no real Postgres/Redis in the unit tier. `go test -race` is expected on anything using goroutines/channels. See `backend-testing.md`.
- **Privacy-by-construction for presence** (`security.md` §1): `NearbyResult`/`userFrame` structurally cannot carry a float64/lat/lon field — asserted by a reflection-based test (`TestNearbyResult_StructurallyCannotCarryCoordinates`), not just by convention. Follow this pattern for any new payload that touches location data — prove it by test, not by review.
- **CORS**: `CORS_ALLOWED_ORIGINS` opt-in, empty by default, `*` rejected at config-load time; `AllowCredentials: false` because auth is bearer-token-in-body, never cookies. Do not add cookie-based auth without revisiting this file (`cmd/server/main.go`'s `buildRouter`).

## Frontend (Flutter/Dart)

- **Layering**: `core/` (no feature dependency) → `domain/<feature>` → `data/<feature>` → `presentation/<feature>`, one Melos package per (layer, feature) pair. Enforced mechanically by `frontend/tool/check_layer_deps.sh` (grep-based; a documented stand-in for a real `dart_dependency_validator`/custom-lint rule). See `frontend-layered-architecture.md`.
- **100% code reuse, no platform forks**: `smusic_mobile`/`smusic_web` are verified-thin entrypoints (140/115 lines, structurally identical, differing only by a literal device-id prefix string). All real UI/business logic lives in `smusic_app_shared` + the layered packages. Any change that introduces platform-conditional UI or business logic outside `core_platform`'s narrow native-adapter role breaks this hard requirement from the founding brief.
- **State management**: Riverpod `AsyncNotifier`, DI via provider overrides (no service locator, no global singletons). See `frontend-state-management.md`.
- **Networking**: single shared `dio`-based `ApiClient` in `core_networking` with auth/retry interceptors — features never construct their own HTTP client. See `frontend-networking.md`.
- **Realtime**: `ReconnectingWebSocketClient` (generic, backoff + jitter) lives in `core_networking` (not inside `social_proximity_data`) specifically so any future stream reuses it. See `frontend-websocket-realtime.md`.
- **Proximity privacy UX** (`security.md` §1.1): opt-in flow shows a dedicated value screen *before* the OS location permission prompt — never request OS location permission as the first touchpoint. See `frontend-proximity-privacy-ui.md`.
- **Testing**: `flutter test --coverage` (Flutter packages) / `dart test --coverage` (pure-Dart packages) per package via `melos run test`; `melos run analyze`/`check-layers` as companion gates. See `frontend-testing.md`.

## Don'ts

- Do NOT let a domain module import another module's `postgres` package or query its tables directly — go through the module's own service/interface (`backend-module-layout.md`).
- Do NOT call `time.Now()`, generate random IDs, or perform real I/O directly inside `internal/<domain>/service.go` — inject `Clock`/`IDGenerator`/the repo interface instead, or the service becomes untestable without real infrastructure.
- Do NOT use `panic` for control flow in backend code — return an `error`, always (project-wide rule, `backend-go.md` §7).
- Do NOT add a field that could carry a raw coordinate (float64 lat/lon, geohash, address) to any struct that crosses the presence-service → client boundary — the codebase enforces this by reflection test today; a straight code review is not sufficient given the stalking risk `security.md` §1 documents.
- Do NOT add goroutine-per-request/per-update code to the presence pipeline — always route through the fixed-size worker pool with explicit backpressure (`backend-concurrency-presence.md`'s anti-pattern section: this is exactly the "idiomatically naive Go service" failure mode the spec calls out).
- Do NOT write platform-conditional (`Platform.isAndroid`/`kIsWeb`) business logic or UI outside `core_platform`'s adapter layer — it breaks the 100%-shared-code requirement; put the platform-specific bit behind an existing interface in `core_platform` instead.
- Do NOT construct a new `dio`/`http` client per feature — always go through `core_networking`'s shared `ApiClient`.
- Do NOT request OS location permission before the user has seen the proximity feature's dedicated value/consent screen (`security.md` §1.1 — LGPD opt-in requirement, not just UX preference).
- Do NOT treat `backend/coverage.out`'s current 72.5% or any single number as "the" coverage metric without checking which files are 0% and why — `cmd/*/main.go` being 0% is the *documented, intentional* exclusion (`00-overview.md` §2); `internal/*/postgres/*.go` being 0% is an *undocumented, real gap* (no integration test tier exists at all). Don't conflate the two when judging the "100% coverage" stop condition.
- Do NOT assume `/v1/presence/*` REST endpoints are reachable through the production domain without checking `deploy/Caddyfile`'s routing first — see Known Issue #1 in `index.md`; `presence-server` only serves the WS `connect` route.
- Do NOT treat the numeric performance targets in `backend-go.md` §6 as validated — they are architecture *commitments*, explicitly stated as needing load-test validation before being claimed true; no load test results exist in this repo.
<!-- vibeflow:auto:end -->
