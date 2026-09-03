import '../repositories/library_repository.dart';

class CreatePlaylistUseCase {
  const CreatePlaylistUseCase(this._repository);

  final LibraryRepository _repository;

  Future<String> call({required String name, bool isPublic = false}) async {
    final trimmed = name.trim();
    if (trimmed.isEmpty) {
      throw ArgumentError.value(name, 'name', 'Playlist name cannot be blank');
    }
    return _repository.createPlaylist(name: trimmed, isPublic: isPublic);
  }
}
