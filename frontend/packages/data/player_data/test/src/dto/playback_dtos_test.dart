import 'package:flutter_test/flutter_test.dart';
import 'package:player_data/player_data.dart';

void main() {
  group('trackResolutionFromPlayResponse', () {
    test('uses the given trackId and parses expires_at when present', () {
      final resolution = PlaybackDtos.trackResolutionFromPlayResponse(
        {
          'stream_url': 'https://cdn.example.com/t1.m3u8',
          'expires_at': '2026-01-01T12:00:00.000Z',
        },
        trackId: 't1',
      );
      expect(resolution.trackId, 't1');
      expect(resolution.streamUrl.host, 'cdn.example.com');
      expect(resolution.expiresAt, DateTime.parse('2026-01-01T12:00:00.000Z'));
    });

    test('falls back to now + 5min when expires_at is absent', () {
      final now = DateTime(2026, 1, 1, 12);
      final resolution = PlaybackDtos.trackResolutionFromPlayResponse(
        {'stream_url': 'https://cdn.example.com/t1.m3u8'},
        trackId: 't1',
        now: now,
      );
      expect(resolution.expiresAt, now.add(const Duration(minutes: 5)));
    });
  });

  group('trackResolutionFromNextResponse', () {
    test('reads trackId from the response', () {
      final resolution = PlaybackDtos.trackResolutionFromNextResponse({
        'track_id': 't2',
        'stream_url': 'https://cdn.example.com/t2.m3u8',
      });
      expect(resolution.trackId, 't2');
    });

    test('falls back to now + 5min when expires_at is absent', () {
      final now = DateTime(2026, 1, 1, 12);
      final resolution = PlaybackDtos.trackResolutionFromNextResponse(
        {'track_id': 't2', 'stream_url': 'https://cdn.example.com/t2.m3u8'},
        now: now,
      );
      expect(resolution.expiresAt, now.add(const Duration(minutes: 5)));
    });
  });
}
