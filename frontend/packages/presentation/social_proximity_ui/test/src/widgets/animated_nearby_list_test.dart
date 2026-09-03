import 'package:core_design_system/core_design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:social_proximity_domain/social_proximity_domain.dart';
import 'package:social_proximity_ui/social_proximity_ui.dart';

NearbyListener _listener(String id, {DistanceBucket bucket = DistanceBucket.veryClose}) {
  return NearbyListener(userId: id, distanceBucket: bucket, revealLevel: RevealLevel.level0);
}

Widget _wrap(List<NearbyListener> listeners) {
  return MaterialApp(
    theme: SmusicTheme.light(),
    home: Scaffold(body: AnimatedNearbyList(listeners: listeners)),
  );
}

void main() {
  testWidgets('renders the initial snapshot without animating', (tester) async {
    await tester.pumpWidget(_wrap([_listener('a'), _listener('b')]));
    await tester.pump();

    expect(find.byType(NearbyListenerCard), findsNWidgets(2));
  });

  testWidgets('inserting a new listener animates it in', (tester) async {
    await tester.pumpWidget(_wrap([_listener('a')]));
    await tester.pump();
    expect(find.byType(NearbyListenerCard), findsOneWidget);

    await tester.pumpWidget(_wrap([_listener('a'), _listener('b')]));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 260));

    expect(find.byType(NearbyListenerCard), findsNWidgets(2));
  });

  testWidgets('removing a listener animates it out and settles empty', (tester) async {
    await tester.pumpWidget(_wrap([_listener('a'), _listener('b')]));
    await tester.pump();

    await tester.pumpWidget(_wrap([_listener('a')]));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 260));

    expect(find.byType(NearbyListenerCard), findsOneWidget);

    await tester.pumpWidget(_wrap(const []));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 260));

    expect(find.byType(NearbyListenerCard), findsNothing);
  });

  testWidgets('reordering an unchanged set swaps in place without extra items', (tester) async {
    await tester.pumpWidget(_wrap([_listener('a'), _listener('b')]));
    await tester.pump();

    await tester.pumpWidget(_wrap([_listener('b'), _listener('a')]));
    await tester.pump();

    expect(find.byType(NearbyListenerCard), findsNWidgets(2));
  });

  testWidgets('an update to an existing listener id refreshes it in place', (tester) async {
    await tester.pumpWidget(
      _wrap([_listener('a', bucket: DistanceBucket.veryClose)]),
    );
    await tester.pump();
    expect(find.text('Bem pertinho'), findsOneWidget);

    await tester.pumpWidget(
      _wrap([_listener('a', bucket: DistanceBucket.city)]),
    );
    await tester.pump();

    expect(find.text('Na sua cidade'), findsOneWidget);
    expect(find.byType(NearbyListenerCard), findsOneWidget);
  });
}
