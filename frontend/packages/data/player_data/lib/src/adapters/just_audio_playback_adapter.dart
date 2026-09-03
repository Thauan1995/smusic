import 'dart:async';

import 'package:core_platform/core_platform.dart';
import 'package:player_domain/player_domain.dart';

/// The single [PlaybackQueueController] implementation, per
/// frontend-flutter.md section 2.1/2.6: translates imperative queue
/// commands into [NativeAudioEngine] calls (actual audio) plus
/// [PlaybackSessionRepository] calls (keeping the backend's session state -
/// cross-device sync, play-event billing - in sync), and republishes engine
/// callbacks as the engine-independent [PlayerState] machine.
///
/// SCOPE FOR FATIA 1 (documented deviations - see frontend/README.md):
/// - No predictive prefetch of the next ~3 tracks (frontend-flutter.md
///   section 2.2) - `NativeAudioEngine.setNextSource` exists on the
///   interface but is never called by this adapter yet; each
///   skip/play issues a fresh `PlaybackSessionRepository` call instead of
///   resolving ahead of time. Gapless-*within* a single loaded source still
///   works because it's `just_audio`'s default behavior, but back-to-back
///   *different* tracks are not yet gapless.
/// - No crossfade (`setCrossfadeDuration`) - out of scope per the task.
/// - `TrackSourceResolver`/offline-first source resolution (section 2.5) is
///   not implemented - every source is a network stream URL.
class JustAudioPlaybackAdapter implements PlaybackQueueController {
  JustAudioPlaybackAdapter({
    required NativeAudioEngine engine,
    required PlaybackSessionRepository sessionRepository,
    required String deviceId,
  })  : _engine = engine,
        _sessionRepository = sessionRepository,
        _deviceId = deviceId {
    _positionSub = _engine.positionStream.listen((event) {
      _lastKnownPosition = event.position;
      _emitCurrentState();
    });
    _engineStateSub = _engine.engineStateStream.listen((state) {
      _engineState = state;
      _emitCurrentState();
    });
    _completionSub = _engine.completionStream.listen((_) => skipNext());
  }

  final NativeAudioEngine _engine;
  final PlaybackSessionRepository _sessionRepository;
  final String _deviceId;

  String? _sessionId;
  List<QueueItem> _queue = const [];
  int _currentIndex = -1;
  Duration _lastKnownPosition = Duration.zero;
  bool _isPlaying = false;
  PlaybackEngineState _engineState = PlaybackEngineState.idle;

  late final StreamSubscription<PlaybackPositionEvent> _positionSub;
  late final StreamSubscription<PlaybackEngineState> _engineStateSub;
  late final StreamSubscription<void> _completionSub;

  final StreamController<PlayerState> _stateController =
      StreamController.broadcast();
  final StreamController<QueueItem?> _nowPlayingController =
      StreamController.broadcast();

  @override
  Stream<PlayerState> get stateStream => _stateController.stream;

  @override
  Stream<QueueItem?> get nowPlayingStream => _nowPlayingController.stream;

  QueueItem? get _current =>
      (_currentIndex >= 0 && _currentIndex < _queue.length)
          ? _queue[_currentIndex]
          : null;

  Future<void> _ensureSession() async {
    _sessionId ??= await _sessionRepository.createSession(deviceId: _deviceId);
  }

  void _emitCurrentState() {
    final item = _current;
    if (item == null) {
      _stateController.add(const PlayerState.idle());
      return;
    }
    if (_engineState == PlaybackEngineState.error) {
      _stateController.add(
        const PlayerState.error(PlayerError('Playback engine reported an error')),
      );
      return;
    }
    if (_engineState == PlaybackEngineState.loading ||
        _engineState == PlaybackEngineState.buffering) {
      _stateController.add(PlayerState.buffering(item));
      return;
    }
    _stateController.add(
      _isPlaying
          ? PlayerState.playing(item, _lastKnownPosition)
          : PlayerState.paused(item, _lastKnownPosition),
    );
  }

  Future<void> _loadAndPlay(PlaybackTrackResolution resolution) async {
    await _engine.load(AudioSource(id: resolution.trackId, uri: resolution.streamUrl));
    _isPlaying = true;
    await _engine.play();
  }

  @override
  Future<void> playFromQueue(
    List<QueueItem> queue, {
    required int startIndex,
  }) async {
    _queue = queue;
    _currentIndex = startIndex;
    final item = _current;
    if (item == null) return;

    _nowPlayingController.add(item);
    _stateController.add(PlayerState.buffering(item));
    try {
      await _ensureSession();
      final resolution =
          await _sessionRepository.play(sessionId: _sessionId!, trackId: item.trackId);
      await _loadAndPlay(resolution);
    } catch (e) {
      _stateController.add(
        PlayerState.error(PlayerError('Failed to start playback', cause: e)),
      );
    }
  }

  @override
  Future<void> pause() async {
    if (_current == null) return;
    _isPlaying = false;
    await _engine.pause();
    if (_sessionId != null) {
      try {
        await _sessionRepository.pause(sessionId: _sessionId!);
      } catch (_) {
        // Best-effort backend sync - local playback state (already paused
        // via NativeAudioEngine above) is the source of truth for the UI.
      }
    }
    _emitCurrentState();
  }

  @override
  Future<void> resume() async {
    if (_current == null) return;
    _isPlaying = true;
    await _engine.play();
    _emitCurrentState();
  }

  @override
  Future<void> skipNext() async {
    if (_currentIndex + 1 >= _queue.length) {
      // End of queue - stop rather than error.
      _isPlaying = false;
      await _engine.pause();
      _emitCurrentState();
      return;
    }
    _currentIndex++;
    final item = _current!;
    _nowPlayingController.add(item);
    _stateController.add(PlayerState.buffering(item));
    try {
      await _ensureSession();
      final resolution = await _sessionRepository.next(sessionId: _sessionId!);
      await _loadAndPlay(resolution);
    } catch (e) {
      _stateController.add(
        PlayerState.error(PlayerError('Failed to skip to next track', cause: e)),
      );
    }
  }

  @override
  Future<void> skipPrevious() async {
    // Spotify-style behavior: restart the current track if we're already
    // more than 3s into it, otherwise go to the actual previous track.
    if (_currentIndex <= 0 || _lastKnownPosition > const Duration(seconds: 3)) {
      await seekTo(Duration.zero);
      return;
    }
    _currentIndex--;
    final item = _current!;
    _nowPlayingController.add(item);
    _stateController.add(PlayerState.buffering(item));
    try {
      await _ensureSession();
      final resolution =
          await _sessionRepository.play(sessionId: _sessionId!, trackId: item.trackId);
      await _loadAndPlay(resolution);
    } catch (e) {
      _stateController.add(
        PlayerState.error(PlayerError('Failed to skip to previous track', cause: e)),
      );
    }
  }

  @override
  Future<void> seekTo(Duration position) async {
    if (_current == null) return;
    await _engine.seek(position);
    _lastKnownPosition = position;
    if (_sessionId != null) {
      try {
        await _sessionRepository.seek(
          sessionId: _sessionId!,
          positionMs: position.inMilliseconds,
        );
      } catch (_) {
        // Best-effort backend sync, same rationale as pause() above.
      }
    }
    _emitCurrentState();
  }

  Future<void> dispose() async {
    await _positionSub.cancel();
    await _engineStateSub.cancel();
    await _completionSub.cancel();
    await _stateController.close();
    await _nowPlayingController.close();
  }
}
