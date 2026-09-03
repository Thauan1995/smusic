import 'dart:async';

import 'package:just_audio/just_audio.dart' as ja;

import 'native_audio_engine.dart';

/// Pure mapping function, extracted from [JustAudioNativeEngine] purely for
/// testability: `just_audio`'s `AudioPlayer` cannot be constructed/driven in
/// a plain `flutter test` run without platform channel mocks (it is a thin
/// platform binding - see frontend-flutter.md section 5.1's exemption for
/// infra glue), but this mapping is real logic and should not be exempted.
PlaybackEngineState mapJustAudioProcessingState(
  ja.ProcessingState processingState,
) {
  switch (processingState) {
    case ja.ProcessingState.idle:
      return PlaybackEngineState.idle;
    case ja.ProcessingState.loading:
      return PlaybackEngineState.loading;
    case ja.ProcessingState.buffering:
      return PlaybackEngineState.buffering;
    case ja.ProcessingState.ready:
      return PlaybackEngineState.ready;
    case ja.ProcessingState.completed:
      return PlaybackEngineState.completed;
  }
}

/// `just_audio`-backed [NativeAudioEngine].
///
/// This is the *only* class in the whole monorepo that imports
/// `package:just_audio`. It runs unmodified on mobile and web because
/// `just_audio` itself ships platform-specific backends (ExoPlayer/AVPlayer
/// on mobile, `just_audio_web` on web) behind one Dart API - see
/// frontend-flutter.md section 1.3/2.6.
///
/// COVERAGE EXCLUSION (documented per docs/architecture/00-overview.md
/// section 2): the instance methods below are thin bindings onto
/// `package:just_audio`'s `AudioPlayer`, which requires a real platform
/// channel (`JustAudioPlatform.instance`) to do anything - constructing or
/// driving it under plain `flutter test` throws `MissingPluginException`.
/// This is the same category of exclusion backend-go.md section 7 grants to
/// `main.go`/DI wiring: infra glue with no branching business logic. The
/// mapping logic that *is* real logic has been extracted to the
/// top-level, fully-unit-tested `mapJustAudioProcessingState` above.
/// Verified instead by manual smoke test (`flutter run`) - see
/// frontend/README.md "Desvios da spec".
// coverage:ignore-start
class JustAudioNativeEngine implements NativeAudioEngine {
  JustAudioNativeEngine({ja.AudioPlayer? player})
      : _player = player ?? ja.AudioPlayer();

  final ja.AudioPlayer _player;

  StreamController<PlaybackEngineState>? _engineStateController;
  StreamSubscription<ja.PlayerState>? _playerStateSub;
  StreamController<void>? _completionController;

  @override
  Stream<PlaybackEngineState> get engineStateStream {
    _engineStateController ??=
        StreamController<PlaybackEngineState>.broadcast(
      onListen: _attachPlayerStateListener,
      onCancel: () => _playerStateSub?.cancel(),
    );
    return _engineStateController!.stream;
  }

  void _attachPlayerStateListener() {
    _playerStateSub ??= _player.playerStateStream.listen((state) {
      _engineStateController
          ?.add(mapJustAudioProcessingState(state.processingState));
      if (state.processingState == ja.ProcessingState.completed) {
        _completionController?.add(null);
      }
    });
  }

  @override
  Future<void> load(AudioSource source) async {
    try {
      await _player.setAudioSource(
        ja.AudioSource.uri(source.uri, headers: source.headers),
      );
    } catch (e) {
      throw AudioEngineException('Failed to load audio source', cause: e);
    }
  }

  @override
  Future<void> setNextSource(AudioSource? source) async {
    // Gapless queueing via just_audio requires a ConcatenatingAudioSource to
    // be the currently loaded source. player_data's TrackSourceResolver is
    // responsible for building that source when it knows the next track; at
    // the engine boundary we expose a best-effort no-op when the current
    // source isn't concatenating, rather than throwing - gapless is an
    // optimization, not a correctness requirement for Fatia 1.
    final current = _player.audioSource;
    if (current is ja.ConcatenatingAudioSource && source != null) {
      await current.add(
        ja.AudioSource.uri(source.uri, headers: source.headers),
      );
    }
  }

  @override
  Future<void> play() => _player.play();

  @override
  Future<void> pause() => _player.pause();

  @override
  Future<void> seek(Duration position) => _player.seek(position);

  @override
  Future<void> setVolume(double volume) => _player.setVolume(volume);

  @override
  Stream<PlaybackPositionEvent> get positionStream =>
      _player.positionStream.map(
        (position) => PlaybackPositionEvent(
          position: position,
          bufferedPosition: _player.bufferedPosition,
          duration: _player.duration,
        ),
      );

  @override
  Stream<void> get completionStream {
    _completionController ??= StreamController<void>.broadcast();
    _attachPlayerStateListener();
    return _completionController!.stream;
  }

  @override
  Future<void> dispose() async {
    await _playerStateSub?.cancel();
    await _engineStateController?.close();
    await _completionController?.close();
    await _player.dispose();
  }
}
// coverage:ignore-end
