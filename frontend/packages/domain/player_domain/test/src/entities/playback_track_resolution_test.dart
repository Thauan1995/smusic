import 'package:player_domain/player_domain.dart';
import 'package:test/test.dart';

void main() {
  test('carries trackId, streamUrl and expiresAt', () {
    final expiresAt = DateTime(2026, 1, 1);
    final resolution = PlaybackTrackResolution(
      trackId: 't1',
      streamUrl: Uri.parse('https://cdn.example.com/t1.m3u8'),
      expiresAt: expiresAt,
    );
    expect(resolution.trackId, 't1');
    expect(resolution.streamUrl.host, 'cdn.example.com');
    expect(resolution.expiresAt, expiresAt);
  });
}
