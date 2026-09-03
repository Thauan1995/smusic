import 'package:auth_domain/auth_domain.dart';
import 'package:test/test.dart';

import '../../support/fake_auth_repository.dart';
import '../../support/fake_token_storage.dart';

void main() {
  test('returns null when nothing is stored', () async {
    final repository = FakeAuthRepository();
    final tokenStorage = FakeTokenStorage();
    final useCase = RestoreSessionUseCase(repository, tokenStorage);

    expect(await useCase(), isNull);
  });

  test('returns session using stored (non-expired) tokens', () async {
    final repository = FakeAuthRepository()
      ..currentUserResult = AuthUser(userId: '1', displayName: 'Ana', email: 'a@b.com');
    final tokens = AuthTokens(
      accessToken: 'a',
      refreshToken: 'r',
      accessTokenExpiresAt: DateTime.now().add(const Duration(hours: 1)),
    );
    final tokenStorage = FakeTokenStorage()..stored = tokens;
    final useCase = RestoreSessionUseCase(repository, tokenStorage);

    final session = await useCase();
    expect(session, isNotNull);
    expect(session!.tokens, tokens);
    expect(repository.lastRefreshToken, isNull); // no refresh needed
  });

  test('refreshes expired tokens before fetching the user', () async {
    final expired = AuthTokens(
      accessToken: 'old',
      refreshToken: 'r',
      accessTokenExpiresAt: DateTime.now().subtract(const Duration(hours: 1)),
    );
    final refreshed = AuthTokens(
      accessToken: 'new',
      refreshToken: 'r2',
      accessTokenExpiresAt: DateTime.now().add(const Duration(hours: 1)),
    );
    final repository = FakeAuthRepository()
      ..refreshResult = refreshed
      ..currentUserResult = AuthUser(userId: '1', displayName: 'Ana', email: 'a@b.com');
    final tokenStorage = FakeTokenStorage()..stored = expired;
    final useCase = RestoreSessionUseCase(repository, tokenStorage);

    final session = await useCase();
    expect(session!.tokens, refreshed);
    expect(repository.lastRefreshToken, 'r');
    expect(tokenStorage.stored, refreshed);
  });

  test('clears storage and returns null when refresh fails', () async {
    final expired = AuthTokens(
      accessToken: 'old',
      refreshToken: 'r',
      accessTokenExpiresAt: DateTime.now().subtract(const Duration(hours: 1)),
    );
    final repository = FakeAuthRepository()
      ..throwOnRefresh = const AuthException(AuthExceptionKind.sessionExpired);
    final tokenStorage = FakeTokenStorage()..stored = expired;
    final useCase = RestoreSessionUseCase(repository, tokenStorage);

    expect(await useCase(), isNull);
    expect(tokenStorage.stored, isNull);
    expect(tokenStorage.clearCalls, 1);
  });

  test('clears storage and returns null when getCurrentUser fails', () async {
    final tokens = AuthTokens(
      accessToken: 'a',
      refreshToken: 'r',
      accessTokenExpiresAt: DateTime.now().add(const Duration(hours: 1)),
    );
    final repository = FakeAuthRepository()
      ..throwOnGetCurrentUser = const AuthException(AuthExceptionKind.sessionExpired);
    final tokenStorage = FakeTokenStorage()..stored = tokens;
    final useCase = RestoreSessionUseCase(repository, tokenStorage);

    expect(await useCase(), isNull);
    expect(tokenStorage.stored, isNull);
  });
}
