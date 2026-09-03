import 'package:library_domain/library_domain.dart';
import 'package:test/test.dart';

void main() {
  test('equal when all fields match (non-identical instances)', () {
    final a = SearchResultItem(
      id: '1',
      type: SearchResultType.track,
      title: 'Song',
      subtitle: 'Artist',
    );
    final b = SearchResultItem(
      id: '1',
      type: SearchResultType.track,
      title: 'Song',
      subtitle: 'Artist',
    );
    expect(a, b);
    expect(a.hashCode, b.hashCode);
  });

  test('not equal when type differs', () {
    final a = SearchResultItem(
      id: '1',
      type: SearchResultType.track,
      title: 'Song',
      subtitle: 'Artist',
    );
    final b = SearchResultItem(
      id: '1',
      type: SearchResultType.album,
      title: 'Song',
      subtitle: 'Artist',
    );
    expect(a == b, isFalse);
  });

  test('SearchResultType has all four kinds', () {
    expect(SearchResultType.values, hasLength(4));
  });
}
