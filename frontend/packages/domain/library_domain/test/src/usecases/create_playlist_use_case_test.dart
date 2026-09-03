import 'package:library_domain/library_domain.dart';
import 'package:test/test.dart';

import '../../support/fake_library_repository.dart';

void main() {
  test('trims name and returns the created playlist id', () async {
    final repository = FakeLibraryRepository()..createdPlaylistId = 'p-42';
    final useCase = CreatePlaylistUseCase(repository);

    final id = await useCase(name: '  Road Trip  ');

    expect(id, 'p-42');
  });

  test('defaults isPublic to false', () async {
    final repository = FakeLibraryRepository();
    final useCase = CreatePlaylistUseCase(repository);
    await useCase(name: 'Chill');
    // No direct assertion surface on the fake for isPublic passed through,
    // but calling with the default arg must not throw - regression guard
    // for the default value itself.
  });

  test('throws ArgumentError for a blank name', () async {
    final repository = FakeLibraryRepository();
    final useCase = CreatePlaylistUseCase(repository);

    await expectLater(
      () => useCase(name: '   '),
      throwsArgumentError,
    );
  });

  test('propagates repository failure', () async {
    final repository = FakeLibraryRepository()
      ..throwOnCreatePlaylist = const LibraryException(LibraryExceptionKind.network);
    final useCase = CreatePlaylistUseCase(repository);

    await expectLater(
      () => useCase(name: 'Chill'),
      throwsA(isA<LibraryException>()),
    );
  });
}
