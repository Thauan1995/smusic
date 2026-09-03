/// Result of asking the backend to resolve a track to a playable URL
/// (`POST /v1/playback/sessions/{id}/play` / `.../next`, backend-go.md
/// section 4: "stream_url é sempre uma URL assinada de curta duração").
class PlaybackTrackResolution {
  const PlaybackTrackResolution({
    required this.trackId,
    required this.streamUrl,
    required this.expiresAt,
  });

  final String trackId;
  final Uri streamUrl;
  final DateTime expiresAt;
}
