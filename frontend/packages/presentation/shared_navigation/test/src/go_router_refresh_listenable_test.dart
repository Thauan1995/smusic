import 'package:auth_domain/auth_domain.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_navigation/shared_navigation.dart';

import '../support/fakes.dart';

void main() {
  testWidgets('notifies listeners whenever authSessionProvider changes', (tester) async {
    final authRepository = FakeAuthRepository();
    final tokenStorage = FakeTokenStorage();
    final container = ProviderContainer(
      overrides: [
        authRepositoryProvider.overrideWithValue(authRepository),
        tokenStorageProvider.overrideWithValue(tokenStorage),
      ],
    );
    addTearDown(container.dispose);

    final listenable = GoRouterRefreshListenable.fromContainer(container);
    var notifications = 0;
    listenable.addListener(() => notifications++);

    // The initial build() resolving (signed out, no stored session) is
    // itself a state change from the provider's uninitialized state.
    await container.read(authSessionProvider.future);
    await tester.pump();
    expect(notifications, greaterThan(0));

    final before = notifications;
    authRepository.logInResult = AuthSession(
      user: const AuthUser(userId: '1', displayName: 'Ana', email: 'a@b.com'),
      tokens: AuthTokens(
        accessToken: 'a',
        refreshToken: 'r',
        accessTokenExpiresAt: DateTime.now().add(const Duration(hours: 1)),
      ),
    );
    await container
        .read(authSessionProvider.notifier)
        .logIn(email: 'a@b.com', password: 'x');
    expect(notifications, greaterThan(before));

    listenable.dispose();
  });
}
