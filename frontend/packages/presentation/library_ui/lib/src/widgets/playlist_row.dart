import 'package:flutter/material.dart';
import 'package:library_domain/library_domain.dart';

class PlaylistRow extends StatelessWidget {
  const PlaylistRow({super.key, required this.playlist, this.onTap});

  final Playlist playlist;
  final VoidCallback? onTap;

  @override
  Widget build(BuildContext context) {
    return ListTile(
      leading: CircleAvatar(
        backgroundColor: Theme.of(context).colorScheme.secondaryContainer,
        child: const Icon(Icons.queue_music),
      ),
      title: Text(playlist.name),
      subtitle: Text(playlist.isPublic ? 'Public playlist' : 'Private playlist'),
      onTap: onTap,
    );
  }
}
