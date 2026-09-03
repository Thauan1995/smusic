import 'package:flutter_test/flutter_test.dart';
import 'package:social_proximity_data/social_proximity_data.dart';
import 'package:social_proximity_domain/social_proximity_domain.dart';

void main() {
  group('listenersFromUsersJson', () {
    test('parses a full user entry with explicit reveal_level', () {
      final listeners = ProximityDtos.listenersFromUsersJson([
        {
          'user_id': 'u1',
          'display_name': 'Ana',
          'avatar_url': 'https://x/a.png',
          'distance_bucket': '150m_1km',
          'reveal_level': 1,
          'now_playing': {'title': 'Song', 'artist_name': 'Artist'},
        },
      ]);

      expect(listeners, hasLength(1));
      final listener = listeners.single;
      expect(listener.userId, 'u1');
      expect(listener.displayName, 'Ana');
      expect(listener.avatarUrl, 'https://x/a.png');
      expect(listener.distanceBucket, DistanceBucket.neighborhood);
      expect(listener.revealLevel, RevealLevel.level1);
      expect(listener.nowPlaying?.trackTitle, 'Song');
      expect(listener.nowPlaying?.artistName, 'Artist');
    });

    test('infers level0 when reveal_level is absent and no identity fields are present', () {
      final listener = ProximityDtos.listenersFromUsersJson([
        {'user_id': 'u2', 'distance_bucket': 'under_150m'},
      ]).single;
      expect(listener.revealLevel, RevealLevel.level0);
      expect(listener.displayName, isNull);
    });

    test('infers level1 when reveal_level is absent but a display_name is present', () {
      final listener = ProximityDtos.listenersFromUsersJson([
        {'user_id': 'u3', 'display_name': 'Bea', 'distance_bucket': '1km_5km'},
      ]).single;
      expect(listener.revealLevel, RevealLevel.level1);
    });

    test('maps an explicit reveal_level 0/2', () {
      final results = ProximityDtos.listenersFromUsersJson([
        {'user_id': 'u4', 'reveal_level': 0, 'display_name': 'leaked-name'},
        {'user_id': 'u5', 'reveal_level': 2, 'display_name': 'Cid'},
      ]);
      // Defense in depth: even though the backend "leaked" a display_name
      // alongside reveal_level 0, the domain entity nulls it out.
      expect(results[0].revealLevel, RevealLevel.level0);
      expect(results[0].displayName, isNull);
      expect(results[1].revealLevel, RevealLevel.level2);
      expect(results[1].displayName, 'Cid');
    });

    test('unrecognized distance_bucket falls back to the least precise bucket', () {
      final listener = ProximityDtos.listenersFromUsersJson([
        {'user_id': 'u6', 'distance_bucket': '42'}, // hypothetical raw meters leak
      ]).single;
      expect(listener.distanceBucket, DistanceBucket.city);
    });

    test('missing distance_bucket falls back to the least precise bucket', () {
      final listener = ProximityDtos.listenersFromUsersJson([
        {'user_id': 'u7'},
      ]).single;
      expect(listener.distanceBucket, DistanceBucket.city);
    });

    test('now_playing without a title is treated as absent', () {
      final listener = ProximityDtos.listenersFromUsersJson([
        {'user_id': 'u8', 'now_playing': {'artist_name': 'Only artist'}},
      ]).single;
      expect(listener.nowPlaying, isNull);
    });

    test('a non-map now_playing is treated as absent', () {
      final listener = ProximityDtos.listenersFromUsersJson([
        {'user_id': 'u9', 'now_playing': 'not a map'},
      ]).single;
      expect(listener.nowPlaying, isNull);
    });

    test('non-map entries in the users array are skipped', () {
      final listeners = ProximityDtos.listenersFromUsersJson([
        'not a map',
        {'user_id': 'u10', 'distance_bucket': '5km_15km'},
      ]);
      expect(listeners, hasLength(1));
      expect(listeners.single.userId, 'u10');
    });

    test('all 4 distance_bucket wire codes map correctly', () {
      final listeners = ProximityDtos.listenersFromUsersJson([
        {'user_id': 'a', 'distance_bucket': 'under_150m'},
        {'user_id': 'b', 'distance_bucket': '150m_1km'},
        {'user_id': 'c', 'distance_bucket': '1km_5km'},
        {'user_id': 'd', 'distance_bucket': '5km_15km'},
      ]);
      expect(listeners.map((l) => l.distanceBucket), [
        DistanceBucket.veryClose,
        DistanceBucket.neighborhood,
        DistanceBucket.region,
        DistanceBucket.city,
      ]);
    });
  });

  group('updateNowPlayingToJson', () {
    test('produces the client outbound shape', () {
      expect(
        ProximityDtos.updateNowPlayingToJson(trackId: 't1', positionMs: 1500),
        {'track_id': 't1', 'position_ms': 1500},
      );
    });
  });

  group('visibility mode wire mapping', () {
    test('round-trips all 3 modes', () {
      for (final mode in ProximityVisibilityMode.values) {
        final wire = ProximityDtos.visibilityModeToWire(mode);
        expect(ProximityDtos.visibilityModeFromWire(wire), mode);
      }
    });

    test('an unrecognized wire value defaults to invisible', () {
      expect(ProximityDtos.visibilityModeFromWire('nonsense'), ProximityVisibilityMode.invisible);
      expect(ProximityDtos.visibilityModeFromWire(null), ProximityVisibilityMode.invisible);
    });
  });

  group('REST presence_visibility wire mapping (distinct literal set from WS)', () {
    test('round-trips all 3 modes using "everyone", not "visible"', () {
      for (final mode in ProximityVisibilityMode.values) {
        final wire = ProximityDtos.presenceVisibilityToWire(mode);
        expect(ProximityDtos.presenceVisibilityFromWire(wire), mode);
      }
      expect(
        ProximityDtos.presenceVisibilityToWire(ProximityVisibilityMode.everyone),
        'everyone',
      );
    });

    test('an unrecognized wire value defaults to invisible', () {
      expect(ProximityDtos.presenceVisibilityFromWire('nonsense'), ProximityVisibilityMode.invisible);
      expect(ProximityDtos.presenceVisibilityFromWire(null), ProximityVisibilityMode.invisible);
    });
  });

  group('settings JSON (backend/internal/presence/api/handlers.go real field names)', () {
    test('settingsToJson emits exactly the 4 PUT-able keys, never consent/enabled', () {
      final json = ProximityDtos.settingsToJson(
        ProximityPrivacySettings.disabled().copyWith(
          enabled: true,
          visibilityMode: ProximityVisibilityMode.friendsOnly,
          radius: ProximityRadius.km5,
          maxRevealLevel: RevealLevel.level1,
          paused: true,
          consentGivenAt: DateTime.utc(2026, 1, 1),
          consentRenewalDueAt: DateTime.utc(2026, 7, 1),
        ),
      );

      expect(json.keys.toSet(), {
        'presence_visibility',
        'visibility_radius_m',
        'reveal_level',
        'paused',
      });
      expect(json['presence_visibility'], 'friends_only');
      expect(json['visibility_radius_m'], 5000);
      expect(json['reveal_level'], 1);
      expect(json['paused'], true);
    });

    test('settingsFromJson reads the real GET/PUT/consent response field names', () {
      final settings = ProximityDtos.settingsFromJson({
        'presence_visibility': 'everyone',
        'presence_share_track': true,
        'proximity_consent_enabled': true,
        'proximity_consent_ts': '2026-01-01T00:00:00.000Z',
        'proximity_consent_renew_due': '2026-07-01T00:00:00.000Z',
        'visibility_radius_m': 5000,
        'reveal_level': 1,
        'paused': true,
      });

      expect(settings.enabled, isTrue);
      expect(settings.visibilityMode, ProximityVisibilityMode.everyone);
      expect(settings.radius, ProximityRadius.km5);
      expect(settings.maxRevealLevel, RevealLevel.level1);
      expect(settings.paused, isTrue);
      expect(settings.consentGivenAt, DateTime.utc(2026, 1, 1));
      expect(settings.consentRenewalDueAt, DateTime.utc(2026, 7, 1));
    });

    test('settingsToJson -> settingsFromJson round-trips the 4 PUT-able fields', () {
      final settings = ProximityPrivacySettings.disabled().copyWith(
        visibilityMode: ProximityVisibilityMode.friendsOnly,
        radius: ProximityRadius.km5,
        maxRevealLevel: RevealLevel.level1,
        paused: true,
      );

      final roundTripped = ProximityDtos.settingsFromJson(ProximityDtos.settingsToJson(settings));
      expect(roundTripped.visibilityMode, settings.visibilityMode);
      expect(roundTripped.radius, settings.radius);
      expect(roundTripped.maxRevealLevel, settings.maxRevealLevel);
      expect(roundTripped.paused, settings.paused);
      // Not round-tripped by design - consent/enabled are never PUT-able,
      // see settingsToJson's doc comment.
      expect(roundTripped.enabled, isFalse);
      expect(roundTripped.consentGivenAt, isNull);
    });

    test('settingsFromJson defaults missing/invalid fields safely', () {
      final settings = ProximityDtos.settingsFromJson(const {});
      expect(settings.enabled, isFalse);
      expect(settings.visibilityMode, ProximityVisibilityMode.invisible);
      expect(settings.radius, ProximityRadius.defaultValue);
      expect(settings.maxRevealLevel, RevealLevel.level0);
      expect(settings.paused, isFalse);
      expect(settings.consentGivenAt, isNull);
      expect(settings.consentRenewalDueAt, isNull);
    });

    test('settingsFromJson accepts a double visibility_radius_m', () {
      final settings = ProximityDtos.settingsFromJson(const {'visibility_radius_m': 1000.0});
      expect(settings.radius, ProximityRadius.km1);
    });

    test('settingsFromJson maps all 4 radius values and an unknown one to the default', () {
      expect(
        ProximityDtos.settingsFromJson(const {'visibility_radius_m': 150}).radius,
        ProximityRadius.m150,
      );
      expect(
        ProximityDtos.settingsFromJson(const {'visibility_radius_m': 5000}).radius,
        ProximityRadius.km5,
      );
      expect(
        ProximityDtos.settingsFromJson(const {'visibility_radius_m': 15000}).radius,
        ProximityRadius.km15,
      );
      expect(
        ProximityDtos.settingsFromJson(const {'visibility_radius_m': 999999}).radius,
        ProximityRadius.defaultValue,
      );
    });

    test('settingsFromJson maps all 3 reveal_level values', () {
      expect(
        ProximityDtos.settingsFromJson(const {'reveal_level': 1}).maxRevealLevel,
        RevealLevel.level1,
      );
      expect(
        ProximityDtos.settingsFromJson(const {'reveal_level': 2}).maxRevealLevel,
        RevealLevel.level2,
      );
    });

    test('settingsFromJson ignores an unparseable consent date string', () {
      final settings = ProximityDtos.settingsFromJson(const {'proximity_consent_ts': 'not-a-date'});
      expect(settings.consentGivenAt, isNull);
    });
  });
}
