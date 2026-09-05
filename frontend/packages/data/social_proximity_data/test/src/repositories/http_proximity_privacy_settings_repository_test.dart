import 'package:core_networking/core_networking.dart';
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http_mock_adapter/http_mock_adapter.dart';
import 'package:social_proximity_data/social_proximity_data.dart';
import 'package:social_proximity_domain/social_proximity_domain.dart';

import '../../support/sequenced_adapter.dart';

void main() {
  late Dio dio;
  late DioAdapter dioAdapter;
  late HttpProximityPrivacySettingsRepository repository;

  setUp(() {
    dio = Dio(BaseOptions(baseUrl: 'https://api.smusic.test'));
    dioAdapter = DioAdapter(dio: dio, matcher: const UrlRequestMatcher(matchMethod: true));
    final client = ApiClient(baseUrl: 'https://api.smusic.test', dio: dio);
    repository = HttpProximityPrivacySettingsRepository(client);
  });

  test('fetch parses the settings payload', () async {
    dioAdapter.onGet(
      '/v1/presence/settings',
      (server) => server.reply(200, {
        'presence_visibility': 'friends_only',
        'presence_share_track': true,
        'proximity_consent_enabled': true,
        'visibility_radius_m': 5000,
        'reveal_level': 1,
        'paused': false,
      }),
    );

    final settings = await repository.fetch();
    expect(settings.enabled, isTrue);
    expect(settings.visibilityMode, ProximityVisibilityMode.friendsOnly);
    expect(settings.radius, ProximityRadius.km5);
    expect(settings.maxRevealLevel, RevealLevel.level1);
    expect(settings.paused, isFalse);
  });

  test('fetch maps a network failure', () async {
    // Not `dioAdapter.onGet(...).throws(...)` - GET requests go through
    // `RetryInterceptor`, which retries connection-level errors with a real
    // `Future.delayed`; `http_mock_adapter`'s single-shot callback combined
    // with that retry loop was observed to hang the test runner well past
    // its 30s timeout. `SequencedAdapter` fails every attempt the same way
    // deterministically instead (same pattern `core_networking`'s own
    // `api_client_test.dart` uses for this exact scenario).
    final failingDio = Dio(BaseOptions(baseUrl: 'https://api.smusic.test'))
      ..httpClientAdapter = SequencedAdapter([
        (options) => throw DioException.connectionError(
              requestOptions: options,
              reason: 'offline',
            ),
      ]);
    final failingClient = ApiClient(baseUrl: 'https://api.smusic.test', dio: failingDio);
    final failingRepository = HttpProximityPrivacySettingsRepository(failingClient);

    await expectLater(
      failingRepository.fetch,
      throwsA(
        isA<ProximityException>().having((e) => e.kind, 'kind', ProximityExceptionKind.network),
      ),
    );
  });

  test('update PUTs only the 4 settable fields and returns the response', () async {
    dioAdapter.onPut(
      '/v1/presence/settings',
      (server) => server.reply(200, {
        'presence_visibility': 'everyone',
        'proximity_consent_enabled': true,
        'visibility_radius_m': 1000,
        'reveal_level': 0,
        'paused': true,
      }),
      data: {
        'presence_visibility': 'everyone',
        'visibility_radius_m': 1000,
        'reveal_level': 0,
        'paused': true,
      },
    );

    final settings = await repository.update(
      ProximityPrivacySettings.disabled().copyWith(
        visibilityMode: ProximityVisibilityMode.everyone,
        paused: true,
      ),
    );
    expect(settings.paused, isTrue);
    expect(settings.visibilityMode, ProximityVisibilityMode.everyone);
  });

  test('update maps a 401 to unauthorized', () async {
    dioAdapter.onPut(
      '/v1/presence/settings',
      (server) => server.reply(401, {'message': 'nope'}),
    );

    await expectLater(
      () => repository.update(ProximityPrivacySettings.disabled()),
      throwsA(
        isA<ProximityException>()
            .having((e) => e.kind, 'kind', ProximityExceptionKind.unauthorized),
      ),
    );
  });

  test('grantConsent POSTs to /v1/presence/consent with no body', () async {
    dioAdapter.onPost(
      '/v1/presence/consent',
      (server) => server.reply(200, {
        'proximity_consent_enabled': true,
        'proximity_consent_ts': '2026-01-01T00:00:00.000Z',
        'proximity_consent_renew_due': '2026-07-01T00:00:00.000Z',
      }),
    );

    final settings = await repository.grantConsent();
    expect(settings.enabled, isTrue);
    expect(settings.consentGivenAt, DateTime.utc(2026, 1, 1));
    expect(settings.consentRenewalDueAt, DateTime.utc(2026, 7, 1));
  });

  test('grantConsent maps an unknown server error', () async {
    dioAdapter.onPost(
      '/v1/presence/consent',
      (server) => server.reply(500, {'message': 'boom'}),
    );

    await expectLater(
      repository.grantConsent,
      throwsA(
        isA<ProximityException>().having((e) => e.kind, 'kind', ProximityExceptionKind.unknown),
      ),
    );
  });

  test('grantConsent maps a 403 mfa_required to ProximityExceptionKind.mfaRequired', () async {
    dioAdapter.onPost(
      '/v1/presence/consent',
      (server) => server.reply(403, {
        'error': {
          'code': 'mfa_required',
          'message': 'presence: MFA required to enable proximity discovery',
        },
      }),
    );

    await expectLater(
      repository.grantConsent,
      throwsA(
        isA<ProximityException>()
            .having((e) => e.kind, 'kind', ProximityExceptionKind.mfaRequired),
      ),
    );
  });

  test('revokeConsent DELETEs /v1/presence/consent and returns the response', () async {
    dioAdapter.onDelete(
      '/v1/presence/consent',
      (server) => server.reply(200, {
        'proximity_consent_enabled': false,
        'paused': true,
      }),
    );

    final settings = await repository.revokeConsent();
    expect(settings.enabled, isFalse);
    expect(settings.paused, isTrue);
  });

  test('revokeConsent maps a 401 to unauthorized', () async {
    dioAdapter.onDelete(
      '/v1/presence/consent',
      (server) => server.reply(401, {'message': 'nope'}),
    );

    await expectLater(
      repository.revokeConsent,
      throwsA(
        isA<ProximityException>()
            .having((e) => e.kind, 'kind', ProximityExceptionKind.unauthorized),
      ),
    );
  });
}
