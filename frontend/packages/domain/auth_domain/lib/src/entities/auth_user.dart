/// Maps to `GET /v1/auth/me`'s response shape (backend-go.md section 4).
class AuthUser {
  const AuthUser({
    required this.userId,
    required this.displayName,
    required this.email,
  });

  final String userId;
  final String displayName;
  final String email;

  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      other is AuthUser &&
          other.userId == userId &&
          other.displayName == displayName &&
          other.email == email;

  @override
  int get hashCode => Object.hash(userId, displayName, email);

  @override
  String toString() =>
      'AuthUser(userId: $userId, displayName: $displayName, email: $email)';
}
