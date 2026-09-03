import '../entities/auth_session.dart';
import '../entities/auth_tokens.dart';
import '../entities/auth_user.dart';

/// Implemented by `auth_data` against the endpoints in backend-go.md
/// section 4 (`/v1/auth/signup`, `/v1/auth/login`, `/v1/auth/refresh`,
/// `/v1/auth/me`). Throws [AuthException] (see auth_exception.dart) on
/// failure - never a transport-layer exception type.
abstract interface class AuthRepository {
  Future<AuthSession> signUp({
    required String email,
    required String password,
    required String displayName,
  });

  Future<AuthSession> logIn({required String email, required String password});

  Future<AuthTokens> refresh({required String refreshToken});

  Future<AuthUser> getCurrentUser();

  Future<void> logOut({required String refreshToken});
}
