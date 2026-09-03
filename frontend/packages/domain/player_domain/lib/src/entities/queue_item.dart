/// A track's display data as it flows through the playback queue. Kept
/// separate from `library_domain`'s `Track` entity so `player_domain` never
/// depends on `library_domain` (a feature-domain package importing another
/// feature-domain package would break the feature-first isolation described
/// in frontend-flutter.md section 1.2) - `player_data` is responsible for
/// mapping a `Track` (or a raw catalog DTO) into a `QueueItem` when a track
/// is added to the queue.
class QueueItem {
  const QueueItem({
    required this.trackId,
    required this.title,
    required this.artistName,
    required this.durationMs,
  });

  final String trackId;
  final String title;
  final String artistName;
  final int durationMs;

  Duration get duration => Duration(milliseconds: durationMs);

  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      other is QueueItem &&
          other.trackId == trackId &&
          other.title == title &&
          other.artistName == artistName &&
          other.durationMs == durationMs;

  @override
  int get hashCode => Object.hash(trackId, title, artistName, durationMs);
}
