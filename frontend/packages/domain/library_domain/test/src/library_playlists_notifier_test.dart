import 'package:library_domain/library_domain.dart';
import 'package:riverpod/riverpod.dart';
import 'package:test/test.dart';

import '../support/fake_library_repository.dart';

void main() {
  test('libraryRepositoryProvider throws when not overridden', () {
    final container = ProviderContainer();
    addTearDown(container.dispose);
    expect(
      () => container.read(libraryRepositoryProvider),
      throwsUnimplementedError,
    );
  });

  group('LibraryPlaylistsNotifier', () {
    late FakeLibraryRepository repository;
    late ProviderContainer container;

    setUp(() {
      repository = FakeLibraryRepository();
      container = ProviderContainer(
        overrides: [libraryRepositoryProvider.overrideWithValue(repository)],
      );
      addTearDown(container.dispose);
    });

    test('build loads playlists from the repository', () async {
      repository.playlists = [Playlist(id: '1', name: 'Chill', isPublic: false)];

      final playlists = await container.read(libraryPlaylistsProvider.future);

      expect(playlists, hasLength(1));
      expect(playlists.first.name, 'Chill');
    });

    test('refresh reloads the list', () async {
      await container.read(libraryPlaylistsProvider.future);
      repository.playlists = [Playlist(id: '2', name: 'Focus', isPublic: true)];

      final notifier = container.read(libraryPlaylistsProvider.notifier);
      await notifier.refresh();

      expect(container.read(libraryPlaylistsProvider).value?.single.name, 'Focus');
    });

    test('createPlaylist creates then refreshes', () async {
      await container.read(libraryPlaylistsProvider.future);
      repository.playlists = [Playlist(id: '3', name: 'New', isPublic: false)];

      final notifier = container.read(libraryPlaylistsProvider.notifier);
      await notifier.createPlaylist(name: 'New');

      expect(container.read(libraryPlaylistsProvider).value?.single.name, 'New');
    });
  });
}
