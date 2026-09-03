import 'package:core_networking/core_networking.dart';
import 'package:core_networking/src/interceptors/auth_interceptor.dart';
import 'package:dio/dio.dart';
import 'package:http_mock_adapter/http_mock_adapter.dart';
import 'package:test/test.dart';

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
    return refreshedToken;
  }
}

void main() {
  test('does not retry a 401 that is already flagged as retried', () async {
    final dio = Dio(BaseOptions(baseUrl: 'https://api.smusic.test'));
    final dioAdapter = DioAdapter(dio: dio);
    final tokenSource = _FakeTokenSource(token: 't', refreshedToken: 'new');
    dio.interceptors.add(AuthInterceptor(tokenSource, dio));

    dioAdapter.onGet(
      '/v1/auth/me',
      (server) => server.reply(401, {'message': 'nope'}),
    );

    await expectLater(
      () => dio.get<dynamic>(
        '/v1/auth/me',
        options: Options(extra: {'authRetried': true}),
      ),
      throwsA(isA<DioException>()),
    );
    expect(tokenSource.refreshCalls, 0);
  });

  test('does not attempt refresh on a 401 for a skipAuth request', () async {
    final dio = Dio(BaseOptions(baseUrl: 'https://api.smusic.test'));
    final dioAdapter = DioAdapter(dio: dio);
    final tokenSource = _FakeTokenSource(token: 't', refreshedToken: 'new');
    dio.interceptors.add(AuthInterceptor(tokenSource, dio));

    dioAdapter.onPost(
      '/v1/auth/login',
      (server) => server.reply(401, {'message': 'bad credentials'}),
    );

    await expectLater(
      () => dio.post<dynamic>(
        '/v1/auth/login',
        options: Options(extra: {'skipAuth': true}),
      ),
      throwsA(isA<DioException>()),
    );
    expect(tokenSource.refreshCalls, 0);
  });

  test('propagates non-401 errors unchanged', () async {
    final dio = Dio(BaseOptions(baseUrl: 'https://api.smusic.test'));
    final dioAdapter = DioAdapter(dio: dio);
    final tokenSource = _FakeTokenSource(token: 't');
    dio.interceptors.add(AuthInterceptor(tokenSource, dio));

    dioAdapter.onGet(
      '/v1/catalog/tracks/x',
      (server) => server.reply(500, {'message': 'server error'}),
    );

    await expectLater(
      () => dio.get<dynamic>('/v1/catalog/tracks/x'),
      throwsA(
        isA<DioException>().having(
          (e) => e.response?.statusCode,
          'statusCode',
          500,
        ),
      ),
    );
  });

  test('surfaces the retry error when refresh succeeds but retry still fails', () async {
    final dio = Dio(BaseOptions(baseUrl: 'https://api.smusic.test'));
    final dioAdapter = DioAdapter(dio: dio);
    final tokenSource = _FakeTokenSource(token: 'expired', refreshedToken: 'new');
    dio.interceptors.add(AuthInterceptor(tokenSource, dio));

    dioAdapter.onGet(
      '/v1/auth/me',
      (server) => server.reply(401, {'message': 'still unauthorized'}),
    );

    await expectLater(
      () => dio.get<dynamic>('/v1/auth/me'),
      throwsA(isA<DioException>()),
    );
    expect(tokenSource.refreshCalls, 1);
  });
}
