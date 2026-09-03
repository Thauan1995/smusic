/// Normalized error shape surfaced to every `*_data` package, so repository
/// implementations never need to know about `dio`'s `DioException`
/// directly.
class ApiException implements Exception {
  const ApiException({
    required this.message,
    this.statusCode,
    this.isNetworkError = false,
  });

  final String message;
  final int? statusCode;

  /// True when the request never reached the server (timeout, DNS,
  /// connectivity) - useful for retry/backoff decisions upstream.
  final bool isNetworkError;

  bool get isUnauthorized => statusCode == 401;
  bool get isNotFound => statusCode == 404;

  @override
  String toString() =>
      'ApiException(statusCode: $statusCode, message: $message)';
}
