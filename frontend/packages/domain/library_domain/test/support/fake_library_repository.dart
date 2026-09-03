import 'package:library_domain/library_domain.dart';

class FakeLibraryRepository implements LibraryRepository {
  /// Optional artificial delay before `search()` resolves, per call index
  /// (0-based). Used to simulate out-of-order responses in
  /// `SearchNotifier` tests. Falls back to `Duration.zero` once exhausted.
  List<Duration> searchDelays = [];
  List<Playlist> playlists = [];
  Track? trackResult;
  Album? albumResult;
  String createdPlaylistId = 'playlist-1';
  Object? throwOnSearch;
  Object? throwOnCreatePlaylist;

  /// Queue of pages returned by successive `search()` calls, in order.
  final List<Paginated<SearchResultItem>> searchPages = [];
  int searchCalls = 0;
  String? lastSearchQuery;
  String? lastSearchCursor;

  int saveTrackCalls = 0;
  String? lastSavedTrackId;
  int addTrackToPlaylistCalls = 0;
  int removeTrackFromPlaylistCalls = 0;

  @override
  Future<Paginated<SearchResultItem>> search({
    required String query,
    SearchResultType? type,
    int limit = 20,
    String? cursor,
  }) async {
    final callIndex = searchCalls;
    searchCalls++;
    lastSearchQuery = query;
    lastSearchCursor = cursor;
    if (callIndex < searchDelays.length) {
      await Future<void>.delayed(searchDelays[callIndex]);
    }
    if (throwOnSearch != null) throw throwOnSearch!;
    if (searchPages.isEmpty) return const Paginated.empty();
    final index = searchCalls - 1 < searchPages.length
        ? searchCalls - 1
        : searchPages.length - 1;
    return searchPages[index];
  }

  @override
  Future<Track> getTrack(String trackId) async => trackResult!;

  @override
  Future<Album> getAlbum(String albumId) async => albumResult!;

  @override
  Future<List<Playlist>> getMyPlaylists() async => playlists;

  @override
  Future<String> createPlaylist({
    required String name,
    required bool isPublic,
  }) async {
    if (throwOnCreatePlaylist != null) throw throwOnCreatePlaylist!;
    return createdPlaylistId;
  }

  @override
  Future<void> addTrackToPlaylist({
    required String playlistId,
    required String trackId,
  }) async {
    addTrackToPlaylistCalls++;
  }

  @override
  Future<void> removeTrackFromPlaylist({
    required String playlistId,
    required String trackId,
  }) async {
    removeTrackFromPlaylistCalls++;
  }

  @override
  Future<void> saveTrack(String trackId) async {
    saveTrackCalls++;
    lastSavedTrackId = trackId;
  }
}
