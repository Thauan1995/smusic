import 'package:core_design_system/core_design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:library_domain/library_domain.dart';
import 'package:library_ui/library_ui.dart';

import '../../support/fake_library_repository.dart';

Widget _wrap(Widget child, FakeLibraryRepository repository) {
  return ProviderScope(
    overrides: [libraryRepositoryProvider.overrideWithValue(repository)],
    child: MaterialApp(theme: SmusicTheme.light(), home: child),
  );
}

SearchResultItem _item(String id) => SearchResultItem(
      id: id,
      type: SearchResultType.track,
      title: 'Song $id',
      subtitle: 'Artist',
    );

/// Types [text] then advances past both `SearchNotifier`'s 300ms debounce
/// and `FakeLibraryRepository.search()`'s 200ms artificial network delay
/// (500ms total), in one deliberate jump.
///
/// `pumpAndSettle()` alone is not reliable here: its default 100ms-step
/// loop stops as soon as two consecutive steps produce no new frame, which
/// can trigger *before* a not-yet-fired `Timer` (the debounce, or this
/// delay) ever gets a chance to run - it has no way to know one is still
/// pending. A single explicit `pump(550ms)` crosses both timers in one
/// step, guaranteeing they fire within it.
Future<void> _typeAndSettle(WidgetTester tester, String text) async {
  await tester.enterText(find.byKey(const Key('search_field')), text);
  await tester.pump(const Duration(milliseconds: 550));
  await tester.pumpAndSettle();
}

void main() {
  testWidgets('shows a prompt before any query is typed', (tester) async {
    await tester.pumpWidget(_wrap(const SearchScreen(), FakeLibraryRepository()));
    await tester.pumpAndSettle();

    expect(find.textContaining('Search for a track'), findsOneWidget);
  });

  testWidgets('typing runs a debounced search and shows results', (tester) async {
    final repository = FakeLibraryRepository()
      ..searchPages.add(Paginated(items: [_item('1')], nextCursor: null));
    await tester.pumpWidget(_wrap(const SearchScreen(), repository));
    await tester.pumpAndSettle();

    await tester.enterText(find.byKey(const Key('search_field')), 'daft punk');
    // Still within the debounce window - no request yet.
    await tester.pump();
    expect(repository.searchCalls, 0);

    await tester.pump(const Duration(milliseconds: 550));
    await tester.pumpAndSettle();

    expect(find.text('Song 1'), findsOneWidget);
    expect(repository.lastSearchQuery, 'daft punk');
  });

  testWidgets('shows a skeleton while a search is in flight', (tester) async {
    final repository = FakeLibraryRepository()
      ..searchPages.add(Paginated(items: [_item('1')], nextCursor: null));
    await tester.pumpWidget(_wrap(const SearchScreen(), repository));
    await tester.pumpAndSettle();

    await tester.enterText(find.byKey(const Key('search_field')), 'x');
    // Just past the 300ms debounce boundary, well before the fake
    // repository's own 200ms artificial network delay resolves.
    await tester.pump(const Duration(milliseconds: 310));

    expect(find.byType(TrackListSkeleton), findsOneWidget);

    await tester.pump(const Duration(milliseconds: 250));
    await tester.pumpAndSettle();
  });

  testWidgets('shows a no-results state', (tester) async {
    final repository = FakeLibraryRepository();
    await tester.pumpWidget(_wrap(const SearchScreen(), repository));
    await tester.pumpAndSettle();

    await _typeAndSettle(tester, 'zzz');

    expect(find.textContaining('No results for'), findsOneWidget);
  });

  testWidgets('shows an error state with a working retry action', (tester) async {
    final repository = FakeLibraryRepository()..throwOnSearch = Exception('boom');
    await tester.pumpWidget(_wrap(const SearchScreen(), repository));
    await tester.pumpAndSettle();

    await _typeAndSettle(tester, 'x');

    expect(find.text('Search failed. Please try again.'), findsOneWidget);

    repository.throwOnSearch = null;
    repository.searchPages.add(Paginated(items: [_item('1')], nextCursor: null));
    await tester.tap(find.text('Retry'));
    await tester.pumpAndSettle();

    expect(find.text('Song 1'), findsOneWidget);
  });

  testWidgets('shows a Load more row and appends the next page', (tester) async {
    final repository = FakeLibraryRepository()
      ..searchPages.addAll([
        Paginated(items: [_item('1')], nextCursor: 'c2'),
        Paginated(items: [_item('2')], nextCursor: null),
      ]);
    await tester.pumpWidget(_wrap(const SearchScreen(), repository));
    await tester.pumpAndSettle();

    await _typeAndSettle(tester, 'x');

    expect(find.byKey(const Key('search_load_more_button')), findsOneWidget);

    await tester.tap(find.byKey(const Key('search_load_more_button')));
    await tester.pumpAndSettle();

    expect(find.text('Song 1'), findsOneWidget);
    expect(find.text('Song 2'), findsOneWidget);
    expect(find.byKey(const Key('search_load_more_button')), findsNothing);
  });

  testWidgets('tapping a result invokes onResultTap', (tester) async {
    final repository = FakeLibraryRepository()
      ..searchPages.add(Paginated(items: [_item('1')], nextCursor: null));
    SearchResultItem? tapped;
    await tester.pumpWidget(_wrap(
      SearchScreen(onResultTap: (item) => tapped = item),
      repository,
    ));
    await tester.pumpAndSettle();

    await _typeAndSettle(tester, 'x');

    await tester.tap(find.text('Song 1'));
    expect(tapped?.id, '1');
  });

  testWidgets('clearing the field goes back to the prompt state immediately', (tester) async {
    final repository = FakeLibraryRepository()
      ..searchPages.add(Paginated(items: [_item('1')], nextCursor: null));
    await tester.pumpWidget(_wrap(const SearchScreen(), repository));
    await tester.pumpAndSettle();

    await _typeAndSettle(tester, 'x');
    expect(find.text('Song 1'), findsOneWidget);

    await tester.enterText(find.byKey(const Key('search_field')), '');
    await tester.pump();

    expect(find.textContaining('Search for a track'), findsOneWidget);
  });
}
