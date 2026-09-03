import 'auth_tokens.dart';
import 'auth_user.dart';

/// The result of a successful signup/login/refresh: the authenticated user
/// plus the token pair to use for subsequent requests.
class AuthSession {
  const AuthSession({required this.user, required this.tokens});

  final AuthUser user;
  final AuthTokens tokens;
}
