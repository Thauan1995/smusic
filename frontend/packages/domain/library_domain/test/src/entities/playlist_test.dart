import 'package:library_domain/library_domain.dart';
import 'package:test/test.dart';

void main() {
  test('equal when all fields match (non-identical instances)', () {
    final a = Playlist(id: '1', name: 'Chill', isPublic: false);
    final b = Playlist(id: '1', name: 'Chill', isPublic: false);
    expect(a, b);
    expect(a.hashCode, b.hashCode);
  });

  test('not equal when a field differs', () {
    final a = Playlist(id: '1', name: 'Chill', isPublic: false);
    final b = Playlist(id: '1', name: 'Chill', isPublic: true);
    expect(a == b, isFalse);
  });
}
