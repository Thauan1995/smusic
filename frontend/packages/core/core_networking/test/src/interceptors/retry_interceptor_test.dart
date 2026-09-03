import 'package:core_networking/src/interceptors/retry_interceptor.dart';
import 'package:dio/dio.dart';
import 'package:http_mock_adapter/http_mock_adapter.dart';
import 'package:test/test.dart';

import '../../support/sequenced_adapter.dart';

void main() {
  test('retries GET up to maxRetries then succeeds', () async {
    final dio = Dio(BaseOptions(baseUrl: 'https://api.smusic.test'));
    dio.interceptors.add(
      RetryInterceptor(dio, maxRetries: 2, delay: Duration.zero),
    );

    final adapter = SequencedAdapter([
      (options) => throw DioException.connectionError(
            requestOptions: options,
            reason: 'timeout',
          ),
      (options) => throw DioException.connectionError(
            requestOptions: options,
            reason: 'timeout',
          ),
      (options) => SequencedAdapter.jsonResponse(200, {'results': []}),
    ]);
    dio.httpClientAdapter = adapter;

    final response = await dio.get<dynamic>('/v1/catalog/search');
    expect(response.data, {'results': []});
    expect(adapter.callCount, 3);
  });

  test('gives up after maxRetries exhausted', () async {
    final dio = Dio(BaseOptions(baseUrl: 'https://api.smusic.test'));
    dio.interceptors.add(
      RetryInterceptor(dio, maxRetries: 1, delay: Duration.zero),
    );

    final adapter = SequencedAdapter([
      (options) => throw DioException.connectionError(
            requestOptions: options,
            reason: 'timeout',
          ),
    ]);
    dio.httpClientAdapter = adapter;

    await expectLater(
      () => dio.get<dynamic>('/v1/catalog/search'),
      throwsA(isA<DioException>()),
    );
    expect(adapter.callCount, 2); // original attempt + 1 retry
  });

  test('does not retry non-GET requests', () async {
    final dio = Dio(BaseOptions(baseUrl: 'https://api.smusic.test'));
    final dioAdapter = DioAdapter(dio: dio);
    dio.interceptors.add(
      RetryInterceptor(dio, maxRetries: 2, delay: Duration.zero),
    );

    var callCount = 0;
    dioAdapter.onPost('/v1/library/me/playlists', (server) {
      callCount++;
      server.throws(
        0,
        DioException.connectionError(
          requestOptions: RequestOptions(path: '/v1/library/me/playlists'),
          reason: 'timeout',
        ),
      );
    });

    await expectLater(
      () => dio.post<dynamic>('/v1/library/me/playlists'),
      throwsA(isA<DioException>()),
    );
    expect(callCount, 1);
  });

  test('does not retry non-connection errors (e.g. real 500 response)', () async {
    final dio = Dio(BaseOptions(baseUrl: 'https://api.smusic.test'));
    final dioAdapter = DioAdapter(dio: dio);
    dio.interceptors.add(
      RetryInterceptor(dio, maxRetries: 2, delay: Duration.zero),
    );

    var callCount = 0;
    dioAdapter.onGet('/v1/catalog/search', (server) {
      callCount++;
      server.reply(500, {'message': 'server error'});
    });

    await expectLater(
      () => dio.get<dynamic>('/v1/catalog/search'),
      throwsA(isA<DioException>()),
    );
    expect(callCount, 1);
  });
}
