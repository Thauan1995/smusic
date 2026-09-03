import 'dart:async';

import '../audio_engine/native_audio_engine.dart';

/// Deterministic, controllable fake of [NativeAudioEngine] for unit tests
/// (frontend-flutter.md section 5.2). Every stream is driven manually by the
/// test via the `emit*`/`complete*` helpers below - nothing here touches
/// real audio, timers, or platform channels.
class FakeNativeAudioEngine implements NativeAudioEngine {
  final List<AudioSource> loadedSources = [];
  AudioSource? currentSource;
  AudioSource? nextSource;
  bool isPlaying = false;
  bool disposed = false;
  double volume = 1;
  final List<Duration> seekedPositions = [];

  final StreamController<PlaybackPositionEvent> _positionController =
      StreamController.broadcast();
  final StreamController<PlaybackEngineState> _engineStateController =
      StreamController.broadcast();
  final StreamController<void> _completionController =
      StreamController.broadcast();

  /// If set, the next call to [load] throws this exception - used to
  /// simulate a codec/network error deterministically.
  Object? loadError;

  @override
  Future<void> load(AudioSource source) async {
    if (loadError != null) {
      final err = loadError!;
      loadError = null;
      emitEngineState(PlaybackEngineState.error);
      throw err;
    }
    loadedSources.add(source);
    currentSource = source;
    emitEngineState(PlaybackEngineState.loading);
  }

  @override
  Future<void> setNextSource(AudioSource? source) async {
    nextSource = source;
  }

  @override
  Future<void> play() async {
    isPlaying = true;
  }

  @override
  Future<void> pause() async {
    isPlaying = false;
  }

  @override
  Future<void> seek(Duration position) async {
    seekedPositions.add(position);
  }

  @override
  Future<void> setVolume(double volume) async {
    this.volume = volume;
  }

  @override
  Stream<PlaybackPositionEvent> get positionStream =>
      _positionController.stream;

  @override
  Stream<PlaybackEngineState> get engineStateStream =>
      _engineStateController.stream;

  @override
  Stream<void> get completionStream => _completionController.stream;

  @override
  Future<void> dispose() async {
    disposed = true;
    await _positionController.close();
    await _engineStateController.close();
    await _completionController.close();
  }

  // --- test-driven simulation helpers -------------------------------------

  void emitPosition(PlaybackPositionEvent event) =>
      _positionController.add(event);

  void emitEngineState(PlaybackEngineState state) =>
      _engineStateController.add(state);

  /// Simulates the current track finishing, mirroring what a real engine
  /// does when it advances to the buffered [nextSource] (gapless).
  void completeCurrentTrack() {
    emitEngineState(PlaybackEngineState.completed);
    _completionController.add(null);
    if (nextSource != null) {
      currentSource = nextSource;
      nextSource = null;
    }
  }
}
