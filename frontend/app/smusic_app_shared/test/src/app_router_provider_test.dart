import 'package:auth_domain/auth_domain.dart';
import 'package:core_design_system/core_design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:library_domain/library_domain.dart';
import 'package:player_domain/player_domain.dart';
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

  testWidgets(
    'tapping a track search result resolves it, starts playback and '
    'navigates to the player screen (real onResultTap wiring)',
    (tester) async {
      final authRepository = FakeAuthRepository()
        ..logInResult = AuthSession(
          user: const AuthUser(userId: '1', displayName: 'Ana', email: 'a@b.com'),
          tokens: AuthTokens(
            accessToken: 'a',
            refreshToken: 'r',
            accessTokenExpiresAt: DateTime.now().add(const Duration(hours: 1)),
          ),
        );
      final libraryRepository = FakeLibraryRepository()
        ..searchResult = const Paginated(
          items: [
            SearchResultItem(
              id: 't1',
              type: SearchResultType.track,
              title: 'Song',
              subtitle: 'Artist',
            ),
          ],
          nextCursor: null,
        )
        ..trackResult = const Track(
          id: 't1',
          title: 'Song',
          artistName: 'Artist',
          albumName: 'Album',
          durationMs: 210000,
        );
      final playbackController = FakePlaybackQueueController();

      final container = ProviderContainer(
        overrides: [
          authRepositoryProvider.overrideWithValue(authRepository),
          tokenStorageProvider.overrideWithValue(FakeTokenStorage()),
          libraryRepositoryProvider.overrideWithValue(libraryRepository),
          playbackQueueControllerProvider.overrideWithValue(playbackController),
        ],
      );
      addTearDown(container.dispose);

      final router = container.read(appRouterProvider);
      await tester.pumpWidget(
        UncontrolledProviderScope(
          container: container,
          child: MaterialApp.router(theme: SmusicTheme.light(), routerConfig: router),
        ),
      );
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

      // Not pumpAndSettle: PlayerScreen shows an indeterminate
      // CircularProgressIndicator while playerStateProvider is loading
      // (the fake controller's stateStream never emits), which animates
      // forever and would make pumpAndSettle time out.
      await tester.tap(find.text('Song'));
      await tester.pump();
      await tester.pump();

      // The real onResultTap wired in appRouterProvider (not a test fake)
      // resolved the full Track, started playback with it, and navigated
      // to the expanded player screen.
      expect(playbackController.lastQueue, hasLength(1));
      expect(playbackController.lastQueue.single.trackId, 't1');
      expect(playbackController.lastQueue.single.durationMs, 210000);
      expect(playbackController.lastStartIndex, 0);
      expect(find.text('Now Playing'), findsOneWidget);
    },
  );

  testWidgets(
    'tapping a non-track search result (e.g. an album) is a no-op',
    (tester) async {
      final authRepository = FakeAuthRepository()
        ..logInResult = AuthSession(
          user: const AuthUser(userId: '1', displayName: 'Ana', email: 'a@b.com'),
          tokens: AuthTokens(
            accessToken: 'a',
            refreshToken: 'r',
            accessTokenExpiresAt: DateTime.now().add(const Duration(hours: 1)),
          ),
        );
      final libraryRepository = FakeLibraryRepository()
        ..searchResult = const Paginated(
          items: [
            SearchResultItem(
              id: 'al1',
              type: SearchResultType.album,
              title: 'Greatest Hits',
              subtitle: 'Album',
            ),
          ],
          nextCursor: null,
        );
      final playbackController = FakePlaybackQueueController();

      final container = ProviderContainer(
        overrides: [
          authRepositoryProvider.overrideWithValue(authRepository),
          tokenStorageProvider.overrideWithValue(FakeTokenStorage()),
          libraryRepositoryProvider.overrideWithValue(libraryRepository),
          playbackQueueControllerProvider.overrideWithValue(playbackController),
        ],
      );
      addTearDown(container.dispose);

      final router = container.read(appRouterProvider);
      await tester.pumpWidget(
        UncontrolledProviderScope(
          container: container,
          child: MaterialApp.router(theme: SmusicTheme.light(), routerConfig: router),
        ),
      );
      await tester.pumpAndSettle();

      await tester.enterText(find.byKey(const Key('login_email_field')), 'a@b.com');
      await tester.enterText(find.byKey(const Key('login_password_field')), 'password1');
      await tester.tap(find.text('Log in'));
      await tester.pumpAndSettle();

      await tester.tap(find.text('Search'));
      await tester.pumpAndSettle();
      await tester.enterText(find.byKey(const Key('search_field')), 'hits');
      await tester.pump(const Duration(milliseconds: 350));
      await tester.pumpAndSettle();

      await tester.tap(find.text('Greatest Hits'));
      await tester.pumpAndSettle();

      expect(playbackController.lastQueue, isEmpty);
      expect(find.text('Now Playing'), findsNothing);
    },
  );
}
