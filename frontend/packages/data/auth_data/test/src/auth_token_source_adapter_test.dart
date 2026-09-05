import 'package:auth_data/auth_data.dart';
import 'package:auth_domain/auth_domain.dart';
import 'package:flutter_test/flutter_test.dart';

class _FakeTokenStorage implements TokenStorage {
  AuthTokens? stored;
  int clearCalls = 0;

  @override
  Future<AuthTokens?> read() async => stored;

  @override
  Future<void> save(AuthTokens tokens) async => stored = tokens;

  @override
  Future<void> clear() async {
    clearCalls++;
    stored = null;
  }
}

class _FakeAuthRepository implements AuthRepository {
  AuthTokens? refreshResult;
  Object? throwOnRefresh;
  String? lastRefreshToken;

  @override
  Future<AuthTokens> refresh({required String refreshToken}) async {
    lastRefreshToken = refreshToken;
    if (throwOnRefresh != null) throw throwOnRefresh!;
    return refreshResult!;
  }

  @override
  Future<AuthSession> signUp({
    required String email,
    required String password,
    required String displayName,
  }) => throw UnimplementedError();

  @override
  Future<AuthSession> logIn({required String email, required String password}) =>
      throw UnimplementedError();

  @override
  Future<AuthUser> getCurrentUser() => throw UnimplementedError();

  @override
  Future<void> logOut({required String refreshToken}) => throw UnimplementedError();

  @override
  Future<MfaEnrollment> enrollMfa() => throw UnimplementedError();

  @override
  Future<void> verifyMfa({required String code}) => throw UnimplementedError();
}

void main() {
  test('currentAccessToken reads from storage', () async {
    final storage = _FakeTokenStorage()
      ..stored = AuthTokens(
        accessToken: 'a',
        refreshToken: 'r',
        accessTokenExpiresAt: DateTime.now(),
      );
    final adapter = AuthTokenSourceAdapter(storage);

    expect(await adapter.currentAccessToken(), 'a');
  });

  test('currentAccessToken returns null when signed out', () async {
    final adapter = AuthTokenSourceAdapter(_FakeTokenStorage());
    expect(await adapter.currentAccessToken(), isNull);
  });

  test('refreshAccessToken throws StateError before attachRepository', () async {
    final adapter = AuthTokenSourceAdapter(_FakeTokenStorage());
    await expectLater(() => adapter.refreshAccessToken(), throwsStateError);
  });

  test('refreshAccessToken returns null when there is nothing stored', () async {
    final adapter = AuthTokenSourceAdapter(_FakeTokenStorage());
    adapter.attachRepository(_FakeAuthRepository());
    expect(await adapter.refreshAccessToken(), isNull);
  });

  test('refreshAccessToken refreshes, saves, and returns the new access token', () async {
    final storage = _FakeTokenStorage()
      ..stored = AuthTokens(
        accessToken: 'old',
        refreshToken: 'r',
        accessTokenExpiresAt: DateTime.now(),
      );
    final newTokens = AuthTokens(
      accessToken: 'new',
      refreshToken: 'r2',
      accessTokenExpiresAt: DateTime.now().add(const Duration(minutes: 15)),
    );
    final repository = _FakeAuthRepository()..refreshResult = newTokens;
    final adapter = AuthTokenSourceAdapter(storage)..attachRepository(repository);

    final result = await adapter.refreshAccessToken();

    expect(result, 'new');
    expect(repository.lastRefreshToken, 'r');
    expect(storage.stored, newTokens);
  });

  test('refreshAccessToken clears storage and returns null on AuthException', () async {
    final storage = _FakeTokenStorage()
      ..stored = AuthTokens(
        accessToken: 'old',
        refreshToken: 'r',
        accessTokenExpiresAt: DateTime.now(),
      );
    final repository = _FakeAuthRepository()
      ..throwOnRefresh = const AuthException(AuthExceptionKind.sessionExpired);
    final adapter = AuthTokenSourceAdapter(storage)..attachRepository(repository);

    final result = await adapter.refreshAccessToken();

    expect(result, isNull);
    expect(storage.clearCalls, 1);
  });
}
