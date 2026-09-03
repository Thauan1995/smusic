import 'package:auth_domain/auth_domain.dart';
import 'package:test/test.dart';

import '../../support/fake_auth_repository.dart';
import '../../support/fake_token_storage.dart';

void main() {
  test('signs up, persists tokens, returns session', () async {
    final repository = FakeAuthRepository();
    final tokenStorage = FakeTokenStorage();
    final user = AuthUser(userId: '1', displayName: 'Ana', email: 'a@b.com');
    final tokens = AuthTokens(
      accessToken: 'a',
      refreshToken: 'r',
      accessTokenExpiresAt: DateTime.now().add(const Duration(hours: 1)),
    );
    repository.signUpResult = AuthSession(user: user, tokens: tokens);

    final useCase = SignUpUseCase(repository, tokenStorage);
    final session = await useCase(
      email: 'a@b.com',
      password: 'p',
      displayName: 'Ana',
    );

    expect(session.user, user);
    expect(repository.lastSignUpEmail, 'a@b.com');
    expect(tokenStorage.saveCalls, 1);
    expect(tokenStorage.stored, tokens);
  });

  test('propagates repository failure without saving tokens', () async {
    final repository = FakeAuthRepository()
      ..throwOnSignUp = const AuthException(AuthExceptionKind.emailAlreadyInUse);
    final tokenStorage = FakeTokenStorage();
    final useCase = SignUpUseCase(repository, tokenStorage);

    await expectLater(
      () => useCase(email: 'a@b.com', password: 'p', displayName: 'Ana'),
      throwsA(isA<AuthException>()),
    );
    expect(tokenStorage.saveCalls, 0);
  });
}
