import 'package:flutter/material.dart';

import '../tokens/colors.dart';

/// Single source of truth for the app's `ThemeData`, shared verbatim by
/// `smusic_mobile` and `smusic_web` (frontend-flutter.md section 1.3 -
/// `SmusicApp` has zero source difference between the two entrypoints).
class SmusicTheme {
  // Private no-op constructor, never called by design (see spacing.dart
  // for the pattern rationale).
  const SmusicTheme._(); // coverage:ignore-line

  static ThemeData light() => _build(Brightness.light);

  static ThemeData dark() => _build(Brightness.dark);

  static ThemeData _build(Brightness brightness) {
    final seeded = ColorScheme.fromSeed(
      seedColor: SmusicColors.brandRed,
      brightness: brightness,
      error: SmusicColors.error,
    );
    // Dark mode: force a genuine near-black surface + white-on-surface
    // text, rather than trusting fromSeed's derived (lighter, red-tinted)
    // dark surface — see colors.dart's doc comment and
    // .vibeflow/specs/brand-color-system-red-black-white.md's DoD ("dark
    // mode must look authentically dark/black, not dark gray that
    // happens to have a red tint"). Light mode keeps fromSeed's derived
    // (near-white) surface unchanged — still coherent, and this spec
    // doesn't require light mode to be pure white.
    final colorScheme = brightness == Brightness.dark
        ? seeded.copyWith(
            surface: SmusicColors.surfaceBlack,
            onSurface: SmusicColors.pureWhite,
          )
        : seeded;
    return ThemeData(
      useMaterial3: true,
      brightness: brightness,
      colorScheme: colorScheme,
      scaffoldBackgroundColor: colorScheme.surface,
    );
  }
}
