/// Maps to `GET /v1/catalog/tracks/{id}` (backend-go.md section 4).
class Track {
  const Track({
    required this.id,
    required this.title,
    required this.artistName,
    required this.albumName,
    required this.durationMs,
  });

  final String id;
  final String title;
  final String artistName;
  final String albumName;
  final int durationMs;

  Duration get duration => Duration(milliseconds: durationMs);

  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      other is Track &&
          other.id == id &&
          other.title == title &&
          other.artistName == artistName &&
          other.albumName == albumName &&
          other.durationMs == durationMs;

  @override
  int get hashCode =>
      Object.hash(id, title, artistName, albumName, durationMs);
}
