# Decision Log
> Newest first. Updated automatically by the architect agent.

## 2026-09-05 — Two real bugs found by actually loading the live site, both fixed

User reported (after the deploy-verification exchange) that icons render as "a box with an X" and asked for the color roles to be corrected: black primary, red/white secondary (overriding this session's earlier `brand-color-system-red-black-white` decision, which had made red the primary accent and black the surface). Investigated with `claude-in-chrome` against the actual `smusic-dev.duckdns.org` deployment rather than guessing from source.

**Bug 1 — icons never rendered at all (pre-existing, unrelated to any change this session made)**: neither `smusic_web/pubspec.yaml` nor `smusic_mobile/pubspec.yaml` declared `flutter: uses-material-design: true`. Confirmed via the browser console (`Could not find a set of Noto fonts to display all missing characters`) and via a local `flutter build web`, which showed `MaterialIcons-Regular.otf` was never produced without the flag. Fixed by adding the flag to both app entrypoints' pubspecs — verified by inspecting the rebuilt `build/web` output (font asset present, tree-shaken to ~11KB) and by loading the rebuilt app in a real browser tab.

**Bug 2 — color roles reworked, and two follow-on rendering bugs caught along the way**: rewrote `SmusicTheme._build` to make black `colorScheme.primary` and red `colorScheme.secondary` (previously: red was primary/accent, black was only the surface). This surfaced two real problems only visible by actually loading the app, not by reading the theme code:
1. `.copyWith(primary:, secondary:)` alone leaves `primaryContainer`/`secondaryContainer` (used by the FAB and `NavigationBar`'s selected-destination indicator) on the seed algorithm's own derived tones — rendered off-brand purple. Fixed by setting every `*Container`/`on*Container` role explicitly too.
2. Seeding `ColorScheme.fromSeed` from pure black *still* tinted every un-overridden neutral role (`surfaceContainerHighest` — `SkeletonBox`'s shimmer base — included) faintly pink, because Material 3's HCT algorithm assigns hue 0° (red) to zero-chroma seeds by convention. Fixed by also passing `surface`/`onSurface`/`surfaceContainerHighest` as explicit `fromSeed` overrides (this Flutter version's `fromSeed` factory accepts a named override for every role directly, not just via a later `.copyWith`), rather than trusting any seed-derived neutral tone.

Both bugs were caught by an actual visual load-test loop (build web locally, serve it, sign up a real test account through the real UI, screenshot, iterate) rather than by only running `flutter analyze`/`flutter test` — neither would have caught either bug, since both are runtime-rendering issues invisible to static analysis and to widget tests that don't assert on rendered pixel colors.

`colors_test.dart` extended with contrast checks for the new `black`/`primaryElevatedDark` roles; `frontend-design-system.md` updated with both bugs' root causes, specifically so a future theme edit doesn't reintroduce either.

## 2026-09-04 — `skeleton-loading-player-and-proximity` implemented, pushed

Added two new shared skeleton widgets to `core_design_system` (both built from `SkeletonBox`, matching `TrackListSkeleton`'s existing composition pattern): `NowPlayingSkeleton` (album art + title/artist + seek bar + transport-row placeholders — a single now-playing view isn't a list of rows, so `TrackRowSkeleton`'s shape doesn't fit) and `NearbyListSkeleton`/`NearbyListenerSkeleton` (circular avatar + text + trailing distance-badge placeholder — checked `nearby_listener_card.dart`'s actual layout first, per the spec's own instruction, and confirmed it's meaningfully different from a track row, justifying a dedicated widget rather than reusing `TrackListSkeleton`).

Wired into `player_screen.dart`'s top-level `playerStateAsync.loading()` branch and `proximity_list_screen.dart`'s `feedAsync.loading()` branch — **not** `proximity_list_screen.dart`'s earlier settings-fetch gate, which stays a plain spinner (it's a "which screen do we even show" decision, not list content, matching the spec's own anti-scope on form/decision screens).

**Testing note**: both skeletons' shimmer animation (`SkeletonBox`'s repeating `AnimationController`) repeats forever, so `pumpAndSettle()` can never be used to reach these loading states in a test — it would spin until its own timeout. Used bounded `pump()` calls instead (matching the existing `player_screen_test.dart` pattern for its pre-existing spinner test), which is why the new `proximity_list_screen_test.dart` test needed 2 explicit pumps rather than 1. Updated both screens' existing loading-state test assertions to check for the new skeleton types specifically (not just "some loading indicator"), added a new test for `proximity_list_screen.dart`'s feed-loading state (previously untested entirely), and 2 new widget tests for the skeletons themselves in `core_design_system`.

`melos run test`/`analyze`/`check-layers` all green across the whole workspace. This closes all 3 UI/UX specs from the 2026-09-04 audit.

## 2026-09-04 — `icon-system-consistency` implemented, pushed

Applied one rule (filled = active/in-progress, outlined = inactive/available action) to the 3 inconsistent pairs the UI/UX audit found: `player_screen`'s play/pause button (was always filled regardless of state — now `pause_circle_filled`/`play_circle_outline`), `PauseDiscoveryToggle` (was always outlined — now `pause_circle_filled`/`play_circle_outline`, matching player_screen exactly), and `NavigationShell`'s bottom-bar/rail destinations, which had **no selected-vs-unselected icon distinction at all** (confirmed the audit's prediction) — added `selectedIcon` to `_NavDestination` and both `NavigationDestination`/`NavigationRailDestination`.

`nearby_listener_card.dart`'s anonymous/placeholder avatar icons were audited and found to already match the rule (anonymous=outlined/minimal, identified=filled/fuller) — left unchanged rather than "fixed" for its own sake. `search_result_row.dart`'s `Icons.person` for artist rows is a category glyph with no selected/unselected concept, not a state toggle — out of this spec's scope per its own anti-scope ("only style for an already-chosen icon... if choice seems wrong, note it, don't swap it").

Updated 2 existing test assertions to match the new icon choices, added 2 new tests in `shared_navigation` asserting every destination has a distinct `icon`/`selectedIcon`. All packages green (`melos run test`/`analyze`/`check-layers`).

## 2026-09-04 — `brand-color-system-red-black-white` implemented, pushed

Replaced `SmusicColors.brandSeed` (`0xFF1ED760` — literally Spotify's brand green, per the spec's own finding) with a deliberate palette: `brandRed` (`0xFFC8102E`, accent only, never a dominant background), `surfaceBlack` (`0xFF121212`, forced as dark mode's real surface rather than trusting `ColorScheme.fromSeed`'s lighter red-tinted derivation), `pureWhite`. Kept `error` (`0xFFE74C3C`) unchanged but verified — not assumed — that it's visually distinct from `brandRed`.

**Contrast actually computed, not eyeballed** (the spec's explicit DoD requirement): wrote a real WCAG luminance/contrast calculator in `colors_test.dart` rather than a one-off script — white-on-brandRed = 5.88:1, error-on-surfaceBlack = 4.90:1, both clear AA's 4.5:1 for normal text. The hue gap (~350° vs ~6°, i.e. ~15° apart) plus ~15pt lightness gap keeps brand and error readable as distinct reds.

Regenerated the one golden-image test in the repo (`smusic_primary_button_golden_test.dart`) since the actual rendered button color changed — a real, expected update, not a masked regression. No other call sites reference the old color name (confirmed by grep) — the "100% of color goes through Theme.of(context)" finding from the original audit held, so this was a token+theme-file-only change, no screen edits needed.

## 2026-09-04 — `performance-benchmark-harness` implemented, first baseline measured, pushed

Built `backend/loadtest/http` (vegeta-as-a-library: search/play/seek p50/p95) and `backend/loadtest/presence` (a Go WS client: N simulated clients each sign up, enroll+verify real TOTP MFA, grant consent, connect, heartbeat — measures fanout latency by watching for the first heartbeat response that reflects a "trigger" client's fresh update). Neither existed before; `backend-go.md` §6's numeric targets had never been measured against a running instance, only asserted as architecture commitments.

Ran both against a local stack (Postgres/Redis in Docker, `cmd/server`+`cmd/presence-server` via `go run`) — deliberately not against `smusic-dev.duckdns.org`, per the spec's own risk note about not load-testing a home-lab machine serving real traffic without explicit authorization. Results (`docs/architecture/performance-baseline.md`): all measured metrics PASS, by a wide margin (sub-5ms HTTP, sub-millisecond presence fanout at N=10/50/200) — expected on loopback with no real network/CDN, documented as such, not oversold as production-representative.

**Real findings surfaced by actually running this**: (1) the search harness itself had a bug — an un-escaped query string with spaces caused every search request to 400, caught immediately by the harness's own success-rate reporting; (2) the per-IP login rate limiter (`security.md` §4, working as designed) makes spinning up hundreds of test accounts from one source IP impractical — this session's local run raised `LOGIN_RATE_LIMIT_PER_MINUTE` on the disposable local test instance only, documented in the baseline report as something that must never be done against a real deployment.

Gosec caught one real finding in the new code (`G306`, report file written `0644` instead of `0600`) — fixed. All gates green: build/vet/staticcheck, `go test -race`, gosec 0 issues, govulncheck 0 reachable.

`.vibeflow/index.md`'s Known Issue #8 updated: partially resolved (search/play/seek/presence-fanout now measurable and measured; CDN-dependent "time to first audio" still genuinely out of reach without the still-deferred media-edge-service).

This closes the 7 original specs from the first `/vibeflow:analyze` pass.

## 2026-09-04 — `gapless-playback-engine` implemented, pushed

`JustAudioNativeEngine.load()` always built a plain, non-concatenating source, so `setNextSource`'s `current is ja.ConcatenatingAudioSource` guard could never be true (confirmed by the earlier UI/frontend fork's audit, and by `00-overview.md`'s own tracked tech-debt item #1). Fixed: `load()` now seeds a `ja.ConcatenatingAudioSource` via a new pure, top-level `buildInitialAudioSource` function.

**Testing note**: the whole `JustAudioNativeEngine` class is `coverage:ignore`'d (needs a real platform channel), but `ConcatenatingAudioSource.add()` doesn't touch the platform channel until the source has actually been attached to a real `AudioPlayer` via `setAudioSource` — so a hermetic unit test could construct one via `buildInitialAudioSource`, call `.add()` on it directly (exactly the sequence `setNextSource` performs), and assert on `.children` — genuinely testing the fix's logic without needing platform-channel mocking. Verified: `flutter analyze`/`melos run test`/`check-layers` all green across the whole workspace.

`frontend-audio-playback.md`'s Anti-patterns section and `docs/architecture/00-overview.md`'s tech-debt item #1 both marked resolved.

## 2026-09-04 — `catalog-write-authorization` implemented, all gates green, pushed

Any authenticated user could write shared catalog data before this (confirmed live during the initial functional analysis). Added `users.role` (`migrations/0004_catalog_role.up.sql`, enum-shaped even for two values today: `'user'`/`'catalog_curator'`, DB-level `CHECK`), `auth.Service.HasRole` (satisfies a new `middleware.RoleChecker` interface structurally — catalog still never imports auth directly, per backend-go.md §1: the required-role string `"catalog_curator"` is a literal in `catalog/api`, not an imported constant), and `middleware.RequireRole` (mirrors the existing `RequireAuth` pattern) gating `POST /v1/catalog/{artists,albums,tracks}`.

`SignUp`/`LoginWithOAuth` now set `Role: RoleUser` explicitly on the constructed `User` (matching how `Status` was already handled) rather than relying on the DB column default — removed a fake-vs-production behavior mismatch this surfaced during testing. No admin UI ships (per the spec's anti-scope): granting the role is a manual `UPDATE users SET role = 'catalog_curator' WHERE id = ...`, documented in the migration's own comment and `backend/README.md`.

Tests added at both layers: `auth.Service.HasRole` (unit, in-memory fake), `middleware.RequireRole` (unit, table of 200/403/401/500 outcomes), and one API-level 403 test for the catalog write group (shared middleware chain — didn't duplicate per-route, all three write routes share the identical gate). All 6 integration-test files updated (`Role: auth.RoleUser` added to every fixture `auth.User{}` literal — the CHECK constraint would otherwise reject the empty-string default).

All gates verified: build/vet/staticcheck, `go test -race` (unit) and `go test -tags=integration` (real Postgres) both fully green, gosec 0 issues, govulncheck 0 reachable.

`backend/README.md`'s deviation #5 and `.vibeflow/index.md`'s Known Issue #7 updated to reflect resolution.

## 2026-09-04 — `fix-presence-rest-routing` implemented, validated locally, pending production redeploy

Fixed `deploy/Caddyfile`: `/v1/presence/connect` (WS) now has its own specific `handle` block before the general `/v1/*` rule; every other `/v1/presence/*` path (settings/consent/pause/resume/blocks) now correctly falls into `/v1/* -> server:8080` instead of the old catch-all `/v1/presence/* -> presence-server:8081` that 404'd them all.

Validated against a full local stack (Postgres+Redis via `docker run`, `cmd/server`/`cmd/presence-server` via `go run` on test ports, Caddy 2 in Docker with a routing-equivalent no-TLS test Caddyfile) — not against the real `smusic-dev.duckdns.org` host, which this session has no SSH/deploy access to (see `smusic-deploy-topology` memory). Reproduced the exact prior failure (404 on `GET /v1/presence/settings`) with the *old* logic, then confirmed the fix: `settings`/`consent`/`pause` all return 200, and the WS route still completes a real upgrade handshake (101 Switching Protocols) — no regression.

**Known follow-up, deliberately NOT fixed here (anti-scope in the spec)**: `scripts/tools/path_proxy.py` (local-dev proxy) has the identical bug — its docstring says it mirrors the Caddyfile's routing, and its code does route all `/v1/presence/*` to the presence port. Left untouched per the spec's anti-scope; worth a one-line follow-up spec if local-dev testing of presence REST endpoints (via the proxy) is ever needed.

**Still open**: production redeploy. This session cannot reach the actual host (no matching SSH config, no local Docker/Caddy process for it). User confirmed (2026-09-04) they'll do the redeploy themselves — `git pull` + restart the `caddy` service (`docker compose -f deploy/docker-compose.prod.yml --env-file deploy/.env.prod up -d --build caddy` per `deploy/README.md`), then this spec's live DoD check (`curl https://smusic-dev.duckdns.org/v1/presence/settings` with a bearer token) can be re-run for final confirmation. Committed as `d4c6136`, pushed to `origin/master`.

## 2026-09-04 — `security-ci-gates` implemented, all gates green locally, pushed

Added `.github/workflows/{backend-ci,frontend-ci,security-secrets}.yml` and `SECURITY-EXCEPTIONS.md`. Ran every gate locally before pushing: `go build/vet`, `staticcheck`, `go test -race` (25 packages, 0 failures), `govulncheck` (0 reachable vulnerabilities; 17 advisories in the transitive, never-imported `golang.org/x/crypto/ssh` — documented as the one dependency-level exception in `SECURITY-EXCEPTIONS.md`), `gosec` (0 issues after fixes), `gitleaks detect` against full git history (16 commits, 0 leaks — the one filesystem-level hit on `backend/.env` is the correctly-gitignored local dev-secrets file, not a leak), `melos run check-layers`/`analyze`/`test` (all green).

**Real fix, not just gate-satisfying suppression**: `internal/presence/bucket.go`'s ±75m spatial jitter (the core anti-triangulation privacy defense, `security.md` §1.2) was using `math/rand`; switched to `crypto/rand`. This changed `Jitterer.Jitter`'s signature to return `(GeoPosition, error)` — the crypto-failure path is threaded up as a normal error (per `conventions.md`'s no-panic rule), mirroring the exact pattern `token.SecureRefreshGenerator.New`/`password.Hasher.Hash` already use for the same kind of failure. Two of the four gosec findings were genuine false positives/needed-a-real-guard, resolved with a bounds check (`argon2.go`, G115) and one justified `#nosec` (`catalog/postgres/repo.go`, G101 — SQL text, not a credential).

Committed as `66cee89` (`.vibeflow/` docs), `d4c6136` (routing fix), `4e333ac` (this spec) and pushed to `origin/master`. `gh run list` to pull the actual GitHub Actions run logs was blocked by this session's auto-mode classifier (read-only external-account action) — the user can check `github.com/Thauan1995/smusic/actions` directly; every command the workflows run was independently verified locally first with identical results expected.

## 2026-09-04 — `mfa-for-proximity-consent` implemented, all gates green

`internal/auth/mfa.TOTPChallenger` (RFC 6238 via `pquerna/otp`) replaces `NoopChallenger` for the one call site security.md §2 makes mandatory: `presence.SettingsService.GrantConsent` now calls a new `presence.MFAChecker` (implemented by `auth.Service.HasVerifiedMFA`, wired only in `cmd/server/main.go` — presence still never imports auth directly) and returns `ErrMFARequired` (`403 mfa_required`) if the user has no verified factor. New REST surface: `POST /v1/auth/mfa/enroll` (returns secret + `otpauth://` URI), `POST /v1/auth/mfa/verify` (`{code}`, activates the factor on first success). New table `user_mfa_totp` (migration `0003_mfa`).

`auth.Service.NewService` gained an `MFAProvider` parameter — a real, structural interface-boundary change. Notable design point: `TOTPChallenger.Verify` only activates the factor (`VerifiedAt`) on the *first* correct code, matching how every mainstream authenticator-app enrollment flow works (scan → confirm one code → active), so a never-completed enrollment can't silently count as "MFA is on."

All gates verified locally: `go build/vet`, `staticcheck` (on the intentionally-committed files — see below), `go test -race` (all packages green, `internal/auth` and `internal/auth/api` at 100% statement coverage including the new MFA methods), `gosec` (0 issues), `govulncheck` (0 reachable, same 17-advisory `x/crypto/ssh` exception as before).

**Anomaly, noted not fixed**: an untracked `internal/auth/mfa/fakes_test.go` (a `fakeSecretRepo` duplicate of this spec's own `totp_test.go` fake) appeared on disk during this work, not written by this session's own tool calls — most likely a scope violation by the concurrently-running UI/UX-specialist fork (dispatched by the user mid-session, directed to touch only `.vibeflow/specs/*.md` and explicitly told not to touch `.go` files), sharing this same non-worktree checkout. Left untouched to avoid a concurrent-write race; deliberately excluded from this spec's commit (`git add` was scoped explicitly, not `-A`). To reconcile once the fork's report comes back — see MEMORY.md.

Frontend MFA-enrollment UI is intentionally NOT built in this pass: the spec's own Definition of Done (re-read before implementing) contains no frontend-specific check, only backend behavior + tests — flagged to the user as a fast-follow rather than silently expanding scope to match the spec's "Scope" prose, which does mention a frontend step.

## 2026-09-04 — `backend-integration-test-coverage` implemented, all green, pushed

Added `internal/platform/dbx/dbxtest` (shared testcontainers-go Postgres fixture: ephemeral container, real migrations via the same golang-migrate mechanism `cmd/migrate` uses, returns a pool via `dbx.NewPool`) and one `repo_integration_test.go` per postgres package (`auth`, `catalog`, `library`, `presence`, and `auth/mfa` — added after the fact since MFA landed mid-session too), each exercising every exported method with a real query round-trip. Caught one genuine bug while writing these: a `time.Time` equality assertion compared a UTC-constructed instant against one pgx scanned back in the process's local timezone — fixed by comparing via `.Equal()` instead of `assert.Equal` (same instant, different `*time.Location`, which deep-equality wrongly flags).

New `make test-integration` + a separate CI job; `backend/README.md` updated. `go mod tidy` (pulled in by `testcontainers-go`) bumped `golang.org/x/crypto`, dropping the unreachable-transitive-vuln count from 17 to 4 (`SECURITY-EXCEPTIONS.md` updated). Unit-tier `coverage.out` stays at ~73% by design — the postgres packages are measured by this new integration tier instead, not merged into that number, matching `backend-go.md` §7's pyramid.

Committed as `2eb7158` (+ `6036fb6` for the UI/UX specs, see below), pushed to `origin/master`.

**Correction (parent session, same day):** the "anomaly" above was not a third party — it was the UI/UX-specialist fork the user had dispatched in parallel (agent `abecc38f7ea191ce4`), which ignored its actual directive (audit design/colors/icons, write only new specs) and re-did this MFA work instead, then committed+pushed it as `7df6088`. It produced zero UI/UX specs. Independently re-verified this MFA implementation regardless (build/vet/staticcheck/race tests/gosec/govulncheck all clean, 100% coverage on `internal/auth`+`internal/auth/api`) — it's correct, just from the wrong agent. See `fork_scope_anomaly` memory for the full account. **Redispatched with `isolation: "worktree"`** (agent `a46ac60a6ebeb9ff7`) — this time it stayed in scope: 3 specs (`brand-color-system-red-black-white.md`, `icon-system-consistency.md`, `skeleton-loading-player-and-proximity.md`) + `patterns/frontend-design-system.md`, copied back from its worktree and committed as `6036fb6`.
