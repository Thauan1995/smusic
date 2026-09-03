/// Platform-agnostic audio source description.
///
/// `player_data` builds this from a track's resolved stream URL (see
/// backend-go.md section 4, `POST /v1/playback/sessions/{id}/play`) without
/// `core_platform` ever needing to know about HTTP or the backend contract.
class AudioSource {
  const AudioSource({
    required this.id,
    required this.uri,
    this.headers,
  });

  /// Stable identifier for this source (matches `QueueItem.trackId` in
  /// `player_domain`). Used to correlate engine callbacks back to a track.
  final String id;

  /// Absolute URI to stream from (a signed CDN URL in production).
  final Uri uri;

  /// Optional extra HTTP headers (rarely needed; signed URLs carry auth in
  /// the query string per backend-go.md section 2).
  final Map<String, String>? headers;
}

/// Coarse engine playback state, independent of any specific native engine.
///
/// `player_data`'s `JustAudioPlaybackAdapter` is the single place that
/// translates `just_audio`'s `ProcessingState`/`playing` combination into
/// this enum (see frontend-flutter.md section 2.6).
enum PlaybackEngineState {
  idle,
  loading,
  buffering,
  ready,
  completed,
  error,
}

/// A position/duration tick emitted by the engine while a source is loaded.
class PlaybackPositionEvent {
  const PlaybackPositionEvent({
    required this.position,
    required this.bufferedPosition,
    this.duration,
  });

  final Duration position;
  final Duration bufferedPosition;
  final Duration? duration;
}

/// Raised by [NativeAudioEngine] implementations on unrecoverable playback
/// failures (codec error, network failure the engine could not retry past).
class AudioEngineException implements Exception {
  const AudioEngineException(this.message, {this.cause});

  final String message;
  final Object? cause;

  @override
  String toString() => 'AudioEngineException: $message';
}

/// Abstraction over the native audio engine (`just_audio` on both mobile and
/// web, see frontend-flutter.md section 1.3/2.1). `player_domain` and
/// `player_ui` never see this interface directly - only `player_data`'s
/// `JustAudioPlaybackAdapter` does, translating to/from the engine-agnostic
/// `PlayerState` machine defined in `player_domain`.
abstract interface class NativeAudioEngine {
  /// Loads [source] as the current source. Does not start playback.
  Future<void> load(AudioSource source);

  /// Sets the source to transition to gaplessly once the current source
  /// finishes (see frontend-flutter.md section 2.2/2.3, `setNextSource`).
  Future<void> setNextSource(AudioSource? source);

  Future<void> play();

  Future<void> pause();

  Future<void> seek(Duration position);

  /// Sets playback volume in the 0.0-1.0 range (used by crossfade in a
  /// future slice; exposed now so the interface does not need to change).
  Future<void> setVolume(double volume);

  Stream<PlaybackPositionEvent> get positionStream;

  Stream<PlaybackEngineState> get engineStateStream;

  /// Emits once when the currently loaded source finishes playing.
  Stream<void> get completionStream;

  Future<void> dispose();
}
