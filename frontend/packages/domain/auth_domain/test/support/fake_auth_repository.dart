import 'package:auth_domain/auth_domain.dart';

/// Deterministic fake used across `auth_domain`'s own tests. Every method
/// records its call args and returns/throws a value the test configures
/// beforehand - no network, no real tokens.
class FakeAuthRepository implements AuthRepository {
  AuthSession? signUpResult;
  AuthSession? logInResult;
  AuthTokens? refreshResult;
  AuthUser? currentUserResult;
  Object? throwOnSignUp;
  Object? throwOnLogIn;
  Object? throwOnRefresh;
  Object? throwOnGetCurrentUser;
  Object? throwOnLogOut;
  MfaEnrollment? enrollMfaResult;
  Object? throwOnEnrollMfa;
  Object? throwOnVerifyMfa;

  String? lastSignUpEmail;
  String? lastLogInEmail;
  String? lastRefreshToken;
  String? lastLogOutRefreshToken;
  String? lastVerifyMfaCode;
  int logOutCalls = 0;
  int enrollMfaCalls = 0;

  @override
  Future<AuthSession> signUp({
    required String email,
    required String password,
    required String displayName,
  }) async {
    lastSignUpEmail = email;
    if (throwOnSignUp != null) throw throwOnSignUp!;
    return signUpResult!;
  }

  @override
  Future<AuthSession> logIn({
    required String email,
    required String password,
  }) async {
    lastLogInEmail = email;
    if (throwOnLogIn != null) throw throwOnLogIn!;
    return logInResult!;
  }

  @override
  Future<AuthTokens> refresh({required String refreshToken}) async {
    lastRefreshToken = refreshToken;
    if (throwOnRefresh != null) throw throwOnRefresh!;
    return refreshResult!;
  }

  @override
  Future<AuthUser> getCurrentUser() async {
    if (throwOnGetCurrentUser != null) throw throwOnGetCurrentUser!;
    return currentUserResult!;
  }

  @override
  Future<void> logOut({required String refreshToken}) async {
    logOutCalls++;
    lastLogOutRefreshToken = refreshToken;
    if (throwOnLogOut != null) throw throwOnLogOut!;
  }

  @override
  Future<MfaEnrollment> enrollMfa() async {
    enrollMfaCalls++;
    if (throwOnEnrollMfa != null) throw throwOnEnrollMfa!;
    return enrollMfaResult!;
  }

  @override
  Future<void> verifyMfa({required String code}) async {
    lastVerifyMfaCode = code;
    if (throwOnVerifyMfa != null) throw throwOnVerifyMfa!;
  }
}
