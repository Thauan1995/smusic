import 'package:core_design_system/core_design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('TrackRowSkeleton renders artwork + two text placeholders',
      (tester) async {
    await tester.pumpWidget(
      const MaterialApp(home: Scaffold(body: TrackRowSkeleton())),
    );
    expect(find.byType(SkeletonBox), findsNWidgets(3));
  });

  testWidgets('TrackListSkeleton renders itemCount rows', (tester) async {
    // Deliberately not const at the root: `const MaterialApp(...)` would
    // make the constant context propagate down and canonicalize
    // TrackListSkeleton's constructor call, which line coverage tooling
    // then never reports as "hit" even though it plainly executes.
    await tester.pumpWidget(
      // ignore: prefer_const_constructors
      MaterialApp(
        // ignore: prefer_const_constructors
        home: Scaffold(body: TrackListSkeleton(itemCount: 3)),
      ),
    );
    expect(find.byType(TrackRowSkeleton), findsNWidgets(3));
  });

  testWidgets('TrackListSkeleton defaults to 8 rows', (tester) async {
    await tester.pumpWidget(
      const MaterialApp(home: Scaffold(body: TrackListSkeleton())),
    );
    expect(find.byType(TrackRowSkeleton), findsNWidgets(8));
  });
}
