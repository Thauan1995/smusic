/// Normalized error shape surfaced to every `*_data` package, so repository
/// implementations never need to know about `dio`'s `DioException`
/// directly.
class ApiException implements Exception {
  const ApiException({
    required this.message,
    this.statusCode,
    this.code,
    this.isNetworkError = false,
  });

  final String message;
  final int? statusCode;

  /// Machine-readable error code from the backend's `{error: {code,
  /// message}}` envelope (`httpx.WriteError`'s `code` argument, e.g.
  /// `"mfa_required"`, `"invalid_input"`) - null when the response didn't
  /// carry one (a network error, or a body shaped some other way).
  /// Callers that need to branch on *which* error happened, not just its
  /// HTTP status, should key off this rather than parsing [message].
  final String? code;

  /// True when the request never reached the server (timeout, DNS,
  /// connectivity) - useful for retry/backoff decisions upstream.
  final bool isNetworkError;

  bool get isUnauthorized => statusCode == 401;
  bool get isNotFound => statusCode == 404;

  @override
  String toString() =>
      'ApiException(statusCode: $statusCode, code: $code, message: $message)';
}
