import 'package:library_domain/library_domain.dart';
import 'package:riverpod/riverpod.dart';
import 'package:test/test.dart';

import '../support/fake_library_repository.dart';

SearchResultItem _item(String id) => SearchResultItem(
      id: id,
      type: SearchResultType.track,
      title: 'Song $id',
      subtitle: 'Artist',
    );

void main() {
  late FakeLibraryRepository repository;
  late ProviderContainer container;

  setUp(() {
    repository = FakeLibraryRepository();
    container = ProviderContainer(
      overrides: [libraryRepositoryProvider.overrideWithValue(repository)],
    );
    addTearDown(container.dispose);
  });

  test('initial state is empty with no query', () async {
    final state = await container.read(searchProvider.future);
    expect(state.query, '');
    expect(state.result.items, isEmpty);
  });

  test('blank query clears results immediately, without debounce/network', () {
    final notifier = container.read(searchProvider.notifier);
    notifier.onQueryChanged('   ');

    final state = container.read(searchProvider).value;
    expect(state?.result.items, isEmpty);
    expect(repository.searchCalls, 0);
  });

  test('non-blank query runs after the debounce delay', () async {
    await container.read(searchProvider.future);
    repository.searchPages.add(
      Paginated(items: [_item('1')], nextCursor: null),
    );

    final notifier = container.read(searchProvider.notifier);
    notifier.onQueryChanged(
      'daft punk',
      debounce: const Duration(milliseconds: 10),
    );

    // Immediately after: debounce hasn't fired yet.
    expect(repository.searchCalls, 0);

    await Future<void>.delayed(const Duration(milliseconds: 40));

    expect(repository.searchCalls, 1);
    expect(repository.lastSearchQuery, 'daft punk');
    expect(container.read(searchProvider).value?.result.items, hasLength(1));
  });

  test('rapid successive queries cancel the earlier debounce timer', () async {
    await container.read(searchProvider.future);
    repository.searchPages.add(Paginated(items: [_item('final')], nextCursor: null));

    final notifier = container.read(searchProvider.notifier);
    notifier.onQueryChanged('da', debounce: const Duration(milliseconds: 30));
    notifier.onQueryChanged('daft', debounce: const Duration(milliseconds: 30));
    notifier.onQueryChanged(
      'daft punk',
      debounce: const Duration(milliseconds: 10),
    );

    await Future<void>.delayed(const Duration(milliseconds: 60));

    // Only the last query should have actually reached the repository.
    expect(repository.searchCalls, 1);
    expect(repository.lastSearchQuery, 'daft punk');
  });

  test('a stale (slower) response is dropped in favor of a newer one', () async {
    await container.read(searchProvider.future);
    repository.searchDelays = [
      const Duration(milliseconds: 50), // response to the FIRST query
      Duration.zero, // response to the SECOND query
    ];
    repository.searchPages.addAll([
      Paginated(items: [_item('old')], nextCursor: null),
      Paginated(items: [_item('new')], nextCursor: null),
    ]);

    final notifier = container.read(searchProvider.notifier);
    // First query with a tiny debounce so it starts (and its network call
    // begins) well before the second one.
    notifier.onQueryChanged('old query', debounce: const Duration(milliseconds: 1));
    await Future<void>.delayed(const Duration(milliseconds: 10));
    notifier.onQueryChanged('new query', debounce: const Duration(milliseconds: 1));

    await Future<void>.delayed(const Duration(milliseconds: 80));

    final state = container.read(searchProvider).value;
    expect(state?.query, 'new query');
    expect(state?.result.items.single.id, 'new');
  });

  test('search failure surfaces as AsyncError', () async {
    await container.read(searchProvider.future);
    repository.throwOnSearch = const LibraryException(LibraryExceptionKind.network);

    final notifier = container.read(searchProvider.notifier);
    notifier.onQueryChanged('x', debounce: Duration.zero);
    await Future<void>.delayed(const Duration(milliseconds: 20));

    expect(container.read(searchProvider).hasError, isTrue);
  });

  group('loadMore', () {
    test('appends the next page and updates the cursor', () async {
      await container.read(searchProvider.future);
      repository.searchPages.addAll([
        Paginated(items: [_item('1')], nextCursor: 'c2'),
        Paginated(items: [_item('2')], nextCursor: null),
      ]);

      final notifier = container.read(searchProvider.notifier);
      notifier.onQueryChanged('x', debounce: Duration.zero);
      await Future<void>.delayed(const Duration(milliseconds: 20));

      await notifier.loadMore();

      final state = container.read(searchProvider).value;
      expect(state?.result.items.map((e) => e.id), ['1', '2']);
      expect(state?.result.hasMore, isFalse);
      expect(repository.lastSearchCursor, 'c2');
    });

    test('is a no-op when there is no current page', () async {
      final notifier = container.read(searchProvider.notifier);
      await notifier.loadMore();
      expect(repository.searchCalls, 0);
    });

    test('is a no-op when the current page has no more results', () async {
      await container.read(searchProvider.future);
      repository.searchPages.add(Paginated(items: [_item('1')], nextCursor: null));
      final notifier = container.read(searchProvider.notifier);
      notifier.onQueryChanged('x', debounce: Duration.zero);
      await Future<void>.delayed(const Duration(milliseconds: 20));

      final callsBefore = repository.searchCalls;
      await notifier.loadMore();
      expect(repository.searchCalls, callsBefore);
    });

    test('a second concurrent loadMore call is a no-op while one is in flight', () async {
      await container.read(searchProvider.future);
      repository.searchPages.addAll([
        Paginated(items: [_item('1')], nextCursor: 'c2'),
        Paginated(items: [_item('2')], nextCursor: null),
      ]);
      repository.searchDelays = [Duration.zero, const Duration(milliseconds: 30)];

      final notifier = container.read(searchProvider.notifier);
      notifier.onQueryChanged('x', debounce: Duration.zero);
      await Future<void>.delayed(const Duration(milliseconds: 20));

      final first = notifier.loadMore();
      final second = notifier.loadMore(); // should see isLoadingMore=true and bail
      await Future.wait([first, second]);

      expect(repository.searchCalls, 2); // initial + exactly one loadMore
    });

    test('keeps existing items and clears the loading flag on failure', () async {
      await container.read(searchProvider.future);
      repository.searchPages.add(Paginated(items: [_item('1')], nextCursor: 'c2'));
      final notifier = container.read(searchProvider.notifier);
      notifier.onQueryChanged('x', debounce: Duration.zero);
      await Future<void>.delayed(const Duration(milliseconds: 20));

      repository.throwOnSearch = const LibraryException(LibraryExceptionKind.network);
      await notifier.loadMore();

      final state = container.read(searchProvider).value;
      expect(state?.result.items, hasLength(1));
      expect(state?.isLoadingMore, isFalse);
    });
  });
}
