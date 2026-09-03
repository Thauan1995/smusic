import 'package:core_design_system/core_design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:social_proximity_domain/social_proximity_domain.dart';
import 'package:social_proximity_ui/social_proximity_ui.dart';

Widget _wrap(Widget child) {
  return MaterialApp(theme: SmusicTheme.light(), home: Scaffold(body: child));
}

void main() {
  group('defense in depth (task requirement): reveal level 0 never renders identity', () {
    testWidgets('level 0 shows the anonymous copy, never a name/avatar/number', (tester) async {
      final listener = NearbyListener(
        userId: 'u1',
        distanceBucket: DistanceBucket.veryClose,
        revealLevel: RevealLevel.level0,
        // A backend bug could theoretically still put a name/avatar on the
        // wire for a level-0 entry - NearbyListener's own constructor
        // already nulls these out (see that class's doc comment), so this
        // test constructs via the same public API the DTO layer uses,
        // proving the *widget* never shows one regardless.
        displayName: 'Leaked Name',
        avatarUrl: 'https://x/leak.png',
      );

      await tester.pumpWidget(_wrap(NearbyListenerCard(listener: listener)));

      expect(find.text('Alguém por perto'), findsOneWidget);
      expect(find.text('Leaked Name'), findsNothing);
      expect(find.byKey(const Key('nearby_listener_anonymous_avatar')), findsOneWidget);
      expect(find.byKey(const Key('nearby_listener_avatar')), findsNothing);
      // Distance is only ever the bucket label, never a raw number.
      expect(find.text('Bem pertinho'), findsOneWidget);
      expect(find.textContaining(RegExp(r'\d')), findsNothing);
    });

    testWidgets('level 0 with nothing playing shows the track-less copy', (tester) async {
      final listener = NearbyListener(
        userId: 'u1',
        distanceBucket: DistanceBucket.city,
        revealLevel: RevealLevel.level0,
      );

      await tester.pumpWidget(_wrap(NearbyListenerCard(listener: listener)));

      expect(find.text('Não está ouvindo nada no momento'), findsOneWidget);
    });
  });

  group('reveal level 1/2 render identity', () {
    testWidgets('level 1 shows display name and now playing', (tester) async {
      final listener = NearbyListener(
        userId: 'u2',
        distanceBucket: DistanceBucket.neighborhood,
        revealLevel: RevealLevel.level1,
        displayName: 'Ana',
        avatarUrl: 'https://x/a.png',
        nowPlaying: const NowPlayingSnapshot(trackTitle: 'Song', artistName: 'Artist'),
      );

      await tester.pumpWidget(_wrap(NearbyListenerCard(listener: listener)));
      // The CircleAvatar renders synchronously with `NetworkImage(avatarUrl)`
      // regardless of whether the image itself ever successfully decodes -
      // `flutter_test` has no real network, so the image load fails
      // asynchronously after the widget is already built; consume that
      // expected, unrelated error rather than mocking network I/O for what
      // is otherwise a pure widget-tree assertion.
      await tester.pump();
      expect(tester.takeException(), isNotNull);

      expect(find.text('Ana'), findsOneWidget);
      expect(find.text('Ouvindo Song'), findsOneWidget);
      expect(find.text('No seu bairro'), findsOneWidget);
      expect(find.byKey(const Key('nearby_listener_avatar')), findsOneWidget);
    });

    testWidgets('level 2 with no display name/avatar falls back to a placeholder', (tester) async {
      final listener = NearbyListener(
        userId: 'u4',
        distanceBucket: DistanceBucket.city,
        revealLevel: RevealLevel.level2,
      );

      await tester.pumpWidget(_wrap(NearbyListenerCard(listener: listener)));

      expect(find.text('Alguém'), findsOneWidget);
      expect(find.byKey(const Key('nearby_listener_placeholder_avatar')), findsOneWidget);
    });

    testWidgets('reveals identity but avatar url is present', (tester) async {
      final listener = NearbyListener(
        userId: 'u3',
        distanceBucket: DistanceBucket.region,
        revealLevel: RevealLevel.level2,
        displayName: 'Cid',
        avatarUrl: 'https://x/c.png',
      );
      await tester.pumpWidget(_wrap(NearbyListenerCard(listener: listener)));
      await tester.pump();
      expect(tester.takeException(), isNotNull);

      expect(find.text('Cid'), findsOneWidget);
      expect(find.text('Na sua região'), findsOneWidget);
      expect(find.byKey(const Key('nearby_listener_avatar')), findsOneWidget);
    });
  });
}
