import 'package:core_design_system/core_design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

/// One golden test for the design system's primary button, per
/// docs/architecture/frontend-flutter.md section 5.2 ("Golden tests ... para
/// core_design_system (todo componente visual compartilhado)"). Uses
/// Flutter's built-in `matchesGoldenFile` rather than `golden_toolkit`/
/// `alchemist` (documented deviation - see frontend/README.md) to avoid an
/// extra dependency for a single-component regression check in Fatia 1.
void main() {
  testWidgets('SmusicPrimaryButton matches golden (light theme, enabled)',
      (tester) async {
    await tester.pumpWidget(
      MaterialApp(
        theme: SmusicTheme.light(),
        home: const Scaffold(
          body: Center(
            child: SmusicPrimaryButton(label: 'Play', onPressed: null),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    await expectLater(
      find.byType(SmusicPrimaryButton),
      matchesGoldenFile('smusic_primary_button_light.png'),
    );
  });

  testWidgets('SmusicPrimaryButton matches golden (dark theme, loading)',
      (tester) async {
    await tester.pumpWidget(
      MaterialApp(
        theme: SmusicTheme.dark(),
        home: const Scaffold(
          body: Center(
            child: SmusicPrimaryButton(
              label: 'Play',
              isLoading: true,
              onPressed: null,
            ),
          ),
        ),
      ),
    );
    // Not pumpAndSettle(): the loading spinner animates indefinitely, so
    // settling never happens. A single fixed pump is enough for a stable
    // golden frame.
    await tester.pump(const Duration(milliseconds: 200));

    await expectLater(
      find.byType(SmusicPrimaryButton),
      matchesGoldenFile('smusic_primary_button_dark_loading.png'),
    );
  });
}
