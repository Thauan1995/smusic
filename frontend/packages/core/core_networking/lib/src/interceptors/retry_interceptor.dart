import 'package:dio/dio.dart';

/// Retries idempotent (GET) requests up to [maxRetries] times, with a fixed
/// [delay] between attempts, when the failure is a connection-level error
/// (timeout/no connection) rather than a real HTTP error response. Never
/// retries non-GET requests - retrying a POST blindly could double-submit
/// (e.g. `/v1/library/me/playlists`).
class RetryInterceptor extends Interceptor {
  RetryInterceptor(
    this._dio, {
    this.maxRetries = 2,
    this.delay = const Duration(milliseconds: 300),
  });

  final Dio _dio;
  final int maxRetries;
  final Duration delay;

  static bool _isRetriableError(DioException err) {
    return err.type == DioExceptionType.connectionTimeout ||
        err.type == DioExceptionType.receiveTimeout ||
        err.type == DioExceptionType.connectionError;
  }

  @override
  Future<void> onError(
    DioException err,
    ErrorInterceptorHandler handler,
  ) async {
    final isGet = err.requestOptions.method.toUpperCase() == 'GET';
    final attempt = (err.requestOptions.extra['retryAttempt'] as int?) ?? 0;

    if (isGet && _isRetriableError(err) && attempt < maxRetries) {
      await Future<void>.delayed(delay);
      // A *copy* of the RequestOptions is required here, not the original
      // instance: http_mock_adapter (and, we found empirically, dio's own
      // internal request bookkeeping) associates state with the exact
      // RequestOptions object identity, so re-fetching the same mutated
      // instance replays the previous failure forever instead of issuing a
      // new request.
      final retryOptions = err.requestOptions.copyWith(
        extra: {...err.requestOptions.extra, 'retryAttempt': attempt + 1},
      );
      try {
        final response = await _dio.fetch<dynamic>(retryOptions);
        handler.resolve(response);
        return;
      } on DioException catch (retryError) {
        handler.next(retryError);
        return;
      }
    }
    handler.next(err);
  }
}
