import '../entities/album.dart';
import '../entities/paginated.dart';
import '../entities/playlist.dart';
import '../entities/search_result_item.dart';
import '../entities/track.dart';

/// Implemented by `library_data` against backend-go.md section 4's
/// catalog/library endpoints. Throws [LibraryException] on failure.
abstract interface class LibraryRepository {
  Future<Paginated<SearchResultItem>> search({
    required String query,
    SearchResultType? type,
    int limit = 20,
    String? cursor,
  });

  Future<Track> getTrack(String trackId);

  Future<Album> getAlbum(String albumId);

  Future<List<Playlist>> getMyPlaylists();

  /// Returns the newly created playlist's id.
  Future<String> createPlaylist({required String name, required bool isPublic});

  Future<void> addTrackToPlaylist({
    required String playlistId,
    required String trackId,
  });

  Future<void> removeTrackFromPlaylist({
    required String playlistId,
    required String trackId,
  });

  Future<void> saveTrack(String trackId);
}
