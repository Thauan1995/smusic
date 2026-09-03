import 'package:core_design_system/core_design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('renders message and default icon, no action by default',
      (tester) async {
    await tester.pumpWidget(
      const MaterialApp(
        home: Scaffold(body: EmptyState(message: 'Nothing here yet')),
      ),
    );
    expect(find.text('Nothing here yet'), findsOneWidget);
    expect(find.byIcon(Icons.info_outline), findsOneWidget);
    expect(find.byType(TextButton), findsNothing);
  });

  testWidgets('renders action button and invokes onAction when both provided',
      (tester) async {
    var tapped = false;
    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: EmptyState(
            message: 'Search failed',
            icon: Icons.error_outline,
            actionLabel: 'Retry',
            onAction: () => tapped = true,
          ),
        ),
      ),
    );
    expect(find.byIcon(Icons.error_outline), findsOneWidget);
    expect(find.text('Retry'), findsOneWidget);
    await tester.tap(find.text('Retry'));
    expect(tapped, isTrue);
  });

  testWidgets('does not render action button when only actionLabel is set',
      (tester) async {
    await tester.pumpWidget(
      const MaterialApp(
        home: Scaffold(
          body: EmptyState(message: 'x', actionLabel: 'Retry'),
        ),
      ),
    );
    expect(find.byType(TextButton), findsNothing);
  });
}
