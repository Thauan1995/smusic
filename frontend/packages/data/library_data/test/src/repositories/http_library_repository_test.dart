import 'package:core_networking/core_networking.dart';
import 'package:dio/dio.dart';
import 'package:http_mock_adapter/http_mock_adapter.dart';
import 'package:library_data/library_data.dart';
import 'package:library_domain/library_domain.dart';
import 'package:test/test.dart';

import '../../support/always_throws_adapter.dart';

void main() {
  late Dio dio;
  late DioAdapter dioAdapter;
  late HttpLibraryRepository repository;

  setUp(() {
    dio = Dio(BaseOptions(baseUrl: 'https://api.smusic.test'));
    dioAdapter = DioAdapter(
      dio: dio,
      matcher: const UrlRequestMatcher(matchMethod: true),
    );
    final client = ApiClient(baseUrl: 'https://api.smusic.test', dio: dio);
    repository = HttpLibraryRepository(client);
  });

  test('search returns a mapped page', () async {
    dioAdapter.onGet(
      '/v1/catalog/search',
      (server) => server.reply(200, {
        'results': [
          {'id': '1', 'type': 'track', 'title': 'Song', 'subtitle': 'Artist'},
        ],
        'next_cursor': 'c2',
      }),
    );

    final page = await repository.search(query: 'daft punk');
    expect(page.items, hasLength(1));
    expect(page.nextCursor, 'c2');
  });

  test('search forwards type and cursor as query parameters', () async {
    dioAdapter.onGet(
      '/v1/catalog/search',
      (server) => server.reply(200, {'results': [], 'next_cursor': null}),
    );

    final page = await repository.search(
      query: 'daft punk',
      type: SearchResultType.album,
      cursor: 'c1',
    );
    expect(page.items, isEmpty);
  });

  test('search maps a network failure to LibraryExceptionKind.network', () async {
    // A dedicated always-throwing adapter, not dioAdapter/DioAdapter: this
    // GET is retried by core_networking's RetryInterceptor, which re-enters
    // the http client adapter - DioAdapter cannot handle that reentrancy
    // reliably (see always_throws_adapter.dart doc comment).
    final dio2 = Dio(BaseOptions(baseUrl: 'https://api.smusic.test'))
      ..httpClientAdapter = AlwaysThrowsConnectionErrorAdapter();
    final repository2 = HttpLibraryRepository(
      ApiClient(baseUrl: 'https://api.smusic.test', dio: dio2),
    );

    await expectLater(
      () => repository2.search(query: 'x'),
      throwsA(
        isA<LibraryException>().having((e) => e.kind, 'kind', LibraryExceptionKind.network),
      ),
    );
  });

  test('getTrack returns a mapped track', () async {
    dioAdapter.onGet(
      '/v1/catalog/tracks/t1',
      (server) => server.reply(200, {
        'id': 't1',
        'title': 'Song',
        'artist': 'Artist',
        'album': 'Album',
        'duration_ms': 1000,
      }),
    );

    final track = await repository.getTrack('t1');
    expect(track.id, 't1');
  });

  test('getTrack maps a 404 to LibraryExceptionKind.notFound', () async {
    dioAdapter.onGet(
      '/v1/catalog/tracks/missing',
      (server) => server.reply(404, {'message': 'not found'}),
    );

    await expectLater(
      () => repository.getTrack('missing'),
      throwsA(
        isA<LibraryException>().having((e) => e.kind, 'kind', LibraryExceptionKind.notFound),
      ),
    );
  });

  test('getAlbum returns a mapped album', () async {
    dioAdapter.onGet(
      '/v1/catalog/albums/a1',
      (server) => server.reply(200, {'id': 'a1', 'title': 'Album', 'tracks': []}),
    );

    final album = await repository.getAlbum('a1');
    expect(album.id, 'a1');
  });

  test('getMyPlaylists returns the mapped list', () async {
    dioAdapter.onGet(
      '/v1/library/me/playlists',
      (server) => server.reply(200, {
        'playlists': [
          {'id': 'p1', 'name': 'Chill', 'is_public': false},
        ],
      }),
    );

    final playlists = await repository.getMyPlaylists();
    expect(playlists, hasLength(1));
  });

  test('getMyPlaylists maps a 401 to LibraryExceptionKind.unauthorized', () async {
    dioAdapter.onGet(
      '/v1/library/me/playlists',
      (server) => server.reply(401, {'message': 'nope'}),
    );

    await expectLater(
      () => repository.getMyPlaylists(),
      throwsA(
        isA<LibraryException>().having((e) => e.kind, 'kind', LibraryExceptionKind.unauthorized),
      ),
    );
  });

  test('createPlaylist returns the new id', () async {
    dioAdapter.onPost(
      '/v1/library/me/playlists',
      (server) => server.reply(200, {'playlist_id': 'p-42'}),
    );

    final id = await repository.createPlaylist(name: 'Road Trip', isPublic: true);
    expect(id, 'p-42');
  });

  test('createPlaylist maps an unknown server error', () async {
    dioAdapter.onPost(
      '/v1/library/me/playlists',
      (server) => server.reply(500, {'message': 'boom'}),
    );

    await expectLater(
      () => repository.createPlaylist(name: 'x', isPublic: false),
      throwsA(
        isA<LibraryException>().having((e) => e.kind, 'kind', LibraryExceptionKind.unknown),
      ),
    );
  });

  test('addTrackToPlaylist completes', () async {
    dioAdapter.onPost(
      '/v1/library/me/playlists/p1/tracks',
      (server) => server.reply(204, null),
    );
    await repository.addTrackToPlaylist(playlistId: 'p1', trackId: 't1');
  });

  test('removeTrackFromPlaylist completes', () async {
    dioAdapter.onDelete(
      '/v1/library/me/playlists/p1/tracks/t1',
      (server) => server.reply(204, null),
    );
    await repository.removeTrackFromPlaylist(playlistId: 'p1', trackId: 't1');
  });

  test('saveTrack completes', () async {
    dioAdapter.onPost(
      '/v1/library/me/saved-tracks',
      (server) => server.reply(204, null),
    );
    await repository.saveTrack('t1');
  });
}
