import 'package:auth_domain/auth_domain.dart';

/// Mapping between backend-go.md section 4's auth JSON shapes and
/// `auth_domain` entities. Kept as free functions (not full DTO classes)
/// since every payload here is small and one-directional
/// (response -> entity); there is no round-trip serialization need.
class AuthDtos {
  // Never-instantiated utility class constructor.
  const AuthDtos._(); // coverage:ignore-line

  /// `POST /v1/auth/signup` / `/login` response:
  /// `{ user_id, access_token, refresh_token }` - no `display_name`/`email`
  /// echoed back, so the caller (`HttpAuthRepository`) fills those in from
  /// what it already knows (the email typed at signup/login) plus a
  /// follow-up `GET /v1/auth/me` call. `accessTokenExpiresAt` is not part
  /// of the JSON envelope either (backend-go.md documents "~15 min" JWT
  /// lifetime as a policy, not a field) - approximated client-side.
  ///
  /// DEVIATION FROM SPEC: this heuristic 15-minute expiry (rather than
  /// decoding the JWT's own `exp` claim) is a documented simplification -
  /// see frontend/README.md. The `AuthInterceptor`'s reactive 401-refresh
  /// path (core_networking) is the real safety net if this estimate is
  /// wrong in either direction.
  static AuthTokens tokensFromLoginResponse(
    Map<String, dynamic> json, {
    String? fallbackRefreshToken,
    DateTime? now,
  }) {
    final accessToken = json['access_token'] as String;
    final refreshToken =
        (json['refresh_token'] as String?) ?? fallbackRefreshToken;
    if (refreshToken == null) {
      throw const FormatException(
        'auth response has no refresh_token and no fallback was provided',
      );
    }
    return AuthTokens(
      accessToken: accessToken,
      refreshToken: refreshToken,
      accessTokenExpiresAt:
          (now ?? DateTime.now()).add(const Duration(minutes: 15)),
    );
  }

  /// `GET /v1/auth/me` response: `{ user_id, display_name, ... }`. `email`
  /// is assumed present under the same key convention as the rest of the
  /// contract (`snake_case`); if the backend's `...` doesn't actually
  /// include it, `HttpAuthRepository` falls back to the email the caller
  /// already knows (signup/login input).
  static AuthUser userFromMeResponse(
    Map<String, dynamic> json, {
    String? fallbackEmail,
  }) {
    return AuthUser(
      userId: json['user_id'] as String,
      displayName: json['display_name'] as String,
      email: (json['email'] as String?) ?? fallbackEmail ?? '',
    );
  }
}
