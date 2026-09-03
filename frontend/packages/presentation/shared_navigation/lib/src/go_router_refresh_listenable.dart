import 'package:auth_domain/auth_domain.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

/// Notifies `go_router` (via its `refreshListenable` constructor param)
/// whenever it's told to - wired up to `authSessionProvider` so `redirect`
/// re-runs once the async session restore on app startup completes.
/// Without this, a returning user with a still-valid stored session would
/// see `redirect` evaluate once (while the restore is still loading,
/// `isAuthenticated()` reads `false`), get sent to `/login`, and then have
/// nothing trigger a second evaluation once the restore actually resolves
/// to signed-in.
///
/// This complements, not replaces, the "screens navigate explicitly on
/// success" design in `LoginScreen`/`SignUpScreen` (see their doc
/// comments): that path handles the interactive sign-in/up transition;
/// this one handles the cold-start-with-existing-session case, which has
/// no explicit "I just signed in" user action to hang navigation off of.
///
/// Deliberately a plain [ChangeNotifier] with just [notify] rather than
/// something that owns its own Riverpod subscription: `app/*`'s
/// `appRouterProvider` wires it up via `ref.listen(authSessionProvider,
/// (_, __) => listenable.notify())` inside a provider body (the idiomatic
/// place for that), so the listenable's own lifecycle doesn't need to know
/// about `ProviderContainer` at all. [fromContainer] is a convenience
/// factory for call sites (tests, or non-provider composition roots) that
/// only have a `ProviderContainer` handle.
class GoRouterRefreshListenable extends ChangeNotifier {
  void notify() => notifyListeners();

  /// Convenience factory that owns a [ProviderContainer] subscription
  /// itself, for callers outside a Riverpod provider body (e.g. tests).
  static GoRouterRefreshListenable fromContainer(ProviderContainer container) {
    final listenable = GoRouterRefreshListenable();
    listenable._ownedSubscription = container.listen(
      authSessionProvider,
      (previous, next) => listenable.notify(),
    );
    return listenable;
  }

  ProviderSubscription<AsyncValue<AuthUser?>>? _ownedSubscription;

  @override
  void dispose() {
    _ownedSubscription?.close();
    super.dispose();
  }
}
