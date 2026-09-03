import 'package:dio/dio.dart';

import 'api_exception.dart';
import 'auth_token_source.dart';
import 'interceptors/auth_interceptor.dart';
import 'interceptors/retry_interceptor.dart';

/// Shared HTTP client for every `*_data` package, wrapping `dio` with the
/// backend's REST contract conventions (backend-go.md section 4): JSON
/// request/response bodies, bearer auth, and short-request-timeout with
/// retry for idempotent GETs.
class ApiClient {
  ApiClient({
    required String baseUrl,
    AuthTokenSource tokenSource = const NoAuthTokenSource(),
    Dio? dio,
  }) : _dio = dio ??
            Dio(
              BaseOptions(
                baseUrl: baseUrl,
                connectTimeout: const Duration(seconds: 10),
                receiveTimeout: const Duration(seconds: 15),
                contentType: 'application/json',
              ),
            ) {
    _dio.interceptors.addAll([
      AuthInterceptor(tokenSource, _dio),
      RetryInterceptor(_dio),
    ]);
  }

  final Dio _dio;

  /// Exposed for callers that need to build a [CancelToken] for
  /// in-flight-request cancellation (library_ui search debounce, per
  /// frontend-flutter.md section 3.3).
  Dio get rawClient => _dio;

  Future<Map<String, dynamic>> get(
    String path, {
    Map<String, dynamic>? queryParameters,
    CancelToken? cancelToken,
    bool skipAuth = false,
  }) async {
    return _send(
      () => _dio.get<dynamic>(
        path,
        queryParameters: queryParameters,
        cancelToken: cancelToken,
        options: Options(extra: {'skipAuth': skipAuth}),
      ),
    );
  }

  Future<Map<String, dynamic>> post(
    String path, {
    Object? data,
    CancelToken? cancelToken,
    bool skipAuth = false,
  }) async {
    return _send(
      () => _dio.post<dynamic>(
        path,
        data: data,
        cancelToken: cancelToken,
        options: Options(extra: {'skipAuth': skipAuth}),
      ),
    );
  }

  Future<Map<String, dynamic>> delete(
    String path, {
    Object? data,
    CancelToken? cancelToken,
  }) async {
    return _send(
      () => _dio.delete<dynamic>(path, data: data, cancelToken: cancelToken),
    );
  }

  Future<Map<String, dynamic>> _send(
    Future<Response<dynamic>> Function() request,
  ) async {
    try {
      final response = await request();
      final data = response.data;
      if (data == null) return const {};
      if (data is Map<String, dynamic>) return data;
      // Some endpoints (e.g. 204 No Content) return non-map bodies; callers
      // that expect a body should not hit this branch in practice.
      return {'data': data};
    } on DioException catch (e) {
      throw _toApiException(e);
    }
  }

  ApiException _toApiException(DioException e) {
    final isNetworkError = e.type == DioExceptionType.connectionTimeout ||
        e.type == DioExceptionType.receiveTimeout ||
        e.type == DioExceptionType.connectionError ||
        e.type == DioExceptionType.unknown;

    final statusCode = e.response?.statusCode;
    final serverMessage = _extractServerMessage(e.response?.data);

    return ApiException(
      message: serverMessage ?? e.message ?? 'Unknown network error',
      statusCode: statusCode,
      isNetworkError: isNetworkError,
    );
  }

  String? _extractServerMessage(Object? data) {
    if (data is Map<String, dynamic>) {
      final message = data['message'] ?? data['error'];
      if (message is String) return message;
    }
    return null;
  }
}
