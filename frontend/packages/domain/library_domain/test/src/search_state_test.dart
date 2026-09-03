import 'package:library_domain/library_domain.dart';
import 'package:test/test.dart';

void main() {
  group('SearchState.copyWith', () {
    test('overrides only the given fields', () {
      const original = SearchState(query: 'a');
      final copy = original.copyWith(query: 'b');
      expect(copy.query, 'b');
      expect(copy.result.items, original.result.items);
      expect(copy.isLoadingMore, original.isLoadingMore);
    });

    test('keeps isLoadingMore unchanged when omitted', () {
      const original = SearchState(query: 'a', isLoadingMore: true);
      final copy = original.copyWith(query: 'b');
      expect(copy.isLoadingMore, isTrue);
    });
  });
}
