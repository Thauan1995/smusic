import 'package:auth_domain/auth_domain.dart';
import 'package:riverpod/riverpod.dart';
import 'package:test/test.dart';

import '../support/fake_auth_repository.dart';
import '../support/fake_token_storage.dart';

void main() {
  group('default providers (not overridden)', () {
    test('authRepositoryProvider throws UnimplementedError', () {
      final container = ProviderContainer();
      addTearDown(container.dispose);
      expect(
        () => container.read(authRepositoryProvider),
        throwsUnimplementedError,
      );
    });

    test('tokenStorageProvider throws UnimplementedError', () {
      final container = ProviderContainer();
      addTearDown(container.dispose);
      expect(
        () => container.read(tokenStorageProvider),
        throwsUnimplementedError,
      );
    });
  });

  group('AuthSessionNotifier', () {
    late FakeAuthRepository repository;
    late FakeTokenStorage tokenStorage;
    late ProviderContainer container;

    setUp(() {
      repository = FakeAuthRepository();
      tokenStorage = FakeTokenStorage();
      container = ProviderContainer(
        overrides: [
          authRepositoryProvider.overrideWithValue(repository),
          tokenStorageProvider.overrideWithValue(tokenStorage),
        ],
      );
      addTearDown(container.dispose);
    });

    test('builds to null (signed out) when no session is stored', () async {
      final session = await container.read(authSessionProvider.future);
      expect(session, isNull);
    });

    test('restores a signed-in session on build when tokens are stored', () async {
      final user = AuthUser(userId: '1', displayName: 'Ana', email: 'a@b.com');
      tokenStorage.stored = AuthTokens(
        accessToken: 'a',
        refreshToken: 'r',
        accessTokenExpiresAt: DateTime.now().add(const Duration(hours: 1)),
      );
      repository.currentUserResult = user;

      final session = await container.read(authSessionProvider.future);
      expect(session, user);
    });

    test('signUp transitions loading -> data(user)', () async {
      await container.read(authSessionProvider.future); // settle initial build

      final user = AuthUser(userId: '1', displayName: 'Ana', email: 'a@b.com');
      repository.signUpResult = AuthSession(
        user: user,
        tokens: AuthTokens(
          accessToken: 'a',
          refreshToken: 'r',
          accessTokenExpiresAt: DateTime.now().add(const Duration(hours: 1)),
        ),
      );

      final notifier = container.read(authSessionProvider.notifier);
      await notifier.signUp(email: 'a@b.com', password: 'p', displayName: 'Ana');

      expect(container.read(authSessionProvider).value, user);
    });

    test('logIn transitions loading -> data(user)', () async {
      await container.read(authSessionProvider.future);

      final user = AuthUser(userId: '1', displayName: 'Ana', email: 'a@b.com');
      repository.logInResult = AuthSession(
        user: user,
        tokens: AuthTokens(
          accessToken: 'a',
          refreshToken: 'r',
          accessTokenExpiresAt: DateTime.now().add(const Duration(hours: 1)),
        ),
      );

      final notifier = container.read(authSessionProvider.notifier);
      await notifier.logIn(email: 'a@b.com', password: 'p');

      expect(container.read(authSessionProvider).value, user);
    });

    test('logIn surfaces AuthException through AsyncError', () async {
      await container.read(authSessionProvider.future);
      repository.throwOnLogIn = const AuthException(AuthExceptionKind.invalidCredentials);

      final notifier = container.read(authSessionProvider.notifier);
      await notifier.logIn(email: 'a@b.com', password: 'wrong');

      final state = container.read(authSessionProvider);
      expect(state.hasError, isTrue);
      expect(state.error, isA<AuthException>());
    });

    test('logOut transitions back to data(null)', () async {
      final user = AuthUser(userId: '1', displayName: 'Ana', email: 'a@b.com');
      tokenStorage.stored = AuthTokens(
        accessToken: 'a',
        refreshToken: 'r',
        accessTokenExpiresAt: DateTime.now().add(const Duration(hours: 1)),
      );
      repository.currentUserResult = user;
      await container.read(authSessionProvider.future);

      final notifier = container.read(authSessionProvider.notifier);
      await notifier.logOut();

      expect(container.read(authSessionProvider).value, isNull);
    });

    test('use case providers are wired through the same overrides', () {
      expect(container.read(signUpUseCaseProvider), isA<SignUpUseCase>());
      expect(container.read(logInUseCaseProvider), isA<LogInUseCase>());
      expect(container.read(logOutUseCaseProvider), isA<LogOutUseCase>());
      expect(
        container.read(restoreSessionUseCaseProvider),
        isA<RestoreSessionUseCase>(),
      );
    });
  });
}
