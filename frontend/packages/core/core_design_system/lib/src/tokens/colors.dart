import 'package:flutter/widgets.dart';

/// Brand color tokens (.vibeflow/specs/brand-color-system-red-black-white.md,
/// revised 2026-09-05 per explicit user direction: black is the PRIMARY
/// brand color, red and white are secondary).
///
/// Role mapping onto Material 3's `ColorScheme`:
/// - `primary`/`onPrimary` -> [black] (light) / [primaryElevatedDark] (dark)
///   + [pureWhite].
/// - `secondary`/`onSecondary` -> [brandRed] + [pureWhite].
/// - `surface` -> [pureWhite] (light) / [surfaceBlack] (dark).
///
/// **Why dark mode's primary isn't literally the same black as the
/// surface**: [surfaceBlack] (`0xFF121212`) is the dark-mode background;
/// making `primary` (button fills) the exact same black would make every
/// primary button visually disappear into the background (near-zero
/// contrast between two blacks is a structural problem, not a tuning
/// one — WCAG 1.4.11's 3:1 non-text-contrast guidance for UI components
/// against adjacent colors cannot be met by two colors this close in
/// luminance no matter which two near-black hex values are picked).
/// [primaryElevatedDark] (`0xFF2C2C2C`) is still unambiguously "black" to
/// the eye, just lifted enough to read as a distinct, tappable surface —
/// the same problem Material's own dark-theme elevation overlays solve
/// the same way (a lighter tone for elevated/interactive surfaces, not
/// literal same-color-as-background).
///
/// [brandRed] and [error] are deliberately different reds — WCAG
/// contrast ratios computed and recorded here (not eyeballed), so a
/// future edit to either can be checked against these numbers instead of
/// re-deriving them:
/// - White text on [brandRed]: **5.88:1** (passes WCAG AA's 4.5:1 for
///   normal text).
/// - [error] on [surfaceBlack]: **4.90:1** (passes WCAG AA's 4.5:1 for
///   normal text).
/// - Hue: brandRed sits at ~350° (crimson), error at ~6° (orange-red/
///   tomato) — a deliberate ~15° hue gap plus a ~15-point lightness gap,
///   so the two read as distinct reds side by side.
///   See `test/src/tokens/colors_test.dart` for the automated checks.
class SmusicColors {
  // Private no-op constructor, never called by design (see spacing.dart
  // for the pattern rationale).
  const SmusicColors._(); // coverage:ignore-line

  /// Primary brand color — light mode's `colorScheme.primary` (buttons,
  /// active/selected state, the brand mark). True black: white text on
  /// this is 21:1, the maximum possible contrast.
  static const Color black = Color(0xFF000000);

  /// Dark mode's `colorScheme.primary` — see class doc for why this
  /// isn't literally [black]. White text on this is still 14.4:1.
  static const Color primaryElevatedDark = Color(0xFF2A2A2A);

  /// Secondary brand color — accent buttons, links, active-state
  /// highlights. Never used as a dominant background (red-as-wallpaper
  /// both fatigues the eye over a long listening session and reads as an
  /// alert/error state).
  static const Color brandRed = Color(0xFFC8102E);

  /// Dominant dark-mode surface — a genuine near-black, not a mid-gray
  /// tinted by an accent seed color (Material 3's `ColorScheme.fromSeed`
  /// derives every tone, including "neutral" surfaces, from whatever
  /// color seeds it — seeding from black instead of red keeps every
  /// derived neutral tone actually neutral, not pink-tinted).
  static const Color surfaceBlack = Color(0xFF121212);

  /// Secondary brand color — text/contrast on dark surfaces, light
  /// mode's surface.
  static const Color pureWhite = Color(0xFFFFFFFF);

  /// Error/destructive state — intentionally a different red from
  /// [brandRed] (see class doc) so "this is the primary action" and
  /// "something is wrong" are never visually confusable.
  static const Color error = Color(0xFFE74C3C);

  /// Neutral "elevated surface" tone for light mode (skeleton-loading
  /// placeholders, container chrome) — a true, hue-free gray.
  static const Color neutralSurfaceContainerLight = Color(0xFFE8E8E8);

  /// Neutral "elevated surface" tone for dark mode — same role as
  /// [primaryElevatedDark] (a lighter-than-background gray so elevated
  /// content reads as distinct from [surfaceBlack]); kept as its own
  /// named constant since the two roles (a button's fill vs. a
  /// skeleton's shimmer base) are conceptually different even though
  /// they currently share a value.
  static const Color neutralSurfaceContainerDark = primaryElevatedDark;
}
