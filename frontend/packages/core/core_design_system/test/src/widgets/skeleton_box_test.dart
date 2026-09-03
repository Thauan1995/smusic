import 'package:core_design_system/core_design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('renders with given width/height and animates', (tester) async {
    await tester.pumpWidget(
      const MaterialApp(
        home: Scaffold(body: SkeletonBox(width: 50, height: 20)),
      ),
    );

    expect(find.byType(SkeletonBox), findsOneWidget);
    final container = tester.widget<Container>(find.byType(Container));
    expect(
      container.constraints,
      const BoxConstraints.tightFor(width: 50, height: 20),
    );
    expect((container.decoration as BoxDecoration).gradient, isNotNull);

    // Pump a few animation frames to exercise the AnimatedBuilder rebuild.
    await tester.pump(const Duration(milliseconds: 300));
    await tester.pump(const Duration(milliseconds: 300));

    // Dispose cleanly (covers State.dispose()).
    await tester.pumpWidget(const SizedBox());
  });

  testWidgets('defaults to a 4px border radius', (tester) async {
    await tester.pumpWidget(
      const MaterialApp(home: Scaffold(body: SkeletonBox())),
    );
    final container = tester.widget<Container>(find.byType(Container));
    final decoration = container.decoration as BoxDecoration;
    expect(decoration.borderRadius, BorderRadius.circular(4));
  });
}
