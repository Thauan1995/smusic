/// Spacing scale shared by every `presentation/*` package. Values are a
/// simple 4px-based scale, kept intentionally small for Fatia 1 - extend
/// here, never inline magic numbers in feature widgets.
class SmusicSpacing {
  // Private no-op constructor that only exists to block instantiation of
  // this static-only token class; never called by design (see README
  // "Desvios da spec" for this exclusion pattern).
  const SmusicSpacing._(); // coverage:ignore-line

  static const double xs = 4;
  static const double sm = 8;
  static const double md = 16;
  static const double lg = 24;
  static const double xl = 32;
  static const double xxl = 48;
}
