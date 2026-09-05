import 'package:auth_data/auth_data.dart';
import 'package:auth_domain/auth_domain.dart';
import 'package:core_networking/core_networking.dart';
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http_mock_adapter/http_mock_adapter.dart';

void main() {
  late Dio dio;
  late DioAdapter dioAdapter;
  late ApiClient apiClient;
  late HttpAuthRepository repository;

  setUp(() {
    dio = Dio(BaseOptions(baseUrl: 'https://api.smusic.test'));
    // UrlRequestMatcher (route + method only): the default
    // FullHttpRequestMatcher requires the mocked request body to match
    // exactly, which is unnecessary friction for these repository tests -
    // the DTO-mapping tests (auth_dtos_test.dart) already cover exact
    // request/response shapes.
    dioAdapter = DioAdapter(
      dio: dio,
      matcher: const UrlRequestMatcher(matchMethod: true),
    );
    apiClient = ApiClient(baseUrl: 'https://api.smusic.test', dio: dio);
    repository = HttpAuthRepository(apiClient);
  });

  group('signUp', () {
    test('builds an AuthSession from the signup response', () async {
      dioAdapter.onPost(
        '/v1/auth/signup',
        (server) => server.reply(200, {
          'user_id': '1',
          'access_token': 'a',
          'refresh_token': 'r',
        }),
        data: {'email': 'a@b.com', 'password': 'p', 'display_name': 'Ana'},
      );

      final session = await repository.signUp(
        email: 'a@b.com',
        password: 'p',
        displayName: 'Ana',
      );

      expect(session.user.userId, '1');
      expect(session.user.displayName, 'Ana');
      expect(session.user.email, 'a@b.com');
      expect(session.tokens.accessToken, 'a');
    });

    test('maps a 409 to emailAlreadyInUse', () async {
      dioAdapter.onPost(
        '/v1/auth/signup',
        (server) => server.reply(409, {'message': 'email taken'}),
      );

      await expectLater(
        () => repository.signUp(email: 'a@b.com', password: 'p', displayName: 'Ana'),
        throwsA(
          isA<AuthException>().having((e) => e.kind, 'kind', AuthExceptionKind.emailAlreadyInUse),
        ),
      );
    });
  });

  group('logIn', () {
    test('fetches /me after login to build the session', () async {
      dioAdapter.onPost(
        '/v1/auth/login',
        (server) => server.reply(200, {'access_token': 'a', 'refresh_token': 'r'}),
      );
      dioAdapter.onGet(
        '/v1/auth/me',
        (server) => server.reply(200, {
          'user_id': '1',
          'display_name': 'Ana',
          'email': 'ana@example.com',
        }),
      );

      final session = await repository.logIn(email: 'a@b.com', password: 'p');

      expect(session.user.displayName, 'Ana');
      expect(session.user.email, 'ana@example.com');
    });

    test('maps a 401 to invalidCredentials', () async {
      dioAdapter.onPost(
        '/v1/auth/login',
        (server) => server.reply(401, {'message': 'bad credentials'}),
      );

      await expectLater(
        () => repository.logIn(email: 'a@b.com', password: 'wrong'),
        throwsA(
          isA<AuthException>().having((e) => e.kind, 'kind', AuthExceptionKind.invalidCredentials),
        ),
      );
    });
  });

  group('refresh', () {
    test('returns refreshed tokens', () async {
      dioAdapter.onPost(
        '/v1/auth/refresh',
        (server) => server.reply(200, {'access_token': 'new-a'}),
      );

      final tokens = await repository.refresh(refreshToken: 'old-r');

      expect(tokens.accessToken, 'new-a');
      expect(tokens.refreshToken, 'old-r'); // fallback, response omitted it
    });

    test('maps network errors to AuthExceptionKind.network', () async {
      dioAdapter.onPost(
        '/v1/auth/refresh',
        (server) => server.throws(
          0,
          DioException.connectionError(
            requestOptions: RequestOptions(path: '/v1/auth/refresh'),
            reason: 'offline',
          ),
        ),
      );

      await expectLater(
        () => repository.refresh(refreshToken: 'r'),
        throwsA(
          isA<AuthException>().having((e) => e.kind, 'kind', AuthExceptionKind.network),
        ),
      );
    });
  });

  group('getCurrentUser', () {
    test('returns the current user', () async {
      dioAdapter.onGet(
        '/v1/auth/me',
        (server) => server.reply(200, {'user_id': '1', 'display_name': 'Ana'}),
      );

      final user = await repository.getCurrentUser();
      expect(user.userId, '1');
    });

    test('maps unknown server errors to AuthExceptionKind.unknown', () async {
      dioAdapter.onGet(
        '/v1/auth/me',
        (server) => server.reply(500, {'message': 'server error'}),
      );

      await expectLater(
        () => repository.getCurrentUser(),
        throwsA(
          isA<AuthException>().having((e) => e.kind, 'kind', AuthExceptionKind.unknown),
        ),
      );
    });
  });

  group('logOut', () {
    test('posts the refresh token and completes', () async {
      dioAdapter.onPost('/v1/auth/logout', (server) => server.reply(204, null));
      await repository.logOut(refreshToken: 'r');
    });
  });

  group('enrollMfa', () {
    test('returns the secret and otpauth url', () async {
      dioAdapter.onPost(
        '/v1/auth/mfa/enroll',
        (server) => server.reply(200, {
          'secret': 'JBSWY3DPEHPK3PXP',
          'otpauth_url': 'otpauth://totp/smusic:a@b.com?secret=JBSWY3DPEHPK3PXP&issuer=smusic',
        }),
      );

      final enrollment = await repository.enrollMfa();
      expect(enrollment.secret, 'JBSWY3DPEHPK3PXP');
      expect(enrollment.otpauthUrl, contains('otpauth://totp'));
    });
  });

  group('verifyMfa', () {
    test('completes on a valid code', () async {
      dioAdapter.onPost('/v1/auth/mfa/verify', (server) => server.reply(204, null));
      await repository.verifyMfa(code: '123456');
    });

    test('maps a 401 invalid_code response to AuthExceptionKind.invalidMfaCode', () async {
      dioAdapter.onPost(
        '/v1/auth/mfa/verify',
        (server) => server.reply(401, {
          'error': {'code': 'invalid_code', 'message': 'invalid or expired code'},
        }),
      );

      await expectLater(
        () => repository.verifyMfa(code: '000000'),
        throwsA(
          isA<AuthException>().having((e) => e.kind, 'kind', AuthExceptionKind.invalidMfaCode),
        ),
      );
    });
  });
}
