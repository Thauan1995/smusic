# Spec: Use shape-matched skeleton loading states in the player and proximity screens

## Objective
Replace generic spinners with content-shaped skeleton placeholders on the two screens that don't yet use the pattern the design system already provides and two other screens already use correctly — the player ("Now Playing") and proximity ("Perto de mim") screens.

## Context
`core_design_system` ships a real skeleton-loading primitive (`track_row_skeleton.dart`'s `TrackListSkeleton`, built on `SkeletonBox`) — the same "content-shaped gray placeholder that mimics the eventual layout" pattern Spotify/YouTube Music use everywhere a list is loading, specifically so the UI doesn't visually "jump" once data arrives and so a load feels faster than an equivalent blank-screen-plus-spinner (this is a well-established perceived-performance technique, not just decoration).

Adoption is inconsistent:
- `library_ui/lib/src/screens/library_screen.dart:29` and `search_screen.dart:46` **correctly** use `TrackListSkeleton()` for their loading state.
- `player_ui/lib/src/screens/player_screen.dart:29,74` uses a bare `CircularProgressIndicator()` for both the initial screen load and the async player-state loading branch — this is the single most-used screen in any music app (per the founding brief's own emphasis on matching Spotify/YT Music playback quality) and the one with the least polished loading state today.
- `social_proximity_ui/lib/src/screens/proximity_list_screen.dart` (loading branch, check its `.when`/`AsyncValue` handling) also falls back to a generic spinner rather than a shape-matched placeholder for the "who's nearby" list — the product's core differentiator feature.

## Definition of Done
- [ ] `player_screen.dart`'s loading state renders a skeleton shaped like the eventual "Now Playing" layout (album art placeholder, title/artist text-bar placeholders, control-row placeholder) — reuse `SkeletonBox` primitives from `core_design_system` to compose it; a new `NowPlayingSkeleton` widget in `core_design_system` is appropriate if this shape doesn't fit `TrackListSkeleton`'s existing list-row shape (it won't — a single now-playing view isn't a list of rows).
- [ ] `proximity_list_screen.dart`'s loading state uses `TrackListSkeleton` (or a `NearbyListSkeleton` variant, if the nearby-card shape differs meaningfully from a track row — check `nearby_listener_card.dart`'s actual layout before deciding) instead of a bare spinner.
- [ ] Both screens' existing tests (`player_screen_test.dart`, proximity list screen's test, if present) are updated to assert on the new skeleton widget appearing during the loading state, not just that *some* loading indicator exists — a test that only checks "a widget of type X or a spinner" isn't a meaningful regression guard for this specific fix.
- [ ] No other screen's loading-state behavior changes in this pass (see Anti-scope).
- [ ] No violation of `conventions.md` Don'ts — new skeleton widgets, if added, go in `core_design_system` (shared), never duplicated locally inside `player_ui`/`social_proximity_ui`, matching this codebase's existing layering rule.

## Scope
- `player_ui/lib/src/screens/player_screen.dart`'s two loading branches (lines 29 and 74 in the version audited).
- `social_proximity_ui/lib/src/screens/proximity_list_screen.dart`'s loading branch.
- Any new skeleton widget this requires, added to `core_design_system/lib/src/widgets/`, exported from `core_design_system.dart` alongside the existing `TrackListSkeleton`/`SkeletonBox`.
- Updating the two screens' existing tests to match.

## Anti-scope
- Do NOT touch `library_screen.dart`/`search_screen.dart` — they already do this correctly; re-touching them risks an unrelated regression for zero benefit.
- Do NOT add skeleton loading states to `auth_ui`'s login/signup screens or `social_proximity_ui`'s value/permission/settings screens — those are form/decision screens, not content-list screens; a skeleton loader doesn't make sense for "waiting for a login request," which is what a determinate/short spinner or disabled-button-with-inline-spinner already communicates correctly. Don't apply this pattern where it doesn't fit just for consistency's sake.
- Do NOT build a generic "any shape" skeleton-generation system — two concrete, purpose-built skeleton widgets (now-playing, nearby-list-or-reused-track-list) are enough; a configurable/generic skeleton builder is speculative infrastructure for two call sites.

## Technical Decisions
- **Reuse `SkeletonBox` as the low-level primitive** for any new skeleton widget (matching how `TrackListSkeleton` itself is built) rather than hand-rolling a new shimmer/placeholder mechanism — one shimmer/placeholder implementation for the whole app, multiple compositions of it per screen shape.

## Applicable Patterns
- `frontend-design-system.md` (new, created alongside this audit) — document the "skeleton for content-list loading states, not for form/action loading states" rule there so future screens follow it without needing to rediscover it.
- `frontend-layered-architecture.md` — new shared skeleton widgets belong in `core_design_system`, not in a feature's `presentation` package.

## Risks
- **Risk**: `player_screen.dart`'s loading state fires twice in quick succession in real usage (once for the initial screen mount, once for each track change's async state) — a skeleton that's too elaborate could itself feel like visual noise if it flashes briefly on every track skip. **Mitigation**: keep the now-playing skeleton simple (a handful of `SkeletonBox` rectangles, not an animated multi-element composition) and confirm during implementation whether the loading branch actually fires on every track change or only on initial session load (re-check `playerStateProvider`'s emission pattern before assuming it needs debouncing/minimum-display-time handling — don't add that complexity speculatively).
