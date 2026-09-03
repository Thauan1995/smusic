import 'package:auth_domain/auth_domain.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:library_domain/library_domain.dart';
import 'package:player_domain/player_domain.dart';
import 'package:shared_navigation/shared_navigation.dart';

/// Builds the shared `go_router` instance once per `ProviderScope`, wiring
/// [GoRouterRefreshListenable] to `authSessionProvider` via `ref.listen`
/// (the idiomatic place for a provider-body Riverpod subscription - see
/// `GoRouterRefreshListenable`'s doc comment for why the listenable itself
/// stays decoupled from `Ref`/`ProviderContainer`).
///
/// DEVIATION FROM SPEC (added while validating the web flow end-to-end
/// against a real backend, see frontend/README.md "Desvios da spec"):
/// `app_router.dart`'s doc comment says track/album detail routes and a
/// playlist detail screen are out of Fatia 1 scope and leaves
/// `onResultTap`/`onPlaylistTap` as no-ops by default. That left tapping a
/// track in search results doing literally nothing in the shipped app -
/// no way to reach playback through the UI at all, which is a real gap,
/// not just a test-scaffolding one. Wiring `onResultTap` here (the single
/// composition root shared by both `smusic_mobile` and `smusic_web`, per
/// frontend-flutter.md section 1.3) is the minimal fix: tapping a track
/// result fetches the full `Track` (for its `durationMs`, which
/// `SearchResultItem` doesn't carry) and starts playback via the same
/// `PlaybackQueueController` the expanded player screen uses, then
/// navigates to `/player`. Album/artist/playlist results remain no-ops
/// (still out of scope - no detail screen exists for them yet).
final appRouterProvider = Provider((ref) {
  final refreshListenable = GoRouterRefreshListenable();
  ref.listen(authSessionProvider, (previous, next) => refreshListenable.notify());
  ref.onDispose(refreshListenable.dispose);

  return buildAppRouter(
    isAuthenticated: () => ref.read(authSessionProvider).value != null,
    refreshListenable: refreshListenable,
    onResultTap: (context, item) async {
      if (item.type != SearchResultType.track) return;
      final track = await ref.read(libraryRepositoryProvider).getTrack(item.id);
      await ref.read(playbackQueueControllerProvider).playFromQueue(
        [
          QueueItem(
            trackId: track.id,
            title: track.title,
            artistName: track.artistName,
            durationMs: track.durationMs,
          ),
        ],
        startIndex: 0,
      );
      if (context.mounted) context.push('/player');
    },
  );
});
