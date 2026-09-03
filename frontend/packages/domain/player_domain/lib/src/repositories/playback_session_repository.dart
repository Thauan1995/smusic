import '../entities/playback_track_resolution.dart';

/// Implemented by `player_data` against backend-go.md section 4's
/// "Reprodução (play / seek / fila)" endpoints. This keeps the *backend's*
/// notion of playback state (used for Spotify-Connect-style cross-device
/// sync and play-event billing/analytics per backend-go.md section 4) in
/// sync with what `NativeAudioEngine` is actually doing locally -
/// `PlaybackQueueController`'s implementation in `player_data` is the one
/// place that drives both.
abstract interface class PlaybackSessionRepository {
  /// `POST /v1/playback/sessions` - returns the new session id.
  Future<String> createSession({required String deviceId});

  /// `POST /v1/playback/sessions/{id}/play` - resolves [trackId] to a
  /// signed stream URL and marks it as the now-playing track server-side.
  Future<PlaybackTrackResolution> play({
    required String sessionId,
    required String trackId,
    int? positionMs,
  });

  /// `POST /v1/playback/sessions/{id}/pause`.
  Future<void> pause({required String sessionId});

  /// `POST /v1/playback/sessions/{id}/seek`.
  Future<void> seek({required String sessionId, required int positionMs});

  /// `POST /v1/playback/sessions/{id}/next` - server-side "what's next in
  /// the queue" resolution (so skip-next behaves consistently across
  /// devices sharing a session).
  Future<PlaybackTrackResolution> next({required String sessionId});

  /// `POST /v1/playback/sessions/{id}/queue`.
  Future<void> enqueue({
    required String sessionId,
    required List<String> trackIds,
    String position = 'end',
  });
}
