/// Access + refresh token pair (backend-go.md section 4: "JWT de acesso de
/// vida curta (~15 min) + refresh token opaco de vida longa, rotacionado a
/// cada uso").
class AuthTokens {
  const AuthTokens({
    required this.accessToken,
    required this.refreshToken,
    required this.accessTokenExpiresAt,
  });

  final String accessToken;
  final String refreshToken;
  final DateTime accessTokenExpiresAt;

  /// True once the access token is expired (or within [skew] of expiring),
  /// used by `RefreshSessionUseCase` to proactively refresh rather than
  /// waiting for a 401.
  bool isExpired({DateTime? now, Duration skew = const Duration(seconds: 30)}) {
    final reference = now ?? DateTime.now();
    return reference.isAfter(accessTokenExpiresAt.subtract(skew));
  }

  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      other is AuthTokens &&
          other.accessToken == accessToken &&
          other.refreshToken == refreshToken &&
          other.accessTokenExpiresAt == accessTokenExpiresAt;

  @override
  int get hashCode =>
      Object.hash(accessToken, refreshToken, accessTokenExpiresAt);
}
