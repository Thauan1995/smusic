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
**Color**: one seed color (`SmusicColors.brandSeed`) feeds Material 3's `ColorScheme.fromSeed(...)` in `SmusicTheme._build`, producing both `light()` and `dark()` `ThemeData`. Screens never construct their own colors — confirmed by a repo-wide audit (2026-09-04): zero hardcoded `Color(0x...)` or `Colors.*` usages anywhere in `presentation/*` outside the design system's own token files.

```dart
// smusic_theme.dart
static ThemeData _build(Brightness brightness) {
  final colorScheme = ColorScheme.fromSeed(
    seedColor: SmusicColors.brandSeed,
    brightness: brightness,
    error: SmusicColors.error,
  );
  return ThemeData(useMaterial3: true, brightness: brightness, colorScheme: colorScheme, ...);
}
```

**Spacing**: a fixed 6-step scale (`SmusicSpacing.xs/sm/md/lg/xl/xxl`, 4px-based). Audited 2026-09-04: 15/15 `EdgeInsets.*` calls and 31/32 `SizedBox(height:/width:)` calls in `presentation/*` use `SmusicSpacing.*` tokens (the one exception is a 14x14 inline spinner size, not a spacing gap — a defensible exception, not a violation).

**Empty/error states**: `EmptyState` (message + icon + optional action) is the shared widget for "no data"/"error" — used correctly in 6 of 9 sampled screens (`library_screen`, `search_screen`, `player_screen`, and 3 `social_proximity_ui` screens).

**Loading states for content lists**: `TrackListSkeleton` (built from `SkeletonBox` primitives) renders row-shaped placeholders during an `AsyncValue.loading()` — used correctly in `library_screen.dart`/`search_screen.dart`. **Not yet extended to `player_screen.dart` or `proximity_list_screen.dart`**, which still use a bare `CircularProgressIndicator` for their loading branch — see `.vibeflow/specs/skeleton-loading-player-and-proximity.md`.

**Icons**: Flutter's built-in Material Icons only (no custom icon font/package in any `pubspec.yaml`) — one family, consistent weight by default. The filled/outlined *pairing* (e.g. `Icons.play_circle_filled` vs `_outline`) is used inconsistently today — see `.vibeflow/specs/icon-system-consistency.md`.

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
- `frontend/packages/presentation/player_ui/lib/src/screens/player_screen.dart:29,74` and `social_proximity_ui/lib/src/screens/proximity_list_screen.dart`'s loading branches use `CircularProgressIndicator` for what is/leads-to list-shaped content — breaks the "shape-matched skeleton for content lists" rule above. Being fixed via `.vibeflow/specs/skeleton-loading-player-and-proximity.md`.
- `SmusicColors.brandSeed` (`0xFF1ED760`) is, concretely, Spotify's own brand green — a placeholder the file's own doc comment flags as "not a final visual identity decision." Being replaced via `.vibeflow/specs/brand-color-system-red-black-white.md`.
- Inconsistent filled/outlined icon pairing (`Icons.play_circle_filled` vs `_outline`, `Icons.pause_circle_filled` vs `_outline`, `Icons.person` vs `_outline`) with no documented state-based rule behind the choice. Being fixed via `.vibeflow/specs/icon-system-consistency.md`.
