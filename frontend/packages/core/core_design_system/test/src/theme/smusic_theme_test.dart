import 'package:core_design_system/core_design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('light() produces a light-brightness Material 3 theme', () {
    final theme = SmusicTheme.light();
    expect(theme.brightness, Brightness.light);
    expect(theme.useMaterial3, isTrue);
  });

  test('dark() produces a dark-brightness Material 3 theme', () {
    final theme = SmusicTheme.dark();
    expect(theme.brightness, Brightness.dark);
    expect(theme.useMaterial3, isTrue);
  });

  test('scaffoldBackgroundColor matches colorScheme.surface', () {
    final theme = SmusicTheme.light();
    expect(theme.scaffoldBackgroundColor, theme.colorScheme.surface);
  });
}
