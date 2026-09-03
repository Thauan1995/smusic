import 'package:library_data/library_data.dart';
import 'package:library_domain/library_domain.dart';
import 'package:test/test.dart';

void main() {
  group('trackFromJson', () {
    test('maps all fields', () {
      final track = LibraryDtos.trackFromJson({
        'id': 't1',
        'title': 'Song',
        'artist': 'Artist',
        'album': 'Album',
        'duration_ms': 210000,
      });
      expect(track.id, 't1');
      expect(track.artistName, 'Artist');
      expect(track.albumName, 'Album');
      expect(track.durationMs, 210000);
    });
  });

  group('albumFromJson', () {
    test('maps id, title and nested tracks', () {
      final album = LibraryDtos.albumFromJson({
        'id': 'a1',
        'title': 'Album',
        'tracks': [
          {
            'id': 't1',
            'title': 'Song',
            'artist': 'Artist',
            'album': 'Album',
            'duration_ms': 1000,
          },
        ],
      });
      expect(album.tracks, hasLength(1));
      expect(album.tracks.first.id, 't1');
    });

    test('defaults to an empty track list when tracks is missing', () {
      final album = LibraryDtos.albumFromJson({'id': 'a1', 'title': 'Album'});
      expect(album.tracks, isEmpty);
    });
  });

  group('playlistFromJson / playlistsFromJson', () {
    test('maps a single playlist', () {
      final playlist = LibraryDtos.playlistFromJson({
        'id': 'p1',
        'name': 'Chill',
        'is_public': true,
      });
      expect(playlist.id, 'p1');
      expect(playlist.isPublic, isTrue);
    });

    test('defaults isPublic to false when missing', () {
      final playlist = LibraryDtos.playlistFromJson({'id': 'p1', 'name': 'Chill'});
      expect(playlist.isPublic, isFalse);
    });

    test('maps the playlists[] envelope', () {
      final playlists = LibraryDtos.playlistsFromJson({
        'playlists': [
          {'id': 'p1', 'name': 'Chill', 'is_public': false},
        ],
      });
      expect(playlists, hasLength(1));
    });

    test('defaults to an empty list when playlists key is missing', () {
      expect(LibraryDtos.playlistsFromJson({}), isEmpty);
    });
  });

  group('searchResultTypeFromJson', () {
    test('parses each known type', () {
      expect(LibraryDtos.searchResultTypeFromJson('track'), SearchResultType.track);
      expect(LibraryDtos.searchResultTypeFromJson('album'), SearchResultType.album);
      expect(LibraryDtos.searchResultTypeFromJson('artist'), SearchResultType.artist);
      expect(LibraryDtos.searchResultTypeFromJson('playlist'), SearchResultType.playlist);
    });

    test('falls back to track for an unknown type', () {
      expect(LibraryDtos.searchResultTypeFromJson('bogus'), SearchResultType.track);
    });
  });

  group('searchResultItemFromJson / searchResultsFromJson', () {
    test('maps a single result item', () {
      final item = LibraryDtos.searchResultItemFromJson({
        'id': '1',
        'type': 'track',
        'title': 'Song',
        'subtitle': 'Artist',
      });
      expect(item.type, SearchResultType.track);
      expect(item.subtitle, 'Artist');
    });

    test('defaults subtitle to empty string when missing', () {
      final item = LibraryDtos.searchResultItemFromJson({
        'id': '1',
        'type': 'track',
        'title': 'Song',
      });
      expect(item.subtitle, '');
    });

    test('maps the results[]/next_cursor envelope', () {
      final page = LibraryDtos.searchResultsFromJson({
        'results': [
          {'id': '1', 'type': 'track', 'title': 'Song'},
        ],
        'next_cursor': 'c2',
      });
      expect(page.items, hasLength(1));
      expect(page.nextCursor, 'c2');
    });

    test('defaults to an empty page when keys are missing', () {
      final page = LibraryDtos.searchResultsFromJson({});
      expect(page.items, isEmpty);
      expect(page.nextCursor, isNull);
    });
  });
}
