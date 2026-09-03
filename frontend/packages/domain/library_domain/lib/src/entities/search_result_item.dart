/// Mirrors `GET /v1/catalog/search`'s `type` query param values
/// (backend-go.md section 4).
enum SearchResultType { track, album, artist, playlist }

/// A single row in `GET /v1/catalog/search`'s `results[]`. Deliberately a
/// flat display-oriented shape (not a union of `Track`/`Album`/...) - the
/// search result list only ever needs id/type/title/subtitle to render a
/// row; tapping a result fetches the full entity via the type-specific
/// endpoint if needed.
class SearchResultItem {
  const SearchResultItem({
    required this.id,
    required this.type,
    required this.title,
    required this.subtitle,
  });

  final String id;
  final SearchResultType type;
  final String title;

  /// Artist name for a track, "Album" for an album, etc. - secondary line
  /// in the row.
  final String subtitle;

  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      other is SearchResultItem &&
          other.id == id &&
          other.type == type &&
          other.title == title &&
          other.subtitle == subtitle;

  @override
  int get hashCode => Object.hash(id, type, title, subtitle);
}
