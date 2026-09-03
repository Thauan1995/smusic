import 'package:core_design_system/core_design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('shows label and invokes onPressed when tapped', (tester) async {
    var tapped = false;
    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: SmusicPrimaryButton(
            label: 'Sign in',
            onPressed: () => tapped = true,
          ),
        ),
      ),
    );

    expect(find.text('Sign in'), findsOneWidget);
    await tester.tap(find.byType(SmusicPrimaryButton));
    expect(tapped, isTrue);
  });

  testWidgets('shows a spinner and disables tap while isLoading', (tester) async {
    var tapped = false;
    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: SmusicPrimaryButton(
            label: 'Sign in',
            isLoading: true,
            onPressed: () => tapped = true,
          ),
        ),
      ),
    );

    expect(find.byType(CircularProgressIndicator), findsOneWidget);
    expect(find.text('Sign in'), findsNothing);

    final button = tester.widget<FilledButton>(find.byType(FilledButton));
    expect(button.onPressed, isNull);
    expect(tapped, isFalse);
  });
}
