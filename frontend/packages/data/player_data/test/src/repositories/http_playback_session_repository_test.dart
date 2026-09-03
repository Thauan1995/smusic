import 'package:core_networking/core_networking.dart';
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http_mock_adapter/http_mock_adapter.dart';
import 'package:player_data/player_data.dart';
import 'package:player_domain/player_domain.dart';

void main() {
  late Dio dio;
  late DioAdapter dioAdapter;
  late HttpPlaybackSessionRepository repository;

  setUp(() {
    dio = Dio(BaseOptions(baseUrl: 'https://api.smusic.test'));
    dioAdapter = DioAdapter(
      dio: dio,
      matcher: const UrlRequestMatcher(matchMethod: true),
    );
    final client = ApiClient(baseUrl: 'https://api.smusic.test', dio: dio);
    repository = HttpPlaybackSessionRepository(client);
  });

  test('createSession returns the session id', () async {
    dioAdapter.onPost(
      '/v1/playback/sessions',
      (server) => server.reply(200, {'session_id': 's1'}),
    );
    expect(await repository.createSession(deviceId: 'd1'), 's1');
  });

  test('createSession maps a network failure', () async {
    dioAdapter.onPost(
      '/v1/playback/sessions',
      (server) => server.throws(
        0,
        DioException.connectionError(
          requestOptions: RequestOptions(path: '/v1/playback/sessions'),
          reason: 'offline',
        ),
      ),
    );

    await expectLater(
      () => repository.createSession(deviceId: 'd1'),
      throwsA(
        isA<PlayerException>().having((e) => e.kind, 'kind', PlayerExceptionKind.network),
      ),
    );
  });

  test('play resolves a stream url, without positionMs', () async {
    dioAdapter.onPost(
      '/v1/playback/sessions/s1/play',
      (server) => server.reply(200, {
        'stream_url': 'https://cdn.example.com/t1.m3u8',
        'expires_at': '2026-01-01T12:00:00.000Z',
      }),
    );

    final resolution = await repository.play(sessionId: 's1', trackId: 't1');
    expect(resolution.trackId, 't1');
  });

  test('play forwards positionMs when given', () async {
    dioAdapter.onPost(
      '/v1/playback/sessions/s1/play',
      (server) => server.reply(200, {'stream_url': 'https://cdn.example.com/t1.m3u8'}),
    );

    final resolution = await repository.play(
      sessionId: 's1',
      trackId: 't1',
      positionMs: 5000,
    );
    expect(resolution.trackId, 't1');
  });

  test('play maps a 404 to sessionNotFound', () async {
    dioAdapter.onPost(
      '/v1/playback/sessions/s1/play',
      (server) => server.reply(404, {'message': 'session gone'}),
    );

    await expectLater(
      () => repository.play(sessionId: 's1', trackId: 't1'),
      throwsA(
        isA<PlayerException>().having((e) => e.kind, 'kind', PlayerExceptionKind.sessionNotFound),
      ),
    );
  });

  test('pause completes', () async {
    dioAdapter.onPost(
      '/v1/playback/sessions/s1/pause',
      (server) => server.reply(204, null),
    );
    await repository.pause(sessionId: 's1');
  });

  test('pause maps a 401 to unauthorized', () async {
    dioAdapter.onPost(
      '/v1/playback/sessions/s1/pause',
      (server) => server.reply(401, {'message': 'nope'}),
    );

    await expectLater(
      () => repository.pause(sessionId: 's1'),
      throwsA(
        isA<PlayerException>().having((e) => e.kind, 'kind', PlayerExceptionKind.unauthorized),
      ),
    );
  });

  test('seek completes', () async {
    dioAdapter.onPost(
      '/v1/playback/sessions/s1/seek',
      (server) => server.reply(204, null),
    );
    await repository.seek(sessionId: 's1', positionMs: 1000);
  });

  test('seek maps an unknown server error', () async {
    dioAdapter.onPost(
      '/v1/playback/sessions/s1/seek',
      (server) => server.reply(500, {'message': 'boom'}),
    );

    await expectLater(
      () => repository.seek(sessionId: 's1', positionMs: 1000),
      throwsA(
        isA<PlayerException>().having((e) => e.kind, 'kind', PlayerExceptionKind.unknown),
      ),
    );
  });

  test('next resolves the next track', () async {
    dioAdapter.onPost(
      '/v1/playback/sessions/s1/next',
      (server) => server.reply(200, {
        'track_id': 't2',
        'stream_url': 'https://cdn.example.com/t2.m3u8',
      }),
    );

    final resolution = await repository.next(sessionId: 's1');
    expect(resolution.trackId, 't2');
  });

  test('enqueue defaults position to end', () async {
    dioAdapter.onPost(
      '/v1/playback/sessions/s1/queue',
      (server) => server.reply(204, null),
    );
    await repository.enqueue(sessionId: 's1', trackIds: ['t1', 't2']);
  });

  test('enqueue forwards a custom position', () async {
    dioAdapter.onPost(
      '/v1/playback/sessions/s1/queue',
      (server) => server.reply(204, null),
    );
    await repository.enqueue(sessionId: 's1', trackIds: ['t1'], position: 'next');
  });
}
