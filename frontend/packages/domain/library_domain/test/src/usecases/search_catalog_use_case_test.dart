import 'package:library_domain/library_domain.dart';
import 'package:test/test.dart';

import '../../support/fake_library_repository.dart';

void main() {
  test('blank query returns an empty page without hitting the repository', () async {
    final repository = FakeLibraryRepository();
    final useCase = SearchCatalogUseCase(repository);

    final page = await useCase(query: '   ');

    expect(page.items, isEmpty);
    expect(repository.searchCalls, 0);
  });

  test('non-blank query is trimmed and forwarded to the repository', () async {
    final repository = FakeLibraryRepository()
      ..searchPages.add(
        Paginated(
          items: [
            SearchResultItem(
              id: '1',
              type: SearchResultType.track,
              title: 'Song',
              subtitle: 'Artist',
            ),
          ],
          nextCursor: 'c1',
        ),
      );
    final useCase = SearchCatalogUseCase(repository);

    final page = await useCase(query: '  daft punk  ');

    expect(repository.lastSearchQuery, 'daft punk');
    expect(page.items, hasLength(1));
    expect(page.nextCursor, 'c1');
  });
}
