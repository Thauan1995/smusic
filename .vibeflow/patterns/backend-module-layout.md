---
tags: [go, module-layout, layering, architecture, ddd]
modules: [backend/internal/auth/, backend/internal/catalog/, backend/internal/library/, backend/internal/playback/, backend/internal/presence/]
applies_to: [services, repositories, handlers, domain-models]
confidence: inferred
---
# Pattern: Backend module layout (domain/repo/service/postgres/api)

<!-- vibeflow:auto:start -->
## What
Every backend domain module (`auth`, `catalog`, `library`, `playback`, `presence`) follows the identical 5-file layering described in `backend/README.md`'s "Module layout" section and `docs/architecture/backend-go.md` §1/§7: `domain.go` (entities + sentinel errors), `repo.go` (repository interfaces), `service.go` (business logic, depends only on interfaces), `postgres/` (real pgx implementation), `api/` (thin chi HTTP handlers). Other modules never reach into a module's Postgres tables directly — only through its exported interfaces/service.

## Where
`backend/internal/{auth,catalog,library,playback,presence}/` — every domain package. `backend/internal/platform/*` holds cross-cutting infra (config, dbx, cache, logging, middleware, clock, idgen, httpx) that every domain module depends on but that itself has no business logic.

## The Pattern
1. `domain.go` — entities as plain structs, string-const enums, and a block of sentinel `errors.New(...)` values wrapped with `%w` by the service layer.
2. `repo.go` — narrow repository interfaces (one per aggregate), implemented both by a real Postgres adapter and, in tests, by hand-written in-memory fakes.
3. `service.go` — a `Service` struct holding only interfaces (repos, `clock.Clock`, `idgen.Generator`, and any narrow deps like `Signer`/`Hasher`) constructed via `NewService(...)`. All validation and business rules live here; every branch is unit-testable with fakes, no real Postgres/Redis needed.
4. `postgres/repo.go` — implements the module's repo interfaces against `*pgxpool.Pool` with parameterized queries (`$1, $2...`), translating Postgres-specific errors (e.g. unique-violation) back to the module's sentinel errors.
5. `api/handlers.go` — a `Handler` struct wrapping a narrow `Service` interface (re-declared locally, not the concrete `*module.Service`, so handler tests can inject fakes). `Mount(r chi.Router, ...)` registers routes. Handlers only decode JSON → call the service → map the result/error to a response. All error-to-HTTP-status mapping happens in one `writeXError` switch per module.

## Rules
- Never import a sibling module's `postgres` package — only its top-level package (service/domain) or its `api` package for wiring.
- `service.go` must not import `net/http`, `pgx`, or `redis` — only `context`, stdlib, and this module's own `repo.go` interfaces plus `internal/platform/{clock,idgen}`.
- Every sentinel error in `domain.go` is prefixed with the module name (e.g. `"auth: invalid input"`, `"presence: consent not granted"`).
- `NewService` takes every dependency as a constructor parameter (no globals, no `init()` wiring) — this is what makes fakes injectable in tests.

## Examples from this codebase
File: `backend/internal/auth/service.go`
```go
type Service struct {
	users         UserRepository
	identities    IdentityRepository
	devices       DeviceRepository
	refreshTokens RefreshTokenRepository
	hasher        Hasher
	signer        Signer
	...
}
```

File: `backend/internal/presence/domain.go` + `backend/internal/presence/redisstore/geoindex.go`
Presence follows the same shape but swaps `postgres/` for `redisstore/` as its ephemeral-state adapter (`GeoIndex` interface in `domain.go`/a sibling file, implemented by `redisstore.Store`) — proof the layering generalizes beyond Postgres-backed modules.
<!-- vibeflow:auto:end -->

## Anti-patterns
None found — the layering is applied with unusual consistency across all 5 modules, likely because `docs/architecture/backend-go.md` was written up front as a binding spec before implementation began.
