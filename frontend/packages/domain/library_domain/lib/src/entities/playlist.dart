/// Maps to entries of `GET /v1/library/me/playlists` (backend-go.md
/// section 4).
class Playlist {
  const Playlist({
    required this.id,
    required this.name,
    required this.isPublic,
  });

  final String id;
  final String name;
  final bool isPublic;

  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      other is Playlist &&
          other.id == id &&
          other.name == name &&
          other.isPublic == isPublic;

  @override
  int get hashCode => Object.hash(id, name, isPublic);
}
