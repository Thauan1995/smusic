import 'package:player_domain/player_domain.dart';

/// Mapping between backend-go.md section 4's "Reprodução" JSON shapes and
/// `player_domain` entities.
///
/// ASSUMPTIONS (backend contract gaps, flagged for the backend specialist
/// per frontend/README.md):
/// - `POST .../play`'s response (`{ stream_url, expires_at }`) doesn't echo
///   back `track_id`; [trackResolutionFromPlayResponse] takes the
///   already-known [trackId] as a parameter instead of reading it from JSON.
/// - `POST .../next`'s response (`{ track_id, stream_url }`) has no
///   `expires_at` field, unlike `.../play`'s. `media-edge-service` signs
///   URLs for "5-10 min" per backend-go.md section 2, so
///   [trackResolutionFromNextResponse] falls back to `now + 5 minutes`
///   (the conservative end of that stated window) when the field is absent.
class PlaybackDtos {
  const PlaybackDtos._(); // coverage:ignore-line

  static PlaybackTrackResolution trackResolutionFromPlayResponse(
    Map<String, dynamic> json, {
    required String trackId,
    DateTime? now,
  }) {
    return PlaybackTrackResolution(
      trackId: trackId,
      streamUrl: Uri.parse(json['stream_url'] as String),
      expiresAt: _parseExpiresAt(json['expires_at'], now: now),
    );
  }

  static PlaybackTrackResolution trackResolutionFromNextResponse(
    Map<String, dynamic> json, {
    DateTime? now,
  }) {
    return PlaybackTrackResolution(
      trackId: json['track_id'] as String,
      streamUrl: Uri.parse(json['stream_url'] as String),
      expiresAt: _parseExpiresAt(json['expires_at'], now: now),
    );
  }

  static DateTime _parseExpiresAt(Object? raw, {DateTime? now}) {
    if (raw is String) return DateTime.parse(raw);
    return (now ?? DateTime.now()).add(const Duration(minutes: 5));
  }
}
