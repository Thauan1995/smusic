import 'package:social_proximity_domain/social_proximity_domain.dart';
import 'package:test/test.dart';

void main() {
  group('reveal level 0 defense in depth', () {
    test('nulls out displayName/avatarUrl even when the caller supplies them', () {
      final listener = NearbyListener(
        userId: 'u1',
        distanceBucket: DistanceBucket.veryClose,
        revealLevel: RevealLevel.level0,
        displayName: 'Should never appear',
        avatarUrl: 'https://example.com/should-never-appear.png',
      );

      expect(listener.displayName, isNull);
      expect(listener.avatarUrl, isNull);
    });

    test('level0 with no name/avatar supplied stays null (baseline case)', () {
      final listener = NearbyListener(
        userId: 'u1',
        distanceBucket: DistanceBucket.city,
        revealLevel: RevealLevel.level0,
      );
      expect(listener.displayName, isNull);
      expect(listener.avatarUrl, isNull);
    });
  });

  group('reveal level 1/2 carry identity', () {
    test('level1 keeps displayName/avatarUrl', () {
      final listener = NearbyListener(
        userId: 'u1',
        distanceBucket: DistanceBucket.neighborhood,
        revealLevel: RevealLevel.level1,
        displayName: 'Ana',
        avatarUrl: 'https://example.com/ana.png',
      );
      expect(listener.displayName, 'Ana');
      expect(listener.avatarUrl, 'https://example.com/ana.png');
    });

    test('level2 keeps displayName/avatarUrl', () {
      final listener = NearbyListener(
        userId: 'u2',
        distanceBucket: DistanceBucket.region,
        revealLevel: RevealLevel.level2,
        displayName: 'Bea',
      );
      expect(listener.displayName, 'Bea');
    });
  });

  test('carries nowPlaying and userId/bucket', () {
    const nowPlaying = NowPlayingSnapshot(trackTitle: 'Song');
    final listener = NearbyListener(
      userId: 'u3',
      distanceBucket: DistanceBucket.city,
      revealLevel: RevealLevel.level0,
      nowPlaying: nowPlaying,
    );
    expect(listener.userId, 'u3');
    expect(listener.distanceBucket, DistanceBucket.city);
    expect(listener.nowPlaying, nowPlaying);
  });

  test('equality/hashCode compare all fields', () {
    NearbyListener build({String userId = 'u1'}) => NearbyListener(
          userId: userId,
          distanceBucket: DistanceBucket.neighborhood,
          revealLevel: RevealLevel.level1,
          displayName: 'Ana',
          avatarUrl: 'a.png',
          nowPlaying: const NowPlayingSnapshot(trackTitle: 'Song'),
        );

    final a = build();
    final b = build();
    final c = build(userId: 'u2');

    expect(a, b);
    expect(a.hashCode, b.hashCode);
    expect(a, isNot(c));
  });
}
