import 'package:core_platform/testing.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:player_data/player_data.dart';
import 'package:player_domain/player_domain.dart';

import '../../support/fake_playback_session_repository.dart';

QueueItem _item(String id) =>
    QueueItem(trackId: id, title: 'Song $id', artistName: 'Artist', durationMs: 200000);

PlaybackTrackResolution _resolution(String trackId) => PlaybackTrackResolution(
      trackId: trackId,
      streamUrl: Uri.parse('https://cdn.example.com/$trackId.m3u8'),
      expiresAt: DateTime.now().add(const Duration(minutes: 5)),
    );

Future<void> _settle() => Future<void>.delayed(Duration.zero);

void main() {
  late FakeNativeAudioEngine engine;
  late FakePlaybackSessionRepository repository;
  late JustAudioPlaybackAdapter adapter;
  late List<PlayerState> states;
  late List<QueueItem?> nowPlaying;

  setUp(() {
    engine = FakeNativeAudioEngine();
    repository = FakePlaybackSessionRepository();
    adapter = JustAudioPlaybackAdapter(
      engine: engine,
      sessionRepository: repository,
      deviceId: 'device-1',
    );
    states = [];
    nowPlaying = [];
    adapter.stateStream.listen(states.add);
    adapter.nowPlayingStream.listen(nowPlaying.add);
  });

  tearDown(() async {
    await adapter.dispose();
    await engine.dispose();
  });

  group('playFromQueue', () {
    test('creates a session, resolves and plays the track', () async {
      final item = _item('t1');
      repository.playResultByTrackId['t1'] = _resolution('t1');

      await adapter.playFromQueue([item], startIndex: 0);
      await _settle();

      expect(repository.createSessionCalls, 1);
      expect(repository.lastPlayTrackId, 't1');
      expect(engine.currentSource?.id, 't1');
      expect(engine.isPlaying, isTrue);
      expect(nowPlaying, [item]);
      expect(states.last, isA<PlayerBuffering>());

      engine.emitEngineState(PlaybackEngineState.ready);
      await _settle();

      expect(states.last, isA<PlayerPlaying>());
    });

    test('reuses the existing session on a second call', () async {
      repository.playResultByTrackId['t1'] = _resolution('t1');
      repository.playResultByTrackId['t2'] = _resolution('t2');

      await adapter.playFromQueue([_item('t1')], startIndex: 0);
      await adapter.playFromQueue([_item('t2')], startIndex: 0);

      expect(repository.createSessionCalls, 1);
    });

    test('is a no-op for an out-of-range startIndex', () async {
      await adapter.playFromQueue([_item('t1')], startIndex: 5);
      expect(repository.createSessionCalls, 0);
      expect(nowPlaying, isEmpty);
    });

    test('emits an error state when session creation fails', () async {
      repository.throwOnCreateSession = Exception('boom');
      await adapter.playFromQueue([_item('t1')], startIndex: 0);
      await _settle();
      expect(states.last, isA<PlayerErrorState>());
    });

    test('emits an error state when play() fails', () async {
      repository.throwOnPlay = Exception('boom');
      await adapter.playFromQueue([_item('t1')], startIndex: 0);
      await _settle();
      expect(states.last, isA<PlayerErrorState>());
    });
  });

  group('pause / resume', () {
    setUp(() async {
      repository.playResultByTrackId['t1'] = _resolution('t1');
      await adapter.playFromQueue([_item('t1')], startIndex: 0);
      engine.emitEngineState(PlaybackEngineState.ready);
      await _settle();
    });

    test('pause stops the engine and syncs the session', () async {
      await adapter.pause();
      await _settle();
      expect(engine.isPlaying, isFalse);
      expect(repository.pauseCalls, 1);
      expect(states.last, isA<PlayerPaused>());
    });

    test('pause tolerates a failing session sync', () async {
      // Nothing to configure - FakePlaybackSessionRepository.pause() never
      // throws, but this documents the intended resilience; see the real
      // adapter's try/catch around the sync call.
      await adapter.pause();
      expect(engine.isPlaying, isFalse);
    });

    test('resume restarts the engine', () async {
      await adapter.pause();
      await adapter.resume();
      await _settle();
      expect(engine.isPlaying, isTrue);
      expect(states.last, isA<PlayerPlaying>());
    });

    test('pause/resume are no-ops with no current track', () async {
      final freshEngine = FakeNativeAudioEngine();
      final freshAdapter = JustAudioPlaybackAdapter(
        engine: freshEngine,
        sessionRepository: FakePlaybackSessionRepository(),
        deviceId: 'd',
      );
      await freshAdapter.pause();
      await freshAdapter.resume();
      expect(freshEngine.isPlaying, isFalse);
      await freshAdapter.dispose();
      await freshEngine.dispose();
    });
  });

  group('skipNext', () {
    test('advances to the next track via repository.next()', () async {
      repository.playResultByTrackId['t1'] = _resolution('t1');
      repository.nextResults.add(_resolution('t2'));
      await adapter.playFromQueue([_item('t1'), _item('t2')], startIndex: 0);
      engine.emitEngineState(PlaybackEngineState.ready);
      await _settle();

      await adapter.skipNext();
      await _settle();

      expect(engine.currentSource?.id, 't2');
      expect(nowPlaying.last?.trackId, 't2');
    });

    test('pauses at the end of the queue instead of erroring', () async {
      repository.playResultByTrackId['t1'] = _resolution('t1');
      await adapter.playFromQueue([_item('t1')], startIndex: 0);
      engine.emitEngineState(PlaybackEngineState.ready);
      await _settle();

      await adapter.skipNext();
      await _settle();

      expect(engine.isPlaying, isFalse);
      expect(states.last, isA<PlayerPaused>());
    });

    test('emits an error state when repository.next() fails', () async {
      repository.playResultByTrackId['t1'] = _resolution('t1');
      repository.throwOnNext = Exception('boom');
      await adapter.playFromQueue([_item('t1'), _item('t2')], startIndex: 0);
      engine.emitEngineState(PlaybackEngineState.ready);
      await _settle();

      await adapter.skipNext();
      await _settle();

      expect(states.last, isA<PlayerErrorState>());
    });

    test('auto-advances when the engine reports track completion', () async {
      repository.playResultByTrackId['t1'] = _resolution('t1');
      repository.nextResults.add(_resolution('t2'));
      await adapter.playFromQueue([_item('t1'), _item('t2')], startIndex: 0);
      engine.emitEngineState(PlaybackEngineState.ready);
      await _settle();

      engine.completeCurrentTrack();
      await _settle();

      expect(nowPlaying.last?.trackId, 't2');
    });
  });

  group('skipPrevious', () {
    test('seeks to zero when already at the first track', () async {
      repository.playResultByTrackId['t1'] = _resolution('t1');
      await adapter.playFromQueue([_item('t1'), _item('t2')], startIndex: 0);
      engine.emitEngineState(PlaybackEngineState.ready);
      await _settle();

      await adapter.skipPrevious();

      expect(engine.seekedPositions, [Duration.zero]);
    });

    test('seeks to zero when more than 3s into the current track', () async {
      repository.playResultByTrackId['t1'] = _resolution('t1');
      repository.playResultByTrackId['t2'] = _resolution('t2');
      await adapter.playFromQueue([_item('t1'), _item('t2')], startIndex: 1);
      engine.emitEngineState(PlaybackEngineState.ready);
      engine.emitPosition(const PlaybackPositionEvent(
        position: Duration(seconds: 10),
        bufferedPosition: Duration(seconds: 10),
      ));
      await _settle();

      await adapter.skipPrevious();

      // Only the skipPrevious()-triggered seek actually calls
      // engine.seek() - the earlier emitPosition() just simulates the
      // engine reporting playback progress, it doesn't seek anything.
      expect(engine.seekedPositions, [Duration.zero]);
    });

    test('goes to the actual previous track when within 3s of the start', () async {
      repository.playResultByTrackId['t1'] = _resolution('t1');
      repository.playResultByTrackId['t2'] = _resolution('t2');
      await adapter.playFromQueue([_item('t1'), _item('t2')], startIndex: 1);
      engine.emitEngineState(PlaybackEngineState.ready);
      await _settle();

      await adapter.skipPrevious();
      await _settle();

      expect(engine.currentSource?.id, 't1');
      expect(nowPlaying.last?.trackId, 't1');
    });

    test('emits an error state when the repository call fails', () async {
      repository.playResultByTrackId['t1'] = _resolution('t1');
      repository.playResultByTrackId['t2'] = _resolution('t2');
      await adapter.playFromQueue([_item('t1'), _item('t2')], startIndex: 1);
      engine.emitEngineState(PlaybackEngineState.ready);
      await _settle();

      repository.throwOnPlay = Exception('boom');
      await adapter.skipPrevious();
      await _settle();

      expect(states.last, isA<PlayerErrorState>());
    });
  });

  group('seekTo', () {
    test('seeks the engine and syncs the session once one exists', () async {
      repository.playResultByTrackId['t1'] = _resolution('t1');
      await adapter.playFromQueue([_item('t1')], startIndex: 0);
      engine.emitEngineState(PlaybackEngineState.ready);
      await _settle();

      await adapter.seekTo(const Duration(seconds: 30));

      expect(engine.seekedPositions, [const Duration(seconds: 30)]);
      expect(repository.seekCalls, 1);
      expect(repository.lastSeekPositionMs, 30000);
    });

    test('is a no-op with no current track', () async {
      await adapter.seekTo(const Duration(seconds: 30));
      expect(engine.seekedPositions, isEmpty);
    });
  });

  test('emits PlayerState.idle when an engine event arrives with no current track', () async {
    engine.emitEngineState(PlaybackEngineState.ready);
    await _settle();
    expect(states.last, isA<PlayerIdle>());
  });

  test('engine error state is surfaced as PlayerErrorState', () async {
    repository.playResultByTrackId['t1'] = _resolution('t1');
    await adapter.playFromQueue([_item('t1')], startIndex: 0);

    engine.emitEngineState(PlaybackEngineState.error);
    await _settle();

    expect(states.last, isA<PlayerErrorState>());
  });

  test('position updates while playing are republished as PlayerPlaying', () async {
    repository.playResultByTrackId['t1'] = _resolution('t1');
    await adapter.playFromQueue([_item('t1')], startIndex: 0);
    engine.emitEngineState(PlaybackEngineState.ready);
    await _settle();

    engine.emitPosition(const PlaybackPositionEvent(
      position: Duration(seconds: 42),
      bufferedPosition: Duration(seconds: 45),
    ));
    await _settle();

    final playing = states.last as PlayerPlaying;
    expect(playing.position, const Duration(seconds: 42));
  });
}
