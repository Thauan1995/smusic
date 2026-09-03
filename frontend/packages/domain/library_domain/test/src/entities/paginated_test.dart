import 'package:library_domain/library_domain.dart';
import 'package:test/test.dart';

void main() {
  test('hasMore is true when nextCursor is set', () {
    final page = Paginated(items: const [1, 2], nextCursor: 'abc');
    expect(page.hasMore, isTrue);
  });

  test('hasMore is false when nextCursor is null', () {
    final page = Paginated(items: const [1, 2], nextCursor: null);
    expect(page.hasMore, isFalse);
  });

  test('empty() has no items and no cursor', () {
    final page = Paginated<int>.empty();
    expect(page.items, isEmpty);
    expect(page.hasMore, isFalse);
  });
}
