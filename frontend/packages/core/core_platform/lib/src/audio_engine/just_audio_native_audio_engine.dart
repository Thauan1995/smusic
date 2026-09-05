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

/// Builds the [ja.ConcatenatingAudioSource] `load()` seeds the player with,
/// containing only [source] initially.
///
/// Extracted as a pure top-level function - like
/// [mapJustAudioProcessingState] above - purely for testability:
/// constructing a [ja.ConcatenatingAudioSource] and calling `.add()` on one
/// that has never been attached to a real [ja.AudioPlayer] (`_player` stays
/// null internally until `AudioPlayer.setAudioSource` attaches it) touches
/// no platform channel, so this is real, unit-testable logic - unlike
/// `_player.setAudioSource(...)` itself, which is the actual platform-glue
/// call `load()` makes and stays coverage:ignore'd below.
///
/// See .vibeflow/specs/gapless-playback-engine.md: this is what makes
/// `setNextSource`'s `current is ja.ConcatenatingAudioSource` check
/// actually true at runtime - before this, `load()` always produced a
/// plain, non-concatenating source, so that check could never pass.
ja.ConcatenatingAudioSource buildInitialAudioSource(AudioSource source) {
  return ja.ConcatenatingAudioSource(
    children: [ja.AudioSource.uri(source.uri, headers: source.headers)],
  );
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
      // Seeds a ConcatenatingAudioSource (via buildInitialAudioSource)
      // instead of a plain source, so setNextSource below can actually
      // append the prefetched next track for a real gapless transition -
      // see .vibeflow/specs/gapless-playback-engine.md.
      await _player.setAudioSource(buildInitialAudioSource(source));
    } catch (e) {
      throw AudioEngineException('Failed to load audio source', cause: e);
    }
  }

  @override
  Future<void> setNextSource(AudioSource? source) async {
    // Gapless queueing via just_audio requires a ConcatenatingAudioSource to
    // be the currently loaded source - guaranteed by load() above always
    // seeding one. player_data's TrackSourceResolver is responsible for
    // resolving the next track's source; at the engine boundary we still
    // expose a best-effort no-op if the current source somehow isn't
    // concatenating (e.g. before the first load()), rather than throwing.
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
