import 'package:library_domain/library_domain.dart';
import 'package:test/test.dart';

import '../../support/fake_library_repository.dart';

void main() {
  test('forwards the track id to the repository', () async {
    final repository = FakeLibraryRepository();
    final useCase = SaveTrackUseCase(repository);

    await useCase('track-1');

    expect(repository.saveTrackCalls, 1);
    expect(repository.lastSavedTrackId, 'track-1');
  });
}
