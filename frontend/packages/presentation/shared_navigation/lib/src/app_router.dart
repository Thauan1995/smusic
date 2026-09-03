import 'package:auth_ui/auth_ui.dart';
import 'package:flutter/widgets.dart';
import 'package:go_router/go_router.dart';
import 'package:library_domain/library_domain.dart';
import 'package:library_ui/library_ui.dart';
import 'package:player_ui/player_ui.dart';

import 'navigation_shell.dart';

/// The single `go_router` route tree shared verbatim by `smusic_mobile` and
/// `smusic_web` (frontend-flutter.md section 3.5/1.3).
///
/// [isAuthenticated] is a synchronous snapshot check (`ref.read`, not
/// `ref.watch`) - `redirect` itself only re-evaluates on navigation
/// attempts or when [refreshListenable] fires, so it acts as a *guard*
/// against deep-linking into protected routes while signed out.
/// [refreshListenable] (typically a `GoRouterRefreshListenable`) is what
/// makes the cold-start-with-existing-session case work: pass one built
/// from the same `ProviderContainer`/`authSessionProvider` used for
/// [isAuthenticated], or startup redirect can get stuck on `/login` while
/// the async session restore is still in flight (see
/// `GoRouterRefreshListenable`'s doc comment). The interactive sign-in/up
/// transition is additionally driven explicitly by `LoginScreen`/
/// `SignUpScreen`'s `onLoggedIn`/`onSignedUp` callbacks below - see
/// auth_ui's `LoginScreen` doc comment for why those screens never
/// navigate themselves.
///
/// Track/album detail routes and a "playlist detail" screen are out of
/// Fatia 1 scope - [onPlaylistTap]/[onResultTap] are no-ops by default,
/// overridable by the caller for when that scope lands.
GoRouter buildAppRouter({
  required bool Function() isAuthenticated,
  Listenable? refreshListenable,
  void Function(BuildContext context, Playlist playlist)? onPlaylistTap,
  void Function(BuildContext context, SearchResultItem searchResult)? onResultTap,
}) {
  return GoRouter(
    initialLocation: '/library',
    refreshListenable: refreshListenable,
    redirect: (context, state) {
      final loggingIn =
          state.matchedLocation == '/login' || state.matchedLocation == '/signup';
      final authed = isAuthenticated();
      if (!authed && !loggingIn) return '/login';
      if (authed && loggingIn) return '/library';
      return null;
    },
    routes: [
      GoRoute(
        path: '/login',
        builder: (context, state) => LoginScreen(
          onLoggedIn: () => context.go('/library'),
          onNavigateToSignUp: () => context.go('/signup'),
        ),
      ),
      GoRoute(
        path: '/signup',
        builder: (context, state) => SignUpScreen(
          onSignedUp: () => context.go('/library'),
          onNavigateToLogin: () => context.go('/login'),
        ),
      ),
      ShellRoute(
        builder: (context, state, child) => NavigationShell(
          currentLocation: state.matchedLocation,
          child: child,
        ),
        routes: [
          GoRoute(
            path: '/library',
            builder: (context, state) => LibraryScreen(
              onPlaylistTap: onPlaylistTap == null
                  ? null
                  : (playlist) => onPlaylistTap(context, playlist),
            ),
          ),
          GoRoute(
            path: '/search',
            builder: (context, state) => SearchScreen(
              onResultTap: onResultTap == null
                  ? null
                  : (item) => onResultTap(context, item),
            ),
          ),
        ],
      ),
      GoRoute(
        path: '/player',
        builder: (context, state) => PlayerScreen(onClose: () => context.pop()),
      ),
    ],
  );
}
