import 'package:core_design_system/core_design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:library_domain/library_domain.dart';
import 'package:library_ui/library_ui.dart';

import '../../support/fake_library_repository.dart';

Widget _wrap(Widget child, FakeLibraryRepository repository) {
  return ProviderScope(
    overrides: [libraryRepositoryProvider.overrideWithValue(repository)],
    child: MaterialApp(theme: SmusicTheme.light(), home: child),
  );
}

void main() {
  testWidgets('shows a skeleton while loading', (tester) async {
    await tester.pumpWidget(_wrap(const LibraryScreen(), FakeLibraryRepository()));
    // Before the first pump/settle, the provider is still in its initial
    // loading AsyncValue.
    expect(find.byType(TrackListSkeleton), findsOneWidget);
    await tester.pumpAndSettle();
  });

  testWidgets('shows an empty state when there are no playlists', (tester) async {
    await tester.pumpWidget(_wrap(const LibraryScreen(), FakeLibraryRepository()));
    await tester.pumpAndSettle();

    expect(find.textContaining('No playlists yet'), findsOneWidget);
  });

  testWidgets('shows an error state with a working retry action', (tester) async {
    final repository = FakeLibraryRepository()..throwOnGetPlaylists = Exception('boom');
    await tester.pumpWidget(_wrap(const LibraryScreen(), repository));
    await tester.pumpAndSettle();

    expect(find.text('Could not load your library.'), findsOneWidget);

    repository.throwOnGetPlaylists = null;
    repository.playlists = [const Playlist(id: '1', name: 'Chill', isPublic: false)];
    await tester.tap(find.text('Retry'));
    await tester.pumpAndSettle();

    expect(find.text('Chill'), findsOneWidget);
  });

  testWidgets('renders a virtualized list of playlists and handles taps', (tester) async {
    final repository = FakeLibraryRepository()
      ..playlists = [
        const Playlist(id: '1', name: 'Chill', isPublic: false),
        const Playlist(id: '2', name: 'Party', isPublic: true),
      ];
    Playlist? tapped;
    await tester.pumpWidget(_wrap(
      LibraryScreen(onPlaylistTap: (p) => tapped = p),
      repository,
    ));
    await tester.pumpAndSettle();

    expect(find.byType(ListView), findsOneWidget);
    expect(find.text('Chill'), findsOneWidget);
    expect(find.text('Party'), findsOneWidget);
    expect(find.text('Public playlist'), findsOneWidget);
    expect(find.text('Private playlist'), findsOneWidget);

    await tester.tap(find.text('Party'));
    expect(tapped?.name, 'Party');
  });

  testWidgets('pull-to-refresh reloads the list', (tester) async {
    final repository = FakeLibraryRepository()
      ..playlists = [const Playlist(id: '1', name: 'Chill', isPublic: false)];
    await tester.pumpWidget(_wrap(const LibraryScreen(), repository));
    await tester.pumpAndSettle();

    repository.playlists = [const Playlist(id: '2', name: 'Focus', isPublic: false)];
    await tester.fling(find.byType(ListView), const Offset(0, 300), 1000);
    await tester.pumpAndSettle();

    expect(find.text('Focus'), findsOneWidget);
  });

  testWidgets('creates a playlist through the FAB dialog', (tester) async {
    final repository = FakeLibraryRepository();
    await tester.pumpWidget(_wrap(const LibraryScreen(), repository));
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(const Key('create_playlist_fab')));
    await tester.pumpAndSettle();

    await tester.enterText(find.byKey(const Key('create_playlist_name_field')), 'Road Trip');
    await tester.tap(find.byKey(const Key('create_playlist_confirm_button')));
    await tester.pumpAndSettle();

    expect(find.text('Road Trip'), findsOneWidget);
  });

  testWidgets('cancelling the dialog creates nothing', (tester) async {
    final repository = FakeLibraryRepository();
    await tester.pumpWidget(_wrap(const LibraryScreen(), repository));
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(const Key('create_playlist_fab')));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Cancel'));
    await tester.pumpAndSettle();

    expect(find.textContaining('No playlists yet'), findsOneWidget);
  });

  testWidgets('confirming with a blank name creates nothing', (tester) async {
    final repository = FakeLibraryRepository();
    await tester.pumpWidget(_wrap(const LibraryScreen(), repository));
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(const Key('create_playlist_fab')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('create_playlist_confirm_button')));
    await tester.pumpAndSettle();

    expect(find.textContaining('No playlists yet'), findsOneWidget);
  });
}
