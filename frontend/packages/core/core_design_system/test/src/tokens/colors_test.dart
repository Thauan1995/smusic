import 'dart:math' as math;

import 'package:core_design_system/core_design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

/// WCAG 2.x relative luminance (https://www.w3.org/TR/WCAG21/#dfn-relative-luminance).
double _relativeLuminance(Color c) {
  double linearize(double channel) {
    return channel <= 0.03928
        ? channel / 12.92
        : math.pow((channel + 0.055) / 1.055, 2.4).toDouble();
  }

  final r = linearize(c.r);
  final g = linearize(c.g);
  final b = linearize(c.b);
  return 0.2126 * r + 0.7152 * g + 0.0722 * b;
}

/// WCAG contrast ratio (https://www.w3.org/TR/WCAG21/#dfn-contrast-ratio).
double _contrastRatio(Color a, Color b) {
  final la = _relativeLuminance(a);
  final lb = _relativeLuminance(b);
  final lighter = math.max(la, lb);
  final darker = math.min(la, lb);
  return (lighter + 0.05) / (darker + 0.05);
}

void main() {
  // .vibeflow/specs/brand-color-system-red-black-white.md's DoD: "A
  // contrast check is actually run (not eyeballed) ... both must clear
  // WCAG AA (4.5:1 for normal text, 3:1 for large text/UI components)."
  // These are real computed ratios, not restated constants - if either
  // color changes, this test recomputes and fails if the pair regresses
  // below AA, rather than silently trusting a stale comment.
  const wcagAANormalText = 4.5;

  test('white text on brandRed clears WCAG AA for normal text', () {
    final ratio = _contrastRatio(SmusicColors.pureWhite, SmusicColors.brandRed);
    expect(
      ratio,
      greaterThanOrEqualTo(wcagAANormalText),
      reason: 'computed ratio was $ratio — see colors.dart doc comment for the design intent',
    );
  });

  test('error red on surfaceBlack clears WCAG AA for normal text', () {
    final ratio = _contrastRatio(SmusicColors.error, SmusicColors.surfaceBlack);
    expect(
      ratio,
      greaterThanOrEqualTo(wcagAANormalText),
      reason: 'computed ratio was $ratio',
    );
  });

  test('brandRed and error are visually distinguishable, not the same red', () {
    // Not a WCAG check - this guards Context point 2 (brand accent and
    // error must never be confusable). A same-color regression would
    // make this ratio 1.0 (identical luminance); anything meaningfully
    // apart confirms they're not the same swatch. Combined with the
    // documented ~15° hue gap (see colors.dart), this is enough to catch
    // an accidental "reuse error as brand" edit.
    expect(SmusicColors.brandRed, isNot(equals(SmusicColors.error)));
    final ratio = _contrastRatio(SmusicColors.brandRed, SmusicColors.error);
    expect(ratio, greaterThan(1.05), reason: 'brandRed and error are too close in luminance to read as distinct');
  });
}
