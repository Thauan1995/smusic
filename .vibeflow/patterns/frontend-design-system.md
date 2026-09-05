---
tags: [flutter, dart, design-system, theming, color, icons, spacing, skeleton, ui]
modules: [frontend/packages/core/core_design_system/, frontend/packages/presentation/]
applies_to: [widgets, screens, themes]
confidence: inferred
---
# Pattern: Design System (tokens, theme, shared widgets)

<!-- vibeflow:auto:start -->
## What
`core_design_system` is the single source of truth for color, spacing, typography (via `ThemeData`), and a handful of shared widgets (`EmptyState`, `SkeletonBox`/`TrackListSkeleton`, `SmusicPrimaryButton`). Every `presentation/*` package consumes it via `Theme.of(context)` and the token classes — never via hardcoded `Color(0x...)`/`Colors.*` or raw numeric spacing.

## Where
- Tokens: `frontend/packages/core/core_design_system/lib/src/tokens/{colors,spacing,breakpoints}.dart`.
- Theme: `frontend/packages/core/core_design_system/lib/src/theme/smusic_theme.dart`.
- Shared widgets: `frontend/packages/core/core_design_system/lib/src/widgets/{empty_state,skeleton_box,track_row_skeleton,smusic_primary_button}.dart`.
- Consumed by every screen in `auth_ui`, `library_ui`, `player_ui`, `social_proximity_ui`, `shared_navigation`.

## The Pattern
**Color**: one seed color (`SmusicColors.brandRed`) feeds Material 3's `ColorScheme.fromSeed(...)` in `SmusicTheme._build`, producing both `light()` and `dark()` `ThemeData`; dark mode additionally forces a genuine near-black `surface`/`onSurface` pair (`SmusicColors.surfaceBlack`/`pureWhite`) rather than trusting `fromSeed`'s derived dark surface — see `.vibeflow/specs/brand-color-system-red-black-white.md`. Screens never construct their own colors — confirmed by a repo-wide audit (2026-09-04): zero hardcoded `Color(0x...)` or `Colors.*` usages anywhere in `presentation/*` outside the design system's own token files.

```dart
// smusic_theme.dart
static ThemeData _build(Brightness brightness) {
  final seeded = ColorScheme.fromSeed(
    seedColor: SmusicColors.brandRed,
    brightness: brightness,
    error: SmusicColors.error,
  );
  final colorScheme = brightness == Brightness.dark
      ? seeded.copyWith(surface: SmusicColors.surfaceBlack, onSurface: SmusicColors.pureWhite)
      : seeded;
  return ThemeData(useMaterial3: true, brightness: brightness, colorScheme: colorScheme, ...);
}
```

Brand accent (`brandRed`, `0xFFC8102E`) and the pre-existing `error` red (`0xFFE74C3C`) are deliberately different reds (~15° hue apart, ~15pt lightness apart) — computed WCAG contrast ratios are recorded in `colors.dart`'s doc comment and enforced by `core_design_system/test/src/tokens/colors_test.dart`, not just eyeballed.

**Spacing**: a fixed 6-step scale (`SmusicSpacing.xs/sm/md/lg/xl/xxl`, 4px-based). Audited 2026-09-04: 15/15 `EdgeInsets.*` calls and 31/32 `SizedBox(height:/width:)` calls in `presentation/*` use `SmusicSpacing.*` tokens (the one exception is a 14x14 inline spinner size, not a spacing gap — a defensible exception, not a violation).

**Empty/error states**: `EmptyState` (message + icon + optional action) is the shared widget for "no data"/"error" — used correctly in 6 of 9 sampled screens (`library_screen`, `search_screen`, `player_screen`, and 3 `social_proximity_ui` screens).

**Loading states for content lists**: shape-matched skeletons (`SkeletonBox` compositions) for every content screen's `AsyncValue.loading()` branch — `TrackListSkeleton` (`library_screen.dart`/`search_screen.dart`), `NowPlayingSkeleton` (`player_screen.dart` — album art + title/artist + seek bar + transport row placeholders, since a single now-playing view isn't a list of rows), and `NearbyListSkeleton`/`NearbyListenerSkeleton` (`proximity_list_screen.dart`'s feed-loading branch — circular avatar + text + trailing distance-badge placeholder, a meaningfully different shape from a track row). `proximity_list_screen.dart`'s *earlier* settings-fetch gate deliberately keeps a plain spinner — it's a "which screen do we even show" decision, not list content (see this doc's Rules section and the spec's anti-scope on form/decision screens).

**Icons**: Flutter's built-in Material Icons only (no custom icon font/package in any `pubspec.yaml`) — one family, consistent weight by default. The filled/outlined *pairing* (e.g. `Icons.play_circle_filled` vs `_outline`) now follows the Rules section's stated rule consistently — `player_screen`'s and `PauseDiscoveryToggle`'s play/pause controls, and `NavigationShell`'s bottom-bar/rail destinations (which previously had no selected-vs-unselected icon distinction at all — a single `icon` reused for both `NavigationDestination.icon` and `.selectedIcon`) — see `.vibeflow/specs/icon-system-consistency.md`. `nearby_listener_card.dart`'s anonymous-vs-placeholder avatar icons (`person_outline`/`person`) were audited too and already matched the rule (anonymous/minimal = outlined, identified/fuller = filled) — left unchanged.

## Rules
- Never hardcode a color or numeric spacing value in a `presentation/*` screen/widget — add a token to `core_design_system` if one doesn't exist yet, don't inline it.
- A new content-list loading state uses `TrackListSkeleton` (or a purpose-built skeleton widget composed from `SkeletonBox`) — never a bare `CircularProgressIndicator` for list-shaped content (a spinner is fine for a short, non-list action like a login submit).
- Icon filled/outlined variant follows: filled = active/selected/in-progress, outlined = inactive/available/unselected (Material Design's own convention for this pairing, matching Spotify/YouTube Music's own nav/control iconography).

## Examples from this codebase
File: `frontend/packages/presentation/library_ui/lib/src/screens/search_screen.dart:46`
```dart
loading: () => const TrackListSkeleton(),
```

File: `frontend/packages/presentation/social_proximity_ui/lib/src/widgets/nearby_listener_card.dart:34,36`
```dart
margin: const EdgeInsets.symmetric(horizontal: SmusicSpacing.md, vertical: SmusicSpacing.xs),
...
padding: const EdgeInsets.all(SmusicSpacing.md),
```
<!-- vibeflow:auto:end -->

## Anti-patterns
- ~~`player_screen.dart`/`proximity_list_screen.dart`'s loading branches used `CircularProgressIndicator` for list-shaped content~~ **Resolved 2026-09-04** — `NowPlayingSkeleton`/`NearbyListSkeleton` added to `core_design_system`; see `.vibeflow/specs/skeleton-loading-player-and-proximity.md`.
- ~~`SmusicColors.brandSeed` (`0xFF1ED760`) is, concretely, Spotify's own brand green~~ **Resolved 2026-09-04** — replaced by `brandRed`/`surfaceBlack`/`pureWhite` per `.vibeflow/specs/brand-color-system-red-black-white.md`.
- ~~Inconsistent filled/outlined icon pairing with no documented state-based rule~~ **Resolved 2026-09-04** — `player_screen`/`PauseDiscoveryToggle`/`NavigationShell` all now follow the Rules section's rule; see `.vibeflow/specs/icon-system-consistency.md`.
