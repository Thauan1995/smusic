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
    final isDark = brightness == Brightness.dark;
    final primary = isDark ? SmusicColors.primaryElevatedDark : SmusicColors.black;
    final surface = isDark ? SmusicColors.surfaceBlack : SmusicColors.pureWhite;
    final onSurface = isDark ? SmusicColors.pureWhite : SmusicColors.black;

    // ColorScheme.fromSeed (seeded from black — see colors.dart's class
    // doc for why pure black/white still isn't fully hue-neutral in
    // Material 3's HCT algorithm) fills every role this app does NOT
    // explicitly override below with an algorithmically-derived, mostly-
    // neutral tone (outline, shadow, scrim, tertiary, the less-visible
    // surfaceContainer* steps) — good enough for roles this app's
    // screens don't put large areas of on screen. Every role real
    // screens actually render as a large fill (primary/secondary
    // buttons, their containers - e.g. the FAB, NavigationBar's
    // selected-destination pill - and surface/surfaceContainerHighest)
    // is set explicitly here, to colors.dart's exact values, not to
    // fromSeed's approximation of them. This fixes two real bugs found
    // by visually loading the live app: (1) primary/secondary alone
    // being overridden via .copyWith left *Container roles un-set,
    // rendering the FAB and nav indicator in fromSeed's default purple;
    // (2) the whole app's neutral surfaces reading faintly pink even
    // with an all-neutral base ColorScheme.light()/dark(), traced to
    // widgets that read colorScheme.surfaceContainerHighest specifically
    // (SkeletonBox) rather than a plain neutral gray.
    return ThemeData(
      useMaterial3: true,
      brightness: brightness,
      colorScheme: ColorScheme.fromSeed(
        seedColor: SmusicColors.black,
        brightness: brightness,
        primary: primary,
        onPrimary: SmusicColors.pureWhite,
        primaryContainer: primary,
        onPrimaryContainer: SmusicColors.pureWhite,
        secondary: SmusicColors.brandRed,
        onSecondary: SmusicColors.pureWhite,
        secondaryContainer: SmusicColors.brandRed,
        onSecondaryContainer: SmusicColors.pureWhite,
        surface: surface,
        onSurface: onSurface,
        surfaceContainerHighest:
            isDark ? SmusicColors.neutralSurfaceContainerDark : SmusicColors.neutralSurfaceContainerLight,
        error: SmusicColors.error,
        onError: SmusicColors.pureWhite,
      ),
      scaffoldBackgroundColor: surface,
    );
  }
}
