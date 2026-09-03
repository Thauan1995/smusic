import 'package:auth_domain/auth_domain.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:shared_navigation/shared_navigation.dart';

/// Builds the shared `go_router` instance once per `ProviderScope`, wiring
/// [GoRouterRefreshListenable] to `authSessionProvider` via `ref.listen`
/// (the idiomatic place for a provider-body Riverpod subscription - see
/// `GoRouterRefreshListenable`'s doc comment for why the listenable itself
/// stays decoupled from `Ref`/`ProviderContainer`).
final appRouterProvider = Provider((ref) {
  final refreshListenable = GoRouterRefreshListenable();
  ref.listen(authSessionProvider, (previous, next) => refreshListenable.notify());
  ref.onDispose(refreshListenable.dispose);

  return buildAppRouter(
    isAuthenticated: () => ref.read(authSessionProvider).value != null,
    refreshListenable: refreshListenable,
  );
});
