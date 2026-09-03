import 'dart:async';

import 'package:riverpod/riverpod.dart';

import 'entities/paginated.dart';
import 'entities/search_result_item.dart';
import 'library_playlists_notifier.dart';
import 'usecases/search_catalog_use_case.dart';

final searchCatalogUseCaseProvider = Provider(
  (ref) => SearchCatalogUseCase(ref.watch(libraryRepositoryProvider)),
);

/// Immutable view of the current search box + its results page, per
/// frontend-flutter.md section 3.3.
class SearchState {
  const SearchState({
    this.query = '',
    this.result = const Paginated<SearchResultItem>.empty(),
    this.isLoadingMore = false,
  });

  final String query;
  final Paginated<SearchResultItem> result;
  final bool isLoadingMore;

  SearchState copyWith({
    String? query,
    Paginated<SearchResultItem>? result,
    bool? isLoadingMore,
  }) {
    return SearchState(
      query: query ?? this.query,
      result: result ?? this.result,
      isLoadingMore: isLoadingMore ?? this.isLoadingMore,
    );
  }
}

/// `SearchNotifier` per frontend-flutter.md section 3.3: debounces the
/// remote catalog search by [debounce] (default 300ms), and guards against
/// out-of-order responses (a slow earlier request completing after a newer
/// one) with a monotonic generation counter rather than `core_networking`'s
/// `CancelToken` - domain must not depend on the networking package, so
/// staleness is detected here instead of the request being physically
/// cancelled (`library_data` may additionally cancel in-flight dio requests
/// as a network-efficiency optimization; that is invisible to this class).
///
/// The spec's "instant local results before remote results arrive" (search
/// against a local library/cache index) is explicitly deferred - Fatia 1
/// has no local search index to query. Documented as a TODO in
/// frontend/README.md.
class SearchNotifier extends AsyncNotifier<SearchState> {
  Timer? _debounceTimer;
  int _generation = 0;

  @override
  FutureOr<SearchState> build() {
    ref.onDispose(() {
      _debounceTimer?.cancel();
    });
    return const SearchState();
  }

  void onQueryChanged(
    String query, {
    Duration debounce = const Duration(milliseconds: 300),
  }) {
    _debounceTimer?.cancel();
    final generation = ++_generation;

    final trimmed = query.trim();
    if (trimmed.isEmpty) {
      state = AsyncData(SearchState(query: query));
      return;
    }

    _debounceTimer = Timer(debounce, () => _runSearch(query, generation));
  }

  Future<void> _runSearch(String query, int generation) async {
    state = const AsyncLoading<SearchState>().copyWithPrevious(state);
    final result = await AsyncValue.guard(() {
      final useCase = ref.read(searchCatalogUseCaseProvider);
      return useCase(query: query);
    });

    // A newer keystroke started a newer search while this one was in
    // flight - drop this stale response rather than overwriting fresher
    // state with older data.
    if (generation != _generation) return;

    state = result.when(
      data: (page) => AsyncData(SearchState(query: query, result: page)),
      error: (error, stackTrace) => AsyncError(error, stackTrace),
      // AsyncValue.guard() only ever produces AsyncData or AsyncError - this
      // branch is unreachable in practice but required to satisfy .when()'s
      // exhaustiveness. Documented exclusion per
      // docs/architecture/00-overview.md section 2.
      loading: () => state, // coverage:ignore-line
    );
  }

  Future<void> loadMore() async {
    final current = state.valueOrNull;
    if (current == null || !current.result.hasMore || current.isLoadingMore) {
      return;
    }

    state = AsyncData(current.copyWith(isLoadingMore: true));
    final useCase = ref.read(searchCatalogUseCaseProvider);
    try {
      final nextPage = await useCase(
        query: current.query,
        cursor: current.result.nextCursor,
      );
      state = AsyncData(
        SearchState(
          query: current.query,
          result: Paginated(
            items: [...current.result.items, ...nextPage.items],
            nextCursor: nextPage.nextCursor,
          ),
        ),
      );
    } catch (_) {
      // Load-more failures keep the already-loaded page visible rather than
      // clearing results - only the loading flag resets so the UI can offer
      // a retry (frontend-flutter.md section 3.4 spirit: never regress a
      // populated list to an error/empty state over a pagination hiccup).
      state = AsyncData(current.copyWith(isLoadingMore: false));
    }
  }
}

final searchProvider =
    AsyncNotifierProvider<SearchNotifier, SearchState>(SearchNotifier.new);
