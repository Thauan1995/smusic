import 'package:flutter/widgets.dart';

/// Brand color tokens (.vibeflow/specs/brand-color-system-red-black-white.md).
///
/// Replaces the earlier placeholder seed (`0xFF1ED760` — literally
/// Spotify's own brand green, per that spec's Context section) with a
/// deliberate red/black/white identity: black as the dominant surface,
/// red as a disciplined accent (never a dominant background — red-as-
/// wallpaper both fatigues the eye over a long listening session and
/// reads as an alert/error state, exactly the wrong vocabulary for a
/// player app's default UI), white for contrast.
///
/// [brandRed] and [error] are deliberately different reds — WCAG
/// contrast ratios computed and recorded here (not eyeballed), so a
/// future edit to either can be checked against these numbers instead of
/// re-deriving them:
/// - White text on [brandRed]: **5.88:1** (passes WCAG AA's 4.5:1 for
///   normal text — the primary button/CTA context).
/// - [error] on [surfaceBlack]: **4.90:1** (passes WCAG AA's 4.5:1 for
///   normal text — error messaging on the dark surface).
/// - Hue: brandRed sits at ~350° (crimson), error at ~6° (orange-red/
///   tomato) — a deliberate ~15° hue gap plus a ~15-point lightness gap
///   (brandRed is the deeper, richer tone), so the two read as distinct
///   reds side by side, never as "the same red at different opacity."
///   See `test/src/tokens/colors_test.dart` for the automated check.
class SmusicColors {
  // Private no-op constructor, never called by design (see spacing.dart
  // for the pattern rationale).
  const SmusicColors._(); // coverage:ignore-line

  /// Brand accent — buttons, active/selected state, the brand mark. Never
  /// used as a dominant background (see class doc).
  static const Color brandRed = Color(0xFFC8102E);

  /// Dominant dark-mode surface — a genuine near-black, not a mid-gray
  /// tinted by the accent seed (Material 3's `ColorScheme.fromSeed` would
  /// otherwise derive a lighter, red-tinted "dark" surface).
  static const Color surfaceBlack = Color(0xFF121212);

  /// Text/contrast on dark surfaces.
  static const Color pureWhite = Color(0xFFFFFFFF);

  /// Error/destructive state — intentionally a different red from
  /// [brandRed] (see class doc) so "this is the primary action" and
  /// "something is wrong" are never visually confusable.
  static const Color error = Color(0xFFE74C3C);
}
