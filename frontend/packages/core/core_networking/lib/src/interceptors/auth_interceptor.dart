import 'package:dio/dio.dart';

import '../auth_token_source.dart';

/// Attaches `Authorization: Bearer <token>` to every request that doesn't
/// opt out (via `options.extra['skipAuth'] = true`, used for
/// `/v1/auth/signup` and `/v1/auth/login`), and transparently refreshes +
/// retries once on a `401` response (backend-go.md section 4, "Tokens: JWT
/// de acesso de vida curta + refresh token ... rotacionado a cada uso").
class AuthInterceptor extends Interceptor {
  AuthInterceptor(this._tokenSource, this._dio);

  final AuthTokenSource _tokenSource;
  final Dio _dio;

  @override
  Future<void> onRequest(
    RequestOptions options,
    RequestInterceptorHandler handler,
  ) async {
    if (options.extra['skipAuth'] != true) {
      final token = await _tokenSource.currentAccessToken();
      if (token != null) {
        options.headers['Authorization'] = 'Bearer $token';
      }
    }
    handler.next(options);
  }

  @override
  Future<void> onError(
    DioException err,
    ErrorInterceptorHandler handler,
  ) async {
    final response = err.response;
    final alreadyRetried = err.requestOptions.extra['authRetried'] == true;
    final skipAuth = err.requestOptions.extra['skipAuth'] == true;

    if (response?.statusCode == 401 && !alreadyRetried && !skipAuth) {
      final newToken = await _tokenSource.refreshAccessToken();
      if (newToken != null) {
        // Copy, not mutate-in-place - see the comment in RetryInterceptor
        // for why reusing the exact same RequestOptions instance breaks
        // retries.
        final retryOptions = err.requestOptions.copyWith(
          headers: {
            ...err.requestOptions.headers,
            'Authorization': 'Bearer $newToken',
          },
          extra: {...err.requestOptions.extra, 'authRetried': true},
        );
        try {
          final retryResponse = await _dio.fetch<dynamic>(retryOptions);
          handler.resolve(retryResponse);
          return;
        } on DioException catch (retryError) {
          handler.next(retryError);
          return;
        }
      }
    }
    handler.next(err);
  }
}
