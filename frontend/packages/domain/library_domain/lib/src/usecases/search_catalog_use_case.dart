import '../entities/paginated.dart';
import '../entities/search_result_item.dart';
import '../repositories/library_repository.dart';

/// Wraps `LibraryRepository.search` with the one bit of real business logic
/// around search: a blank/whitespace-only query never hits the network and
/// returns an empty page immediately (frontend-flutter.md section 3.3 -
/// this is what lets `SearchNotifier` clear results instantly when the user
/// clears the search field, without waiting on debounce/network).
class SearchCatalogUseCase {
  const SearchCatalogUseCase(this._repository);

  final LibraryRepository _repository;

  Future<Paginated<SearchResultItem>> call({
    required String query,
    SearchResultType? type,
    int limit = 20,
    String? cursor,
  }) async {
    final trimmed = query.trim();
    if (trimmed.isEmpty) return const Paginated.empty();
    return _repository.search(
      query: trimmed,
      type: type,
      limit: limit,
      cursor: cursor,
    );
  }
}
