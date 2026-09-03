import 'package:auth_domain/auth_domain.dart';
import 'package:test/test.dart';

import '../../support/fake_auth_repository.dart';
import '../../support/fake_token_storage.dart';

void main() {
  test('revokes server-side session and clears local tokens', () async {
    final repository = FakeAuthRepository();
    final tokenStorage = FakeTokenStorage()
      ..stored = AuthTokens(
        accessToken: 'a',
        refreshToken: 'r',
        accessTokenExpiresAt: DateTime.now(),
      );

    final useCase = LogOutUseCase(repository, tokenStorage);
    await useCase();

    expect(repository.logOutCalls, 1);
    expect(repository.lastLogOutRefreshToken, 'r');
    expect(tokenStorage.clearCalls, 1);
    expect(tokenStorage.stored, isNull);
  });

  test('clears local tokens even when there is nothing stored', () async {
    final repository = FakeAuthRepository();
    final tokenStorage = FakeTokenStorage();

    final useCase = LogOutUseCase(repository, tokenStorage);
    await useCase();

    expect(repository.logOutCalls, 0);
    expect(tokenStorage.clearCalls, 1);
  });

  test('clears local tokens even when server revocation fails', () async {
    final repository = FakeAuthRepository()
      ..throwOnLogOut = const AuthException(AuthExceptionKind.network);
    final tokenStorage = FakeTokenStorage()
      ..stored = AuthTokens(
        accessToken: 'a',
        refreshToken: 'r',
        accessTokenExpiresAt: DateTime.now(),
      );

    final useCase = LogOutUseCase(repository, tokenStorage);
    await useCase();

    expect(repository.logOutCalls, 1);
    expect(tokenStorage.clearCalls, 1);
  });
}
