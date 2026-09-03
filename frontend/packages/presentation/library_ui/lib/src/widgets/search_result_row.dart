import 'package:flutter/material.dart';
import 'package:library_domain/library_domain.dart';

class SearchResultRow extends StatelessWidget {
  const SearchResultRow({super.key, required this.item, this.onTap});

  final SearchResultItem item;
  final VoidCallback? onTap;

  IconData get _icon => switch (item.type) {
        SearchResultType.track => Icons.music_note,
        SearchResultType.album => Icons.album,
        SearchResultType.artist => Icons.person,
        SearchResultType.playlist => Icons.queue_music,
      };

  @override
  Widget build(BuildContext context) {
    return ListTile(
      leading: Icon(_icon),
      title: Text(item.title),
      subtitle: item.subtitle.isEmpty ? null : Text(item.subtitle),
      onTap: onTap,
    );
  }
}
