# Spec: Add integration-test tier for Postgres repository implementations

## Objective
Close the backend's largest test-coverage gap — every `internal/*/postgres/*.go` repository implementation is at 0% coverage — by adding the `testcontainers-go`-based integration tier `backend-go.md` §7 specifies but that was never built.

## Context
`backend/coverage.out` (`go tool cover -func`) shows total coverage at **72.5%**, well under `00-overview.md` §2's policy ("100% de cobertura em todo código escrito à mão... excluindo código gerado... e wiring de main.go"). Per-package inspection shows the service layers (business logic, unit-tested with in-memory fakes) are in good shape, but **every single function in every `internal/{auth,catalog,library}/postgres/repo.go`** (and `internal/platform/dbx/postgres.go`'s `NewPool`) is at exactly 0.0% — e.g. `auth/postgres/repo.go`: `Create`, `GetByEmail`, `GetByID`, `Store`, `GetByHash`, `Revoke`, `MarkReplaced`, `RevokeAllForUser` all 0%; `catalog/postgres/repo.go`: `Create`, `GetByID`, `Search`, `ListByArtist`, `ListAudioAssets` all 0%; same pattern in `library/postgres/repo.go`. `go.mod` has no `testcontainers-go` dependency — the integration tier `backend-go.md` §7.3 calls for ("sobem dependências reais via containers efêmeros... testando o módulo de domínio completo... contra um banco real — cobre migrations, queries SQL reais, comportamento real do driver") does not exist at all. This is not a "hard to test, documented exception" gap like `cmd/*/main.go` — it's real, untested SQL that has never been exercised by an automated test against a real Postgres.

## Definition of Done
- [ ] `testcontainers-go` (with the Postgres module) is added as a backend dev dependency.
- [ ] An integration test file exists for `internal/auth/postgres`, `internal/catalog/postgres`, `internal/library/postgres`, and `internal/presence/postgres` (check its coverage too — likely the same gap), each spinning up a real ephemeral Postgres container, running `migrations/*.up.sql` against it, and exercising every exported repo method at least once with a real query round-trip.
- [ ] `go tool cover -func` on the merged unit+integration run shows every method in the four `postgres/` packages above at >0%, with the true business-logic paths (not just "called once") covered — e.g. `Search`'s cursor pagination, `GetByEmail`'s not-found path, `MarkReplaced`'s refresh-token-reuse-detection path (security-critical, per `backend/README.md`'s Auth section).
- [ ] `internal/platform/dbx.NewPool` gets at least a smoke-test integration check (connects to the ephemeral container successfully).
- [ ] Integration tests are gated behind a separate `make` target (e.g. `make test-integration`, distinct from the existing `make cover`) so the fast unit-test loop (`backend-go.md` §7's "<30s in CI" requirement for the unit tier) isn't slowed down — and wired into CI as its own job (coordinate with `.vibeflow/specs/security-ci-gates.md` if that lands first).
- [ ] `backend/README.md`'s test-running instructions are updated to document the new `make test-integration` target and its Docker requirement.
- [ ] No violation of `conventions.md` Don'ts (in particular: integration tests still don't call real Postgres from anything in the unit tier — they're additive, not a replacement).

## Scope
- New integration test files under each `internal/<domain>/postgres/` package (or a sibling `_integration_test.go` following Go's existing build-tag/naming convention for separating fast vs slow tests — pick whichever this codebase's existing test files already lean toward, defaulting to a `//go:build integration` tag if none exists).
- `testcontainers-go` wiring (container lifecycle, migration application) — a small shared test helper is fine (e.g. `internal/platform/dbx/testhelper` or similar), reused by all four packages' integration tests rather than duplicated four times.
- `Makefile` target + CI job wiring.

## Anti-scope
- Do NOT rewrite any production `postgres/*.go` code as part of this spec — the goal is coverage of existing, presumably-correct SQL, not a refactor. If a real bug is found while writing these tests, file it as a separate, scoped fix (don't silently expand this spec).
- Do NOT add integration tests for `internal/playback/redisstore` or `internal/presence/redisstore` in this spec — those already have `miniredis` (an in-process fake, already a dependency) covering them at the unit tier per `backend-testing.md`; this spec is Postgres-only. A Redis-integration follow-up is a separate, smaller spec if `miniredis` fidelity is ever in question.
- Do NOT chase 100% of every branch inside the postgres packages if a branch is a genuinely-impossible defensive check (e.g. a `json.Marshal` on a static struct that can't fail) — per `backend-go.md` §7's own explicit guidance, document the exclusion instead of writing an artificial test for it.

## Technical Decisions
- **`testcontainers-go` over a shared long-lived test DB**: matches `backend-go.md` §7.3's explicit recommendation ("testcontainers-go para Postgres/Redis... isolados por schema/container por suíte") and keeps tests hermetic/parallelizable, consistent with this codebase's existing preference for fakes over shared mutable test state.

## Applicable Patterns
- `backend-module-layout.md` — tests live alongside the code they cover, following the existing per-module test-file convention.
- `backend-testing.md` — this spec adds the "integration" rung to the pyramid `backend-go.md` §7 describes; unit tests (with fakes) stay exactly as they are, untouched.

## Risks
- **Risk**: CI runtime grows once real containers are spun up per suite. **Mitigation**: the DoD's separate `make test-integration` target and separate CI job keep this off the fast inner-loop path.
- **Risk**: this repo currently has no CI at all (see `.vibeflow/specs/security-ci-gates.md`) — if that lands first, wire the integration job into it directly; if this lands first, at minimum document the `make` target so a human runs it before merge until CI exists.
