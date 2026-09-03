import 'dart:async';

import 'package:riverpod/riverpod.dart';

import 'entities/auth_exception.dart';
import 'entities/auth_user.dart';
import 'repositories/auth_repository.dart';
import 'repositories/token_storage.dart';
import 'usecases/log_in_use_case.dart';
import 'usecases/log_out_use_case.dart';
import 'usecases/restore_session_use_case.dart';
import 'usecases/sign_up_use_case.dart';

/// Overridden in `app/*` with the real `auth_data` implementation
/// (frontend-flutter.md section 1.3 - `ProviderScope(overrides: [...])` is
/// where `data -> domain` wiring happens).
///
/// DEVIATION FROM SPEC: this file uses hand-written `riverpod` (not
/// `@riverpod` code-gen via `riverpod_generator`/`riverpod_annotation`) for
/// Fatia 1, to avoid requiring a `build_runner` step in the test/CI loop for
/// this MVP slice. The public shape (a `Provider<AuthRepository>` overridden
/// at the app root, an `AsyncNotifierProvider` for session state) is
/// unchanged by that choice and migrating to code-gen later is a mechanical,
/// non-breaking refactor. See frontend/README.md "Desvios da spec".
final authRepositoryProvider = Provider<AuthRepository>((ref) {
  throw UnimplementedError(
    'authRepositoryProvider must be overridden by app/* with an auth_data implementation.',
  );
});

final tokenStorageProvider = Provider<TokenStorage>((ref) {
  throw UnimplementedError(
    'tokenStorageProvider must be overridden by app/* with an auth_data implementation.',
  );
});

final signUpUseCaseProvider = Provider(
  (ref) => SignUpUseCase(ref.watch(authRepositoryProvider), ref.watch(tokenStorageProvider)),
);

final logInUseCaseProvider = Provider(
  (ref) => LogInUseCase(ref.watch(authRepositoryProvider), ref.watch(tokenStorageProvider)),
);

final logOutUseCaseProvider = Provider(
  (ref) => LogOutUseCase(ref.watch(authRepositoryProvider), ref.watch(tokenStorageProvider)),
);

final restoreSessionUseCaseProvider = Provider(
  (ref) => RestoreSessionUseCase(ref.watch(authRepositoryProvider), ref.watch(tokenStorageProvider)),
);

/// Session state machine per task scope item 3 ("Riverpod para o estado de
/// sessão"). `null` data means "signed out"; an error state means the last
/// sign-in/sign-up attempt failed (surfaced by `auth_ui` as a form error,
/// not a full-screen error).
class AuthSessionNotifier extends AsyncNotifier<AuthUser?> {
  @override
  FutureOr<AuthUser?> build() async {
    final restore = ref.watch(restoreSessionUseCaseProvider);
    final session = await restore();
    return session?.user;
  }

  Future<void> signUp({
    required String email,
    required String password,
    required String displayName,
  }) async {
    state = const AsyncLoading<AuthUser?>().copyWithPrevious(state);
    state = await AsyncValue.guard(() async {
      final useCase = ref.read(signUpUseCaseProvider);
      final session = await useCase(
        email: email,
        password: password,
        displayName: displayName,
      );
      return session.user;
    });
  }

  Future<void> logIn({required String email, required String password}) async {
    state = const AsyncLoading<AuthUser?>().copyWithPrevious(state);
    state = await AsyncValue.guard(() async {
      final useCase = ref.read(logInUseCaseProvider);
      final session = await useCase(email: email, password: password);
      return session.user;
    });
  }

  Future<void> logOut() async {
    state = const AsyncLoading<AuthUser?>().copyWithPrevious(state);
    state = await AsyncValue.guard(() async {
      final useCase = ref.read(logOutUseCaseProvider);
      await useCase();
      return null;
    });
  }
}

final authSessionProvider =
    AsyncNotifierProvider<AuthSessionNotifier, AuthUser?>(AuthSessionNotifier.new);

/// Convenience re-export so `presentation/auth_ui` doesn't need to import
/// `entities/auth_exception.dart` separately when reacting to
/// `authSessionProvider`'s error state.
typedef AuthFailure = AuthException;
