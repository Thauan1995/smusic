import 'package:library_domain/library_domain.dart';
import 'package:test/test.dart';

void main() {
  test('carries id, title and tracks', () {
    final track = Track(
      id: 't1',
      title: 'Song',
      artistName: 'Artist',
      albumName: 'Album',
      durationMs: 1000,
    );
    final album = Album(id: 'a1', title: 'Album', tracks: [track]);
    expect(album.id, 'a1');
    expect(album.title, 'Album');
    expect(album.tracks, [track]);
  });
}
