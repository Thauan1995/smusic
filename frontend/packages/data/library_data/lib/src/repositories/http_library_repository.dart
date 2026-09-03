import 'package:core_networking/core_networking.dart';
import 'package:library_domain/library_domain.dart';

import '../dto/library_dtos.dart';

class HttpLibraryRepository implements LibraryRepository {
  HttpLibraryRepository(this._client);

  final ApiClient _client;

  @override
  Future<Paginated<SearchResultItem>> search({
    required String query,
    SearchResultType? type,
    int limit = 20,
    String? cursor,
  }) {
    return _wrap(() async {
      final response = await _client.get(
        '/v1/catalog/search',
        queryParameters: {
          'q': query,
          if (type != null) 'type': type.name,
          'limit': limit,
          if (cursor != null) 'cursor': cursor,
        },
      );
      return LibraryDtos.searchResultsFromJson(response);
    });
  }

  @override
  Future<Track> getTrack(String trackId) {
    return _wrap(() async {
      final response = await _client.get('/v1/catalog/tracks/$trackId');
      return LibraryDtos.trackFromJson(response);
    });
  }

  @override
  Future<Album> getAlbum(String albumId) {
    return _wrap(() async {
      final response = await _client.get('/v1/catalog/albums/$albumId');
      return LibraryDtos.albumFromJson(response);
    });
  }

  @override
  Future<List<Playlist>> getMyPlaylists() {
    return _wrap(() async {
      final response = await _client.get('/v1/library/me/playlists');
      return LibraryDtos.playlistsFromJson(response);
    });
  }

  @override
  Future<String> createPlaylist({required String name, required bool isPublic}) {
    return _wrap(() async {
      final response = await _client.post(
        '/v1/library/me/playlists',
        data: {'name': name, 'is_public': isPublic},
      );
      return response['playlist_id'] as String;
    });
  }

  @override
  Future<void> addTrackToPlaylist({
    required String playlistId,
    required String trackId,
  }) {
    return _wrap(() async {
      await _client.post(
        '/v1/library/me/playlists/$playlistId/tracks',
        data: {'track_id': trackId},
      );
    });
  }

  @override
  Future<void> removeTrackFromPlaylist({
    required String playlistId,
    required String trackId,
  }) {
    return _wrap(() async {
      await _client.delete(
        '/v1/library/me/playlists/$playlistId/tracks/$trackId',
      );
    });
  }

  @override
  Future<void> saveTrack(String trackId) {
    return _wrap(() async {
      await _client.post('/v1/library/me/saved-tracks', data: {'track_id': trackId});
    });
  }

  Future<T> _wrap<T>(Future<T> Function() body) async {
    try {
      return await body();
    } on ApiException catch (e) {
      throw LibraryException(_kindFor(e), message: e.message);
    }
  }

  LibraryExceptionKind _kindFor(ApiException e) {
    if (e.isUnauthorized) return LibraryExceptionKind.unauthorized;
    if (e.isNotFound) return LibraryExceptionKind.notFound;
    if (e.isNetworkError) return LibraryExceptionKind.network;
    return LibraryExceptionKind.unknown;
  }
}
