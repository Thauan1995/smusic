import 'package:auth_domain/auth_domain.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:smusic_app_shared/smusic_app_shared.dart';

import '../support/fakes.dart';

void main() {
  test('builds a GoRouter and disposes its refresh listenable cleanly', () async {
    final container = ProviderContainer(
      overrides: [
        authRepositoryProvider.overrideWithValue(FakeAuthRepository()),
        tokenStorageProvider.overrideWithValue(FakeTokenStorage()),
      ],
    );

    final router = container.read(appRouterProvider);
    expect(router, isA<GoRouter>());

    // Let the initial authSessionProvider.build() (and thus the ref.listen
    // wired inside appRouterProvider) settle before disposing.
    await container.read(authSessionProvider.future);

    container.dispose(); // must not throw
  });
}
