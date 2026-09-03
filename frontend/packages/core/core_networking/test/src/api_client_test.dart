import 'package:core_networking/core_networking.dart';
import 'package:dio/dio.dart';
import 'package:http_mock_adapter/http_mock_adapter.dart';
import 'package:test/test.dart';

import '../support/sequenced_adapter.dart';

class _FakeTokenSource implements AuthTokenSource {
  _FakeTokenSource({this.token, this.refreshedToken});

  String? token;
  String? refreshedToken;
  int refreshCalls = 0;

  @override
  Future<String?> currentAccessToken() async => token;

  @override
  Future<String?> refreshAccessToken() async {
    refreshCalls++;
    token = refreshedToken;
    return refreshedToken;
  }
}

void main() {
  group('ApiClient success paths', () {
    late Dio dio;
    late DioAdapter dioAdapter;
    late ApiClient client;

    setUp(() {
      dio = Dio(BaseOptions(baseUrl: 'https://api.smusic.test'));
      dioAdapter = DioAdapter(dio: dio);
      client = ApiClient(baseUrl: 'https://api.smusic.test', dio: dio);
    });

    test('get() returns decoded map body', () async {
      dioAdapter.onGet(
        '/v1/catalog/tracks/1',
        (server) => server.reply(200, {'id': '1', 'title': 'Song'}),
      );

      final result = await client.get('/v1/catalog/tracks/1');
      expect(result, {'id': '1', 'title': 'Song'});
    });

    test('get() returns empty map when body is null', () async {
      dioAdapter.onGet('/v1/library/me/playlists', (server) => server.reply(204, null));

      final result = await client.get('/v1/library/me/playlists');
      expect(result, isEmpty);
    });

    test('post() sends body and returns response', () async {
      dioAdapter.onPost(
        '/v1/auth/login',
        (server) => server.reply(200, {'access_token': 'abc'}),
        data: {'email': 'a@b.com', 'password': 'x'},
      );

      final result = await client.post(
        '/v1/auth/login',
        data: {'email': 'a@b.com', 'password': 'x'},
        skipAuth: true,
      );
      expect(result['access_token'], 'abc');
    });

    test('delete() sends request and returns response', () async {
      dioAdapter.onDelete(
        '/v1/library/me/playlists/1/tracks/2',
        (server) => server.reply(204, null),
      );

      final result =
          await client.delete('/v1/library/me/playlists/1/tracks/2');
      expect(result, isEmpty);
    });

    test('wraps a non-map JSON body under a "data" key', () async {
      dioAdapter.onGet(
        '/v1/catalog/search',
        (server) => server.reply(200, ['a', 'b']),
      );

      final result = await client.get('/v1/catalog/search');
      expect(result, {'data': ['a', 'b']});
    });
  });

  group('ApiClient default construction', () {
    test('builds its own Dio instance when none is injected', () {
      final client = ApiClient(baseUrl: 'https://api.smusic.test');
      expect(client.rawClient.options.baseUrl, 'https://api.smusic.test');
    });
  });

  group('ApiClient auth header attachment', () {
    test('attaches Authorization header from token source', () async {
      final dio = Dio(BaseOptions(baseUrl: 'https://api.smusic.test'));
      final dioAdapter = DioAdapter(dio: dio);
      final tokenSource = _FakeTokenSource(token: 'my-token');

      final client = ApiClient(
        baseUrl: 'https://api.smusic.test',
        tokenSource: tokenSource,
        dio: dio,
      );

      // Registered *after* ApiClient's own interceptors so it observes the
      // fully-prepared request (dio runs onRequest interceptors in
      // registration order).
      String? capturedAuthHeader;
      dio.interceptors.add(
        InterceptorsWrapper(
          onRequest: (options, handler) {
            capturedAuthHeader = options.headers['Authorization'] as String?;
            handler.next(options);
          },
        ),
      );

      dioAdapter.onGet('/v1/auth/me', (server) => server.reply(200, {}));
      await client.get('/v1/auth/me');

      expect(capturedAuthHeader, 'Bearer my-token');
    });

    test('does not attach header when skipAuth is true', () async {
      final dio = Dio(BaseOptions(baseUrl: 'https://api.smusic.test'));
      final dioAdapter = DioAdapter(dio: dio);
      final tokenSource = _FakeTokenSource(token: 'my-token');

      final client = ApiClient(
        baseUrl: 'https://api.smusic.test',
        tokenSource: tokenSource,
        dio: dio,
      );

      String? capturedAuthHeader = 'unset';
      dio.interceptors.add(
        InterceptorsWrapper(
          onRequest: (options, handler) {
            capturedAuthHeader = options.headers['Authorization'] as String?;
            handler.next(options);
          },
        ),
      );

      dioAdapter.onPost('/v1/auth/signup', (server) => server.reply(200, {}));
      await client.post('/v1/auth/signup', skipAuth: true);

      expect(capturedAuthHeader, isNull);
    });
  });

  group('ApiClient 401 refresh-and-retry', () {
    test('refreshes token and retries once on 401', () async {
      final dio = Dio(BaseOptions(baseUrl: 'https://api.smusic.test'));
      final tokenSource =
          _FakeTokenSource(token: 'expired', refreshedToken: 'fresh');

      final adapter = SequencedAdapter([
        (options) => SequencedAdapter.jsonResponse(401, {'message': 'expired'}),
        (options) => SequencedAdapter.jsonResponse(200, {'user_id': '42'}),
      ]);
      dio.httpClientAdapter = adapter;

      final client = ApiClient(
        baseUrl: 'https://api.smusic.test',
        tokenSource: tokenSource,
        dio: dio,
      );

      final result = await client.get('/v1/auth/me');

      expect(result['user_id'], '42');
      expect(tokenSource.refreshCalls, 1);
      expect(adapter.callCount, 2);
    });

    test('surfaces ApiException(401) when refresh fails', () async {
      final dio = Dio(BaseOptions(baseUrl: 'https://api.smusic.test'));
      final dioAdapter = DioAdapter(dio: dio);
      final tokenSource = _FakeTokenSource(token: 'expired');

      final client = ApiClient(
        baseUrl: 'https://api.smusic.test',
        tokenSource: tokenSource,
        dio: dio,
      );

      dioAdapter.onGet(
        '/v1/auth/me',
        (server) => server.reply(401, {'message': 'expired'}),
      );

      await expectLater(
        () => client.get('/v1/auth/me'),
        throwsA(
          isA<ApiException>()
              .having((e) => e.statusCode, 'statusCode', 401)
              .having((e) => e.isUnauthorized, 'isUnauthorized', isTrue),
        ),
      );
    });
  });

  group('ApiClient error mapping', () {
    test('maps 404 response to ApiException with server message', () async {
      final dio = Dio(BaseOptions(baseUrl: 'https://api.smusic.test'));
      final dioAdapter = DioAdapter(dio: dio);
      final client = ApiClient(baseUrl: 'https://api.smusic.test', dio: dio);

      dioAdapter.onGet(
        '/v1/catalog/tracks/missing',
        (server) => server.reply(404, {'message': 'track not found'}),
      );

      await expectLater(
        () => client.get('/v1/catalog/tracks/missing'),
        throwsA(
          isA<ApiException>()
              .having((e) => e.statusCode, 'statusCode', 404)
              .having((e) => e.isNotFound, 'isNotFound', isTrue)
              .having((e) => e.message, 'message', 'track not found'),
        ),
      );
    });

    test('maps connection errors to isNetworkError=true', () async {
      final dio = Dio(BaseOptions(baseUrl: 'https://api.smusic.test'));
      final client = ApiClient(baseUrl: 'https://api.smusic.test', dio: dio);

      // Every attempt (original + RetryInterceptor's retries) fails the same
      // way - a bounded number of canned responses is enough since
      // SequencedAdapter repeats the last entry once exhausted.
      dio.httpClientAdapter = SequencedAdapter([
        (options) => throw DioException.connectionError(
              requestOptions: options,
              reason: 'Failed host lookup',
            ),
      ]);

      await expectLater(
        () => client.get('/v1/catalog/search'),
        throwsA(
          isA<ApiException>()
              .having((e) => e.isNetworkError, 'isNetworkError', isTrue),
        ),
      );
    });
  });
}
