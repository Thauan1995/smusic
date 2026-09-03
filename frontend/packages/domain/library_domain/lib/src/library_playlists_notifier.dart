import 'dart:async';

import 'package:riverpod/riverpod.dart';

import 'entities/playlist.dart';
import 'repositories/library_repository.dart';
import 'usecases/create_playlist_use_case.dart';

/// Overridden in `app/*` with `library_data`'s implementation - same
/// composition pattern as `auth_domain.authRepositoryProvider`. See that
/// file for the code-gen deviation note (applies here too).
final libraryRepositoryProvider = Provider<LibraryRepository>((ref) {
  throw UnimplementedError(
    'libraryRepositoryProvider must be overridden by app/* with a library_data implementation.',
  );
});

final createPlaylistUseCaseProvider = Provider(
  (ref) => CreatePlaylistUseCase(ref.watch(libraryRepositoryProvider)),
);

/// Backs `library_ui`'s main library screen. A simple `AsyncNotifier`
/// rather than a paging one - `GET /v1/library/me/playlists` is not
/// cursor-paginated in backend-go.md section 4 (unlike catalog search).
class LibraryPlaylistsNotifier extends AsyncNotifier<List<Playlist>> {
  @override
  FutureOr<List<Playlist>> build() async {
    return ref.watch(libraryRepositoryProvider).getMyPlaylists();
  }

  Future<void> refresh() async {
    state = const AsyncLoading<List<Playlist>>().copyWithPrevious(state);
    state = await AsyncValue.guard(
      () => ref.read(libraryRepositoryProvider).getMyPlaylists(),
    );
  }

  Future<void> createPlaylist({required String name, bool isPublic = false}) async {
    final useCase = ref.read(createPlaylistUseCaseProvider);
    await useCase(name: name, isPublic: isPublic);
    await refresh();
  }
}

final libraryPlaylistsProvider =
    AsyncNotifierProvider<LibraryPlaylistsNotifier, List<Playlist>>(
  LibraryPlaylistsNotifier.new,
);
