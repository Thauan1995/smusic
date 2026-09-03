import 'package:auth_domain/auth_domain.dart';
import 'package:test/test.dart';

import '../../support/fake_auth_repository.dart';
import '../../support/fake_token_storage.dart';

void main() {
  test('logs in, persists tokens, returns session', () async {
    final repository = FakeAuthRepository();
    final tokenStorage = FakeTokenStorage();
    final user = AuthUser(userId: '1', displayName: 'Ana', email: 'a@b.com');
    final tokens = AuthTokens(
      accessToken: 'a',
      refreshToken: 'r',
      accessTokenExpiresAt: DateTime.now().add(const Duration(hours: 1)),
    );
    repository.logInResult = AuthSession(user: user, tokens: tokens);

    final useCase = LogInUseCase(repository, tokenStorage);
    final session = await useCase(email: 'a@b.com', password: 'p');

    expect(session.user, user);
    expect(repository.lastLogInEmail, 'a@b.com');
    expect(tokenStorage.stored, tokens);
  });

  test('propagates invalid credentials failure', () async {
    final repository = FakeAuthRepository()
      ..throwOnLogIn = const AuthException(AuthExceptionKind.invalidCredentials);
    final tokenStorage = FakeTokenStorage();
    final useCase = LogInUseCase(repository, tokenStorage);

    await expectLater(
      () => useCase(email: 'a@b.com', password: 'wrong'),
      throwsA(isA<AuthException>()),
    );
    expect(tokenStorage.saveCalls, 0);
  });
}
