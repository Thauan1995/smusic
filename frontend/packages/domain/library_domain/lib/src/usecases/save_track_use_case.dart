import '../repositories/library_repository.dart';

class SaveTrackUseCase {
  const SaveTrackUseCase(this._repository);

  final LibraryRepository _repository;

  Future<void> call(String trackId) => _repository.saveTrack(trackId);
}
