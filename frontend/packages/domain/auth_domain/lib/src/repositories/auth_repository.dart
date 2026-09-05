import '../entities/auth_session.dart';
import '../entities/auth_tokens.dart';
import '../entities/auth_user.dart';
import '../entities/mfa_enrollment.dart';

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

  /// `POST /v1/auth/mfa/enroll` - starts (or restarts) TOTP enrollment for
  /// the signed-in user, returning a fresh secret every call.
  Future<MfaEnrollment> enrollMfa();

  /// `POST /v1/auth/mfa/verify` - confirms the code the user entered from
  /// their authenticator app matches the secret from [enrollMfa], marking
  /// the account as having a verified second factor server-side. Throws
  /// [AuthException] with [AuthExceptionKind.invalidMfaCode] on a wrong/
  /// expired code.
  Future<void> verifyMfa({required String code});
}
