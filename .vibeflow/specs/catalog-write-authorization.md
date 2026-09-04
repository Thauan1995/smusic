# Spec: Restrict catalog write endpoints to an authorized role

## Objective
Stop any authenticated user from being able to create/modify shared catalog data (artists/albums/tracks), replacing the current "any logged-in user" gate with a real role check.

## Context
`backend/README.md`'s "Desvios da spec" #5 documents this as a known, intentional-for-now gap: `POST /v1/catalog/{artists,albums,tracks}` are "gated behind authentication (any authenticated user) as a minimal guard — a real admin/ingest role is a TODO (role-based authz doesn't exist yet)." Confirmed live during this analysis: the QA test user created purely to seed sample data for functional testing was able to create an artist, album, and two tracks with no special privilege — any signup can pollute or vandalize the shared catalog today. This is low severity while there's no public signup and no real content pipeline, but it's a straightforward privilege-escalation-shaped gap that should close before any wider audience has access, and it's exactly the kind of finding `security.md` §5's STRIDE table calls "Elevação de privilégio" risk for the catalog surface.

## Definition of Done
- [ ] `users` gains a role/flag distinguishing at least "regular user" from "catalog admin/curator" (schema choice is an implementation detail — an enum column, a separate `user_roles` table, or reusing `status` are all acceptable; pick the smallest change consistent with `data-architecture.md`'s existing `users` table shape).
- [ ] `POST /v1/catalog/artists`, `POST /v1/catalog/albums`, `POST /v1/catalog/tracks` return `403` for an authenticated user without the required role, and continue to succeed for one with it.
- [ ] At least one user (seed/migration/manual grant path — doesn't need an admin UI) can be granted this role for operational use (e.g. seeding the catalog for a demo).
- [ ] Unit tests cover both the allowed and denied path using the existing in-memory fake pattern (`backend-testing.md`), and the existing `catalog` service/API tests are updated to reflect the new requirement rather than silently starting to fail.
- [ ] `backend/README.md`'s "Desvios da spec" #5 entry is updated to reflect that the TODO is resolved (or narrowed, if this spec deliberately ships a minimal role model rather than a full RBAC system — say so explicitly).
- [ ] No violation of `conventions.md` Don'ts.

## Scope
- `internal/auth` (or a small new `internal/authz`, if a role concept doesn't fit cleanly into `auth`'s existing domain model — prefer extending `auth` first per the module-boundary convention, only split out if it would otherwise leak catalog-specific concerns into `auth`).
- `internal/catalog/api`'s three write handlers — add a role check alongside the existing `RequireAuth` middleware.
- A migration adding whatever role column/table is chosen.

## Anti-scope
- Do NOT build a general-purpose RBAC/permissions system — this spec's job is closing one concrete gap (catalog writes), not designing a role framework for every future admin action. A minimal boolean-ish "is this user allowed to write catalog data" check is sufficient; generalize later if/when a second use case actually needs it.
- Do NOT build an admin UI in the frontend for granting this role — out of scope; a manual DB grant or a small CLI/migration seed is enough for now, matching this project's current "no public signup yet" stage.
- Do NOT touch `library` or `playback` endpoints — this gap is specific to catalog writes; other modules' authorization is unaffected and out of scope here.

## Technical Decisions
- **Extend `auth`'s `users` table over a new microservice-style permissions table**: matches `data-architecture.md`'s existing relational model and this codebase's stated preference (in `backend-go.md` §1's justification) for avoiding premature structural complexity — a role concept this narrow doesn't need its own bounded context yet.

## Applicable Patterns
- `backend-module-layout.md` — role check lives in the service layer (`catalog.Service.CreateArtist`/`CreateAlbum`/`CreateTrack`, or a shared authz check called from there), not just the HTTP handler, so it's testable without spinning up the router.
- `backend-error-handling.md` — a new sentinel error (e.g. `ErrForbidden`) mapped to `403` in catalog's `writeCatalogError`.

## Risks
- **Risk**: if the role model is too minimal, it becomes awkward to extend later (e.g. hardcoded boolean instead of an extensible enum). **Mitigation**: use an enum-shaped column even for a two-value role today (`'user' | 'catalog_curator'`), so adding a third role later doesn't require another migration + rename.
- **Risk**: the QA test data created during this analysis session (an artist, album, and 2 tracks under a non-privileged account) will need to either be retroactively re-owned/re-authorized or cleaned up once this ships, since it was created under the old any-authenticated-user rule. **Mitigation**: not this spec's job to clean up — flag it to whoever runs this spec so it isn't mistaken for a bug in the new check ("why does old QA data still exist despite the new role gate" — because the gate is not retroactive).
