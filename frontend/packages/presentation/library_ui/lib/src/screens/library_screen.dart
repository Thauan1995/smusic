import 'package:core_design_system/core_design_system.dart';
import 'package:flutter/material.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:library_domain/library_domain.dart';

import '../widgets/playlist_row.dart';

/// The main library screen: "Sua Biblioteca" (task scope item 4) - a
/// virtualized list of the user's playlists (`ListView.builder`, per
/// frontend-flutter.md section 3.1), with skeleton loading (section 3.4)
/// and pull-to-refresh.
class LibraryScreen extends HookConsumerWidget {
  const LibraryScreen({super.key, this.onPlaylistTap});

  final void Function(Playlist playlist)? onPlaylistTap;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final playlistsAsync = ref.watch(libraryPlaylistsProvider);

    return Scaffold(
      appBar: AppBar(title: const Text('Your Library')),
      floatingActionButton: FloatingActionButton(
        key: const Key('create_playlist_fab'),
        onPressed: () => _showCreatePlaylistDialog(context, ref),
        child: const Icon(Icons.add),
      ),
      body: playlistsAsync.when(
        loading: () => const TrackListSkeleton(),
        error: (error, stackTrace) => EmptyState(
          message: 'Could not load your library.',
          icon: Icons.error_outline,
          actionLabel: 'Retry',
          onAction: () => ref.read(libraryPlaylistsProvider.notifier).refresh(),
        ),
        data: (playlists) {
          if (playlists.isEmpty) {
            return const EmptyState(
              message: 'No playlists yet. Tap + to create one.',
              icon: Icons.queue_music,
            );
          }
          return RefreshIndicator(
            onRefresh: () => ref.read(libraryPlaylistsProvider.notifier).refresh(),
            child: ListView.builder(
              itemCount: playlists.length,
              itemBuilder: (context, index) {
                final playlist = playlists[index];
                return PlaylistRow(
                  key: ValueKey(playlist.id),
                  playlist: playlist,
                  onTap: onPlaylistTap == null ? null : () => onPlaylistTap!(playlist),
                );
              },
            ),
          );
        },
      ),
    );
  }

  Future<void> _showCreatePlaylistDialog(BuildContext context, WidgetRef ref) async {
    final controller = TextEditingController();
    final name = await showDialog<String>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: const Text('New playlist'),
        content: TextField(
          key: const Key('create_playlist_name_field'),
          controller: controller,
          autofocus: true,
          decoration: const InputDecoration(labelText: 'Name'),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(dialogContext).pop(),
            child: const Text('Cancel'),
          ),
          TextButton(
            key: const Key('create_playlist_confirm_button'),
            onPressed: () => Navigator.of(dialogContext).pop(controller.text),
            child: const Text('Create'),
          ),
        ],
      ),
    );
    // Deliberately not disposing `controller` here: disposing immediately
    // after the dialog's route pops races the pop transition (the
    // TextField can still be mid-animation-out and rebuild against an
    // already-disposed controller, which crashes with "Tried to build
    // dirty widget in the wrong build scope"). The controller is a
    // short-lived, un-attached-to-any-long-lived-State object scoped to a
    // single dialog invocation, so leaving it for GC is an accepted,
    // documented trade-off here rather than chasing exact pop-animation
    // timing.
    final trimmed = name?.trim();
    if (trimmed == null || trimmed.isEmpty) return;
    await ref.read(libraryPlaylistsProvider.notifier).createPlaylist(name: trimmed);
  }
}
