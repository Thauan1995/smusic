import 'package:auth_domain/auth_domain.dart';
import 'package:auth_ui/auth_ui.dart';
import 'package:core_design_system/core_design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:library_domain/library_domain.dart';
import 'package:library_ui/library_ui.dart';
import 'package:player_domain/player_domain.dart';
import 'package:player_ui/player_ui.dart';
import 'package:shared_navigation/shared_navigation.dart';
import 'package:social_proximity_domain/social_proximity_domain.dart';
import 'package:social_proximity_ui/social_proximity_ui.dart';

import '../support/fakes.dart';
import '../support/proximity_fakes.dart';

/// Wraps the app in an [UncontrolledProviderScope] over a
/// caller-owned [ProviderContainer] (rather than `ProviderScope(overrides:
/// ...)`), so `isAuthenticated` can read the container's *live*
/// `authSessionProvider` state - a fixed closure (`() => false` forever)
/// would fight the explicit `context.go('/library')` navigation
/// `LoginScreen.onLoggedIn` performs on a real successful login, since
/// `redirect` re-evaluates on every navigation attempt.
Widget _app(
  ProviderContainer container, {
  void Function(BuildContext, Playlist)? onPlaylistTap,
  void Function(BuildContext, SearchResultItem)? onResultTap,
}) {
  final router = buildAppRouter(
    isAuthenticated: () => container.read(authSessionProvider).value != null,
    refreshListenable: GoRouterRefreshListenable.fromContainer(container),
    onPlaylistTap: onPlaylistTap,
    onResultTap: onResultTap,
  );
  return UncontrolledProviderScope(
    container: container,
    child: MaterialApp.router(theme: SmusicTheme.light(), routerConfig: router),
  );
}

ProviderContainer _container({
  required FakeAuthRepository authRepository,
  required FakeTokenStorage tokenStorage,
  required FakeLibraryRepository libraryRepository,
  required FakePlaybackQueueController playbackController,
  FakeProximityPrivacySettingsRepository? proximitySettingsRepository,
  FakeLocationProvider? locationProvider,
}) {
  return ProviderContainer(
    overrides: [
      authRepositoryProvider.overrideWithValue(authRepository),
      tokenStorageProvider.overrideWithValue(tokenStorage),
      libraryRepositoryProvider.overrideWithValue(libraryRepository),
      playbackQueueControllerProvider.overrideWithValue(playbackController),
      // Proximity route tree (task scope item 5): real repositories aren't
      // used here (`social_proximity_data` isn't a shared_navigation
      // dependency, by design - `presentation/*` never depends on `data/*`,
      // frontend-flutter.md section 1.2), so tests that only care about
      // routing/auth get inert-but-non-throwing fakes; tests that exercise
      // the `/nearby` flow itself pass configured fakes.
      proximityPrivacySettingsRepositoryProvider
          .overrideWithValue(proximitySettingsRepository ?? FakeProximityPrivacySettingsRepository()),
      proximityFeedRepositoryProvider.overrideWithValue(FakeProximityFeedRepository()),
      locationProviderProvider.overrideWithValue(locationProvider ?? FakeLocationProvider()),
    ],
  );
}

void main() {
  late FakeAuthRepository authRepository;
  late FakeTokenStorage tokenStorage;
  late FakeLibraryRepository libraryRepository;
  late FakePlaybackQueueController playbackController;
  late ProviderContainer container;

  setUp(() {
    authRepository = FakeAuthRepository();
    tokenStorage = FakeTokenStorage();
    libraryRepository = FakeLibraryRepository();
    playbackController = FakePlaybackQueueController();
    container = _container(
      authRepository: authRepository,
      tokenStorage: tokenStorage,
      libraryRepository: libraryRepository,
      playbackController: playbackController,
    );
  });

  tearDown(() async {
    await playbackController.dispose();
    container.dispose();
  });

  testWidgets('unauthenticated users are redirected to /login', (tester) async {
    await tester.pumpWidget(_app(container));
    await tester.pumpAndSettle();

    expect(find.byType(LoginScreen), findsOneWidget);
  });

  testWidgets('a previously-signed-in user lands on the library shell on startup', (tester) async {
    tokenStorage.stored = AuthTokens(
      accessToken: 'a',
      refreshToken: 'r',
      accessTokenExpiresAt: DateTime.now().add(const Duration(hours: 1)),
    );
    authRepository.currentUserResult =
        const AuthUser(userId: '1', displayName: 'Ana', email: 'a@b.com');

    await tester.pumpWidget(_app(container));
    await tester.pumpAndSettle();

    expect(find.byType(LibraryScreen), findsOneWidget);
  });

  testWidgets('the sign-up link navigates to SignUpScreen', (tester) async {
    await tester.pumpWidget(_app(container));
    await tester.pumpAndSettle();

    await tester.tap(find.text("Don't have an account? Sign up"));
    await tester.pumpAndSettle();

    expect(find.byType(SignUpScreen), findsOneWidget);
  });

  testWidgets('the login link from SignUpScreen navigates back to LoginScreen', (tester) async {
    await tester.pumpWidget(_app(container));
    await tester.pumpAndSettle();
    await tester.tap(find.text("Don't have an account? Sign up"));
    await tester.pumpAndSettle();

    await tester.tap(find.text('Already have an account? Log in'));
    await tester.pumpAndSettle();

    expect(find.byType(LoginScreen), findsOneWidget);
  });

  testWidgets('signing up navigates to the library shell', (tester) async {
    authRepository.signUpResult = AuthSession(
      user: const AuthUser(userId: '1', displayName: 'Ana', email: 'a@b.com'),
      tokens: AuthTokens(
        accessToken: 'a',
        refreshToken: 'r',
        accessTokenExpiresAt: DateTime.now().add(const Duration(hours: 1)),
      ),
    );
    await tester.pumpWidget(_app(container));
    await tester.pumpAndSettle();
    await tester.tap(find.text("Don't have an account? Sign up"));
    await tester.pumpAndSettle();

    await tester.enterText(find.byKey(const Key('signup_display_name_field')), 'Ana');
    await tester.enterText(find.byKey(const Key('signup_email_field')), 'a@b.com');
    await tester.enterText(find.byKey(const Key('signup_password_field')), 'password1');
    await tester.tap(find.text('Sign up'));
    await tester.pumpAndSettle();

    expect(find.byType(LibraryScreen), findsOneWidget);
  });

  testWidgets('onPlaylistTap/onResultTap are wired through to the shell screens', (tester) async {
    authRepository.logInResult = AuthSession(
      user: const AuthUser(userId: '1', displayName: 'Ana', email: 'a@b.com'),
      tokens: AuthTokens(
        accessToken: 'a',
        refreshToken: 'r',
        accessTokenExpiresAt: DateTime.now().add(const Duration(hours: 1)),
      ),
    );
    libraryRepository.playlists = [const Playlist(id: 'p1', name: 'Chill', isPublic: false)];
    Playlist? tappedPlaylist;

    await tester.pumpWidget(_app(
      container,
      onPlaylistTap: (context, playlist) => tappedPlaylist = playlist,
    ));
    await tester.pumpAndSettle();
    await tester.enterText(find.byKey(const Key('login_email_field')), 'a@b.com');
    await tester.enterText(find.byKey(const Key('login_password_field')), 'password1');
    await tester.tap(find.text('Log in'));
    await tester.pumpAndSettle();

    await tester.tap(find.text('Chill'));
    expect(tappedPlaylist?.name, 'Chill');
  });

  testWidgets('onResultTap is wired through to SearchScreen', (tester) async {
    authRepository.logInResult = AuthSession(
      user: const AuthUser(userId: '1', displayName: 'Ana', email: 'a@b.com'),
      tokens: AuthTokens(
        accessToken: 'a',
        refreshToken: 'r',
        accessTokenExpiresAt: DateTime.now().add(const Duration(hours: 1)),
      ),
    );
    libraryRepository.searchResult = const Paginated(
      items: [
        SearchResultItem(id: 't1', type: SearchResultType.track, title: 'Song', subtitle: 'Artist'),
      ],
      nextCursor: null,
    );
    SearchResultItem? tappedResult;

    await tester.pumpWidget(_app(
      container,
      onResultTap: (context, item) => tappedResult = item,
    ));
    await tester.pumpAndSettle();
    await tester.enterText(find.byKey(const Key('login_email_field')), 'a@b.com');
    await tester.enterText(find.byKey(const Key('login_password_field')), 'password1');
    await tester.tap(find.text('Log in'));
    await tester.pumpAndSettle();

    await tester.tap(find.text('Search'));
    await tester.pumpAndSettle();
    await tester.enterText(find.byKey(const Key('search_field')), 'song');
    await tester.pump(const Duration(milliseconds: 350));
    await tester.pumpAndSettle();

    await tester.tap(find.text('Song'));
    expect(tappedResult?.id, 't1');
  });

  testWidgets('logging in navigates to the library shell', (tester) async {
    authRepository.logInResult = AuthSession(
      user: const AuthUser(userId: '1', displayName: 'Ana', email: 'a@b.com'),
      tokens: AuthTokens(
        accessToken: 'a',
        refreshToken: 'r',
        accessTokenExpiresAt: DateTime.now().add(const Duration(hours: 1)),
      ),
    );
    await tester.pumpWidget(_app(container));
    await tester.pumpAndSettle();

    await tester.enterText(find.byKey(const Key('login_email_field')), 'a@b.com');
    await tester.enterText(find.byKey(const Key('login_password_field')), 'password1');
    await tester.tap(find.text('Log in'));
    await tester.pumpAndSettle();

    expect(find.byType(LibraryScreen), findsOneWidget);
  });

  testWidgets('switching to the Search destination shows SearchScreen', (tester) async {
    authRepository.logInResult = AuthSession(
      user: const AuthUser(userId: '1', displayName: 'Ana', email: 'a@b.com'),
      tokens: AuthTokens(
        accessToken: 'a',
        refreshToken: 'r',
        accessTokenExpiresAt: DateTime.now().add(const Duration(hours: 1)),
      ),
    );
    await tester.pumpWidget(_app(container));
    await tester.pumpAndSettle();
    await tester.enterText(find.byKey(const Key('login_email_field')), 'a@b.com');
    await tester.enterText(find.byKey(const Key('login_password_field')), 'password1');
    await tester.tap(find.text('Log in'));
    await tester.pumpAndSettle();

    await tester.tap(find.text('Search'));
    await tester.pumpAndSettle();

    expect(find.byType(SearchScreen), findsOneWidget);
  });

  testWidgets('pushing /player renders PlayerScreen', (tester) async {
    authRepository.logInResult = AuthSession(
      user: const AuthUser(userId: '1', displayName: 'Ana', email: 'a@b.com'),
      tokens: AuthTokens(
        accessToken: 'a',
        refreshToken: 'r',
        accessTokenExpiresAt: DateTime.now().add(const Duration(hours: 1)),
      ),
    );
    await tester.pumpWidget(_app(container));
    await tester.pumpAndSettle();
    await tester.enterText(find.byKey(const Key('login_email_field')), 'a@b.com');
    await tester.enterText(find.byKey(const Key('login_password_field')), 'password1');
    await tester.tap(find.text('Log in'));
    await tester.pumpAndSettle();

    final context = tester.element(find.byType(NavigationShell));
    context.push('/player');
    // Not pumpAndSettle(): PlayerScreen shows an indefinitely-animating
    // CircularProgressIndicator while playerStateProvider has no emitted
    // state yet (this fake controller never emits one), which would never
    // let pumpAndSettle() converge.
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));

    expect(find.byType(PlayerScreen), findsOneWidget);
  });

  group('the /nearby route tree (task scope item 5)', () {
    Future<void> logIn(WidgetTester tester) async {
      authRepository.logInResult = AuthSession(
        user: const AuthUser(userId: '1', displayName: 'Ana', email: 'a@b.com'),
        tokens: AuthTokens(
          accessToken: 'a',
          refreshToken: 'r',
          accessTokenExpiresAt: DateTime.now().add(const Duration(hours: 1)),
        ),
      );
      await tester.pumpWidget(_app(container));
      await tester.pumpAndSettle();
      await tester.enterText(find.byKey(const Key('login_email_field')), 'a@b.com');
      await tester.enterText(find.byKey(const Key('login_password_field')), 'password1');
      await tester.tap(find.text('Log in'));
      await tester.pumpAndSettle();
    }

    testWidgets('the Perto destination navigates to the proximity value screen', (tester) async {
      await logIn(tester);

      await tester.tap(find.text('Perto'));
      await tester.pumpAndSettle();

      expect(find.byType(ProximityListScreen), findsOneWidget);
      expect(find.text('Veja quem está ouvindo música perto de você'), findsOneWidget);
    });

    testWidgets('opting in pushes to the permission gate, then the settings button reaches '
        '/nearby/settings', (tester) async {
      await logIn(tester);
      await tester.tap(find.text('Perto'));
      await tester.pumpAndSettle();

      await tester.ensureVisible(find.text('Ativar descoberta por proximidade'));
      await tester.tap(find.text('Ativar descoberta por proximidade'));
      await tester.pumpAndSettle();

      // Past the value screen (enabled: true now); OS location permission
      // was never granted by the FakeLocationProvider default, so the
      // permission gate renders next - proving `onOptedIn`
      // (`app_router.dart`'s `context.push('/nearby/settings')` closure's
      // sibling `request()` call) actually ran, not just that the settings
      // repository was mutated.
      expect(find.text('Permitir localização'), findsOneWidget);
    });

    testWidgets('the settings button in ProximityListScreen pushes /nearby/settings', (tester) async {
      container.dispose();
      container = _container(
        authRepository: authRepository,
        tokenStorage: tokenStorage,
        libraryRepository: libraryRepository,
        playbackController: playbackController,
        proximitySettingsRepository: FakeProximityPrivacySettingsRepository(
          initial: ProximityPrivacySettings.disabled().copyWith(enabled: true, paused: false),
        ),
        locationProvider: FakeLocationProvider()..permissionStatus = LocationPermissionState.granted,
      );
      await logIn(tester);

      await tester.tap(find.text('Perto'));
      await tester.pumpAndSettle();

      await tester.tap(find.byKey(const Key('proximity_open_settings_button')));
      await tester.pumpAndSettle();

      expect(find.byType(ProximityPrivacySettingsScreen), findsOneWidget);
    });
  });
}
