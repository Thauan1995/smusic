import 'track.dart';

/// Maps to `GET /v1/catalog/albums/{id}` (backend-go.md section 4).
class Album {
  const Album({required this.id, required this.title, required this.tracks});

  final String id;
  final String title;
  final List<Track> tracks;
}
