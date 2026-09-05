import 'package:core_design_system/core_design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('NearbyListenerSkeleton renders avatar + text + badge placeholders', (tester) async {
    await tester.pumpWidget(
      const MaterialApp(home: Scaffold(body: NearbyListenerSkeleton())),
    );
    // Avatar + title + subtitle + distance badge = 4.
    expect(find.byType(SkeletonBox), findsNWidgets(4));
  });

  testWidgets('NearbyListSkeleton renders itemCount cards', (tester) async {
    // Deliberately not const at the root - see track_row_skeleton_test.dart's
    // identical comment on why a const context here would hide the
    // constructor call from line-coverage tooling.
    await tester.pumpWidget(
      // ignore: prefer_const_constructors
      MaterialApp(
        // ignore: prefer_const_constructors
        home: Scaffold(body: NearbyListSkeleton(itemCount: 3)),
      ),
    );
    expect(find.byType(NearbyListenerSkeleton), findsNWidgets(3));
  });

  testWidgets('NearbyListSkeleton defaults to 6 cards', (tester) async {
    await tester.pumpWidget(
      const MaterialApp(home: Scaffold(body: NearbyListSkeleton())),
    );
    expect(find.byType(NearbyListenerSkeleton), findsNWidgets(6));
  });
}
