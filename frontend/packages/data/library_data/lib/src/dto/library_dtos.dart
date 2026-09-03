import 'package:library_domain/library_domain.dart';

/// Mapping between backend-go.md section 4's catalog/library JSON shapes
/// and `library_domain` entities.
///
/// ASSUMPTION (backend contract gap): backend-go.md's illustrative shapes
/// give `GET /v1/catalog/tracks/{id} -> { id, title, artist, album,
/// duration_ms, available_bitrates[] }` and `GET /v1/catalog/search ->
/// { results[], next_cursor }` without spelling out each result row's exact
/// fields. This mapping assumes each `results[]` entry looks like
/// `{ id, type, title, subtitle }` (`type` one of "track"/"album"/
/// "artist"/"playlist", matching the `type=` query param's own vocabulary)
/// - flagged for the backend specialist to confirm per frontend/README.md.
/// `available_bitrates[]` is read by `player_data`, not here (search/list
/// rows never need it).
class LibraryDtos {
  const LibraryDtos._(); // coverage:ignore-line

  static Track trackFromJson(Map<String, dynamic> json) {
    return Track(
      id: json['id'] as String,
      title: json['title'] as String,
      artistName: json['artist'] as String,
      albumName: json['album'] as String,
      durationMs: json['duration_ms'] as int,
    );
  }

  static Album albumFromJson(Map<String, dynamic> json) {
    final tracksJson = (json['tracks'] as List?) ?? const [];
    return Album(
      id: json['id'] as String,
      title: json['title'] as String,
      tracks: tracksJson
          .map((t) => trackFromJson(t as Map<String, dynamic>))
          .toList(),
    );
  }

  static Playlist playlistFromJson(Map<String, dynamic> json) {
    return Playlist(
      id: json['id'] as String,
      name: json['name'] as String,
      isPublic: (json['is_public'] as bool?) ?? false,
    );
  }

  static List<Playlist> playlistsFromJson(Map<String, dynamic> json) {
    final list = (json['playlists'] as List?) ?? const [];
    return list.map((p) => playlistFromJson(p as Map<String, dynamic>)).toList();
  }

  static SearchResultType searchResultTypeFromJson(String type) {
    return SearchResultType.values.firstWhere(
      (t) => t.name == type,
      orElse: () => SearchResultType.track,
    );
  }

  static SearchResultItem searchResultItemFromJson(Map<String, dynamic> json) {
    return SearchResultItem(
      id: json['id'] as String,
      type: searchResultTypeFromJson(json['type'] as String),
      title: json['title'] as String,
      subtitle: (json['subtitle'] as String?) ?? '',
    );
  }

  static Paginated<SearchResultItem> searchResultsFromJson(
    Map<String, dynamic> json,
  ) {
    final list = (json['results'] as List?) ?? const [];
    return Paginated(
      items: list
          .map((r) => searchResultItemFromJson(r as Map<String, dynamic>))
          .toList(),
      nextCursor: json['next_cursor'] as String?,
    );
  }
}
