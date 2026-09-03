import 'package:core_platform/testing.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  late FakeNativeAudioEngine engine;

  setUp(() {
    engine = FakeNativeAudioEngine();
  });

  test('load records source and emits loading state', () async {
    final states = <PlaybackEngineState>[];
    engine.engineStateStream.listen(states.add);

    final source = AudioSource(id: 't1', uri: Uri.parse('https://x/t1'));
    await engine.load(source);

    expect(engine.loadedSources, [source]);
    expect(engine.currentSource, source);
    await Future<void>.delayed(Duration.zero);
    expect(states, [PlaybackEngineState.loading]);
  });

  test('load throws and emits error state when loadError is set', () async {
    engine.loadError = Exception('codec error');
    final states = <PlaybackEngineState>[];
    engine.engineStateStream.listen(states.add);

    await expectLater(
      () => engine.load(AudioSource(id: 't1', uri: Uri.parse('https://x/t1'))),
      throwsException,
    );
    await Future<void>.delayed(Duration.zero);
    expect(states, [PlaybackEngineState.error]);
    // loadError is consumed - subsequent load succeeds.
    await engine.load(AudioSource(id: 't2', uri: Uri.parse('https://x/t2')));
    expect(engine.currentSource?.id, 't2');
  });

  test('play/pause toggle isPlaying', () async {
    await engine.play();
    expect(engine.isPlaying, isTrue);
    await engine.pause();
    expect(engine.isPlaying, isFalse);
  });

  test('seek records seeked positions', () async {
    await engine.seek(const Duration(seconds: 5));
    expect(engine.seekedPositions, [const Duration(seconds: 5)]);
  });

  test('emitPosition forwards through positionStream', () async {
    final events = <PlaybackPositionEvent>[];
    engine.positionStream.listen(events.add);

    const event = PlaybackPositionEvent(
      position: Duration(seconds: 1),
      bufferedPosition: Duration(seconds: 2),
    );
    engine.emitPosition(event);

    await Future<void>.delayed(Duration.zero);
    expect(events, [event]);
  });

  test('setVolume stores volume', () async {
    await engine.setVolume(0.5);
    expect(engine.volume, 0.5);
  });

  test('setNextSource stores nextSource', () async {
    final next = AudioSource(id: 'next', uri: Uri.parse('https://x/next'));
    await engine.setNextSource(next);
    expect(engine.nextSource, next);
  });

  test('completeCurrentTrack advances to nextSource when set', () async {
    final first = AudioSource(id: 'first', uri: Uri.parse('https://x/first'));
    final next = AudioSource(id: 'next', uri: Uri.parse('https://x/next'));
    await engine.load(first);
    await engine.setNextSource(next);

    final completions = <void>[];
    engine.completionStream.listen(completions.add);
    final states = <PlaybackEngineState>[];
    engine.engineStateStream.listen(states.add);

    engine.completeCurrentTrack();

    await Future<void>.delayed(Duration.zero);
    expect(completions, hasLength(1));
    expect(states, contains(PlaybackEngineState.completed));
    expect(engine.currentSource, next);
    expect(engine.nextSource, isNull);
  });

  test('completeCurrentTrack keeps currentSource when no nextSource', () async {
    final first = AudioSource(id: 'first', uri: Uri.parse('https://x/first'));
    await engine.load(first);

    engine.completeCurrentTrack();

    expect(engine.currentSource, first);
  });

  test('dispose closes streams and sets disposed flag', () async {
    await engine.dispose();
    expect(engine.disposed, isTrue);
  });
}
