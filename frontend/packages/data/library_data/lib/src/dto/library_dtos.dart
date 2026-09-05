import 'package:library_domain/library_domain.dart';

/// Mapping between backend-go.md section 4's catalog/library JSON shapes
/// and `library_domain` entities.
///
/// CORRECTED (was an unverified ASSUMPTION, now confirmed wrong against a
/// real running backend during real-browser E2E validation - see
/// frontend/README.md "Desvios da spec" and backend/README.md's CORS
/// section for the validation this was found under):
///
/// - `GET /v1/catalog/tracks/{id}` does **not** return flat `artist`/
///   `album` string fields as originally assumed. It returns
///   `{ id, title, album_id, duration_ms, explicit, artists: [{artist_id,
///   artist_name, role}], available_bitrates: [...] }` (see
///   `backend/internal/catalog/api/handlers.go`'s `trackResponse`).
///   `trackFromJson` below now reads `artists[]` (joining names when a
///   track has multiple credited artists) and falls back to the
///   originally-assumed flat `artist` key only if present (kept so this
///   mapping - and its existing unit tests - still accept the flat shape
///   too). `album` (a title string) genuinely isn't in that response at
///   all, only `album_id` - the backend has no endpoint-level way to give
///   us an album title without a second `GET /v1/catalog/albums/{id}`
///   call, which this mapping deliberately does not make (no screen
///   consumes `Track.albumName` yet in Fatia 1, so a second round trip
///   isn't worth it here); `albumName` is `''` unless a flat `album` key
///   is present. Flagged for the backend specialist: either add an
///   `album_title` field to `trackResponse`, or this stays `''` in
///   production.
/// - `GET /v1/catalog/search` does **not** return a flat `{ results[],
///   next_cursor }` envelope as originally assumed. It returns
///   `{ tracks: [...], albums: [...], artists: [...], next_cursor }`, each
///   using its own full response shape (`trackResponse`/`albumResponse`/
///   `artistResponse` - see `handlers.go`'s `searchResponse`), not a
///   pre-flattened `{id, type, title, subtitle}` row. `searchResultsFromJson`
///   below now flattens all three arrays into `SearchResultItem`s
///   (subtitle = joined artist names for a track, `'Album'`/`'Artist'` for
///   the other two, matching `search_result_row.dart`'s doc comment on
///   what `subtitle` means). The original flat `results[]` shape is still
///   accepted first if present, so existing callers/tests relying on it
///   keep working.
class LibraryDtos {
  const LibraryDtos._(); // coverage:ignore-line

  static Track trackFromJson(Map<String, dynamic> json) {
    final flatArtist = json['artist'] as String?;
    final artistsList = (json['artists'] as List?) ?? const [];
    final artistName = flatArtist ??
        (artistsList.isEmpty
            ? ''
            : artistsList
                .map((a) => (a as Map<String, dynamic>)['artist_name'] as String)
                .join(', '));
    return Track(
      id: json['id'] as String,
      title: json['title'] as String,
      artistName: artistName,
      albumName: (json['album'] as String?) ?? '',
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
      name: (json['title'] ?? json['name']) as String,
      isPublic: json['visibility'] != null
          ? json['visibility'] == 'public'
          : (json['is_public'] as bool?) ?? false,
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
    final flatResults = json['results'] as List?;
    if (flatResults != null) {
      return Paginated(
        items: flatResults
            .map((r) => searchResultItemFromJson(r as Map<String, dynamic>))
            .toList(),
        nextCursor: json['next_cursor'] as String?,
      );
    }

    final items = <SearchResultItem>[];
    for (final raw in (json['tracks'] as List?) ?? const []) {
      final t = raw as Map<String, dynamic>;
      final artists = (t['artists'] as List?) ?? const [];
      final subtitle = artists.isEmpty
          ? ''
          : artists
              .map((a) => (a as Map<String, dynamic>)['artist_name'] as String)
              .join(', ');
      items.add(SearchResultItem(
        id: t['id'] as String,
        type: SearchResultType.track,
        title: t['title'] as String,
        subtitle: subtitle,
      ));
    }
    for (final raw in (json['albums'] as List?) ?? const []) {
      final a = raw as Map<String, dynamic>;
      items.add(SearchResultItem(
        id: a['id'] as String,
        type: SearchResultType.album,
        title: a['title'] as String,
        subtitle: 'Album',
      ));
    }
    for (final raw in (json['artists'] as List?) ?? const []) {
      final a = raw as Map<String, dynamic>;
      items.add(SearchResultItem(
        id: a['id'] as String,
        type: SearchResultType.artist,
        title: a['name'] as String,
        subtitle: 'Artist',
      ));
    }
    return Paginated(items: items, nextCursor: json['next_cursor'] as String?);
  }
}
