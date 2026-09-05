import 'package:auth_domain/auth_domain.dart';

class FakeAuthRepository implements AuthRepository {
  AuthSession? signUpResult;
  AuthSession? logInResult;
  Object? throwOnSignUp;
  Object? throwOnLogIn;
  MfaEnrollment? enrollMfaResult;
  Object? throwOnEnrollMfa;
  Object? throwOnVerifyMfa;
  String? lastVerifyMfaCode;

  @override
  Future<AuthSession> signUp({
    required String email,
    required String password,
    required String displayName,
  }) async {
    // A real (Timer-based) delay, not just a microtask hop: without it, the
    // whole loading -> data transition would resolve within a single
    // `tester.pump()`, making the screen's loading spinner impossible to
    // observe in a widget test.
    await Future<void>.delayed(const Duration(milliseconds: 10));
    if (throwOnSignUp != null) throw throwOnSignUp!;
    return signUpResult!;
  }

  @override
  Future<AuthSession> logIn({required String email, required String password}) async {
    await Future<void>.delayed(const Duration(milliseconds: 10));
    if (throwOnLogIn != null) throw throwOnLogIn!;
    return logInResult!;
  }

  @override
  Future<AuthTokens> refresh({required String refreshToken}) => throw UnimplementedError();

  @override
  Future<AuthUser> getCurrentUser() => throw UnimplementedError();

  @override
  Future<void> logOut({required String refreshToken}) async {}

  @override
  Future<MfaEnrollment> enrollMfa() async {
    await Future<void>.delayed(const Duration(milliseconds: 10));
    if (throwOnEnrollMfa != null) throw throwOnEnrollMfa!;
    return enrollMfaResult!;
  }

  @override
  Future<void> verifyMfa({required String code}) async {
    await Future<void>.delayed(const Duration(milliseconds: 10));
    lastVerifyMfaCode = code;
    if (throwOnVerifyMfa != null) throw throwOnVerifyMfa!;
  }
}

class FakeTokenStorage implements TokenStorage {
  AuthTokens? stored;

  @override
  Future<AuthTokens?> read() async => stored;

  @override
  Future<void> save(AuthTokens tokens) async => stored = tokens;

  @override
  Future<void> clear() async => stored = null;
}
