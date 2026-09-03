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
/// - **One-ahead prefetch** (frontend-flutter.md section 2.2's "prefetch
///   antecipado") is implemented: right after the current queue item starts
///   loading, [_prefetchNext] resolves and pushes the *next* queue item to
///   [NativeAudioEngine.setNextSource], so `just_audio` can warm/buffer it
///   ahead of time and (once [JustAudioNativeEngine] loads a
///   `ConcatenatingAudioSource` - a separate, engine-level change not in
///   this adapter) transition gaplessly. The **predictive 3-track-deep**
///   prefetch queue (section 2.2's second bullet) and adaptive bitrate
///   selection are still TODO.
/// - No crossfade (`setCrossfadeDuration`) - out of scope per the task.
/// - `TrackSourceResolver`/offline-first source resolution (section 2.5) is
///   not implemented - every source is a network stream URL.
///
/// ASSUMPTION FLAGGED FOR THE BACKEND SPECIALIST (frontend-flutter.md
/// section 7): `PlaybackSessionRepository` has no side-effect-free "resolve
/// this trackId to a stream URL without marking it now-playing" endpoint -
/// `play()` is the only trackId-addressable resolver, and per
/// backend-go.md section 4 it also marks the session's server-side
/// now-playing pointer. [_prefetchNext] necessarily reuses `play()` for the
/// next item, which means the backend's cross-device "now playing" pointer
/// moves to N+1 slightly before local audio actually reaches it. This is
/// invisible in Fatia 1 (no cross-device Connect-style UI consumes that
/// pointer yet - see frontend/README.md "Desvios da spec" item 11) but
/// should be revisited (e.g. a dedicated non-mutating resolve endpoint) once
/// it does.
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

  /// Resolution already fetched for `_queue[_currentIndex + 1]` by the most
  /// recent [_prefetchNext] call, if any - consumed by [skipNext] to avoid a
  /// redundant network round-trip when the prefetch already warmed it (see
  /// frontend-flutter.md section 2.2/6's skip-latency target). `null` means
  /// "no warm prefetch available", not "no next item" - callers must still
  /// fall back to resolving on demand.
  PlaybackTrackResolution? _prefetchedResolution;

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

  /// Prefetch/gapless (frontend-flutter.md section 2.2/2.3): resolves
  /// `_queue[_currentIndex + 1]` (the item right after whatever just started
  /// loading) and pushes it to [NativeAudioEngine.setNextSource], so
  /// `just_audio` can warm/buffer it ahead of time instead of the queue
  /// advance happening cold. Called after every successful load in
  /// [playFromQueue]/[skipNext]/[skipPrevious] - i.e. every time "the queue
  /// advances or changes", per the task that reintroduced this.
  ///
  /// Best-effort: a failed resolve here must never surface as a playback
  /// error for the *current* track (it hasn't been requested by the user
  /// yet) - [skipNext] simply falls back to resolving on demand when no warm
  /// prefetch is available.
  Future<void> _prefetchNext() async {
    final nextIndex = _currentIndex + 1;
    if (nextIndex >= _queue.length) {
      _prefetchedResolution = null;
      await _engine.setNextSource(null);
      return;
    }
    final nextItem = _queue[nextIndex];
    try {
      await _ensureSession();
      final resolution = await _sessionRepository.play(
        sessionId: _sessionId!,
        trackId: nextItem.trackId,
      );
      _prefetchedResolution = resolution;
      await _engine.setNextSource(
        AudioSource(id: resolution.trackId, uri: resolution.streamUrl),
      );
    } catch (_) {
      // Best-effort - see method doc. Drop any stale cached resolution
      // rather than risk skipNext() reusing a resolution for the wrong
      // track; skipNext() falls back to an on-demand resolve when this is
      // null.
      _prefetchedResolution = null;
    }
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
      await _prefetchNext();
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
      // Reuse the resolution _prefetchNext() already warmed for this exact
      // track (frontend-flutter.md section 2.2/6 skip-latency target) - only
      // falls back to a fresh `next()` round-trip when no matching prefetch
      // landed in time (e.g. this is the first skip, or the prefetch call
      // failed/hasn't resolved yet).
      final warm = _prefetchedResolution;
      final resolution = (warm != null && warm.trackId == item.trackId)
          ? warm
          : await _sessionRepository.next(sessionId: _sessionId!);
      _prefetchedResolution = null;
      await _loadAndPlay(resolution);
      await _prefetchNext();
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
      await _prefetchNext();
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
