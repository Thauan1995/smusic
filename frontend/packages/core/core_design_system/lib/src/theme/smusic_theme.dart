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
    final colorScheme = ColorScheme.fromSeed(
      seedColor: SmusicColors.brandSeed,
      brightness: brightness,
      error: SmusicColors.error,
    );
    return ThemeData(
      useMaterial3: true,
      brightness: brightness,
      colorScheme: colorScheme,
      scaffoldBackgroundColor: colorScheme.surface,
    );
  }
}
