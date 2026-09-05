import 'package:core_design_system/core_design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('NowPlayingSkeleton renders artwork + text + transport placeholders', (tester) async {
    await tester.pumpWidget(
      const MaterialApp(home: Scaffold(body: NowPlayingSkeleton())),
    );
    // Album art + title + artist + seek bar + 3 transport buttons = 7.
    expect(find.byType(SkeletonBox), findsNWidgets(7));
  });
}
