/// Read-only view onto the current access token, implemented by
/// `auth_data` and injected into [ApiClient] at composition time (`app/*`).
///
/// `core_networking` intentionally does not depend on `auth_domain`/
/// `auth_data` (core has no feature dependencies, per
/// docs/architecture/frontend-flutter.md section 1.2) - this narrow
/// interface is how the auth interceptor gets a token without that
/// dependency existing.
abstract interface class AuthTokenSource {
  /// Returns the current access token, or `null` if signed out.
  Future<String?> currentAccessToken();

  /// Called by [ApiClient] when a request fails with 401, asking the token
  /// source to refresh and return a new access token (or `null` if refresh
  /// failed / no refresh token available, in which case the request is not
  /// retried and the caller sees the original 401 as an [ApiException]).
  Future<String?> refreshAccessToken();
}

/// No-op implementation for unauthenticated requests / tests.
class NoAuthTokenSource implements AuthTokenSource {
  const NoAuthTokenSource();

  @override
  Future<String?> currentAccessToken() async => null;

  @override
  Future<String?> refreshAccessToken() async => null;
}
