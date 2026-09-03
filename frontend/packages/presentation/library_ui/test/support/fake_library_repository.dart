import 'package:library_domain/library_domain.dart';

class FakeLibraryRepository implements LibraryRepository {
  List<Playlist> playlists = [];
  String createdPlaylistId = 'p-new';
  Object? throwOnGetPlaylists;
  Object? throwOnSearch;
  final List<Paginated<SearchResultItem>> searchPages = [];
  int searchCalls = 0;
  String? lastSearchQuery;
  String? lastSearchCursor;

  @override
  Future<List<Playlist>> getMyPlaylists() async {
    if (throwOnGetPlaylists != null) throw throwOnGetPlaylists!;
    return playlists;
  }

  @override
  Future<String> createPlaylist({required String name, required bool isPublic}) async {
    playlists = [...playlists, Playlist(id: createdPlaylistId, name: name, isPublic: isPublic)];
    return createdPlaylistId;
  }

  @override
  Future<Paginated<SearchResultItem>> search({
    required String query,
    SearchResultType? type,
    int limit = 20,
    String? cursor,
  }) async {
    lastSearchQuery = query;
    lastSearchCursor = cursor;
    // A real (Timer-based) delay: without it, the debounce firing and the
    // network resolving would both complete within a single
    // `tester.pump(duration)` call, making the screen's loading skeleton
    // impossible to observe in a widget test (same rationale as auth_ui's
    // fakes.dart).
    await Future<void>.delayed(const Duration(milliseconds: 200));
    if (throwOnSearch != null) throw throwOnSearch!;
    if (searchPages.isEmpty) return const Paginated.empty();
    final index = searchCalls < searchPages.length ? searchCalls : searchPages.length - 1;
    searchCalls++;
    return searchPages[index];
  }

  @override
  Future<Track> getTrack(String trackId) => throw UnimplementedError();

  @override
  Future<Album> getAlbum(String albumId) => throw UnimplementedError();

  @override
  Future<void> addTrackToPlaylist({required String playlistId, required String trackId}) async {}

  @override
  Future<void> removeTrackFromPlaylist({required String playlistId, required String trackId}) async {}

  @override
  Future<void> saveTrack(String trackId) async {}
}
