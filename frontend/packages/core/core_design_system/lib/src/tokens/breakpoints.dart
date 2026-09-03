/// Width-based layout classes. Per frontend-flutter.md section 1.3:
/// "Responsividade ... adapta por breakpoint de largura de tela ... não por
/// Platform.isX". A mobile web browser in a narrow window and a phone app
/// both get [compact]; a desktop browser and a large tablet both get
/// [expanded].
enum WindowSizeClass { compact, medium, expanded }

class Breakpoints {
  // Private no-op constructor, never called by design (see spacing.dart
  // for the pattern rationale).
  const Breakpoints._(); // coverage:ignore-line

  /// Below this width: single-column layouts, bottom navigation bar.
  static const double compactMax = 600;

  /// Below this width (and >= [compactMax]): condensed multi-pane layouts,
  /// navigation rail without labels.
  static const double mediumMax = 1024;

  /// >= [mediumMax]: wide layouts, navigation rail/sidebar with labels (see
  /// frontend-flutter.md section 3.5).
  static WindowSizeClass classify(double width) {
    if (width < compactMax) return WindowSizeClass.compact;
    if (width < mediumMax) return WindowSizeClass.medium;
    return WindowSizeClass.expanded;
  }
}
