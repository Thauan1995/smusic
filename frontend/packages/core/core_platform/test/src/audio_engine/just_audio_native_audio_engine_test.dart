import 'package:core_platform/src/audio_engine/native_audio_engine.dart';
import 'package:core_platform/src/audio_engine/just_audio_native_audio_engine.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:just_audio/just_audio.dart' as ja;

void main() {
  group('mapJustAudioProcessingState', () {
    final cases = <ja.ProcessingState, PlaybackEngineState>{
      ja.ProcessingState.idle: PlaybackEngineState.idle,
      ja.ProcessingState.loading: PlaybackEngineState.loading,
      ja.ProcessingState.buffering: PlaybackEngineState.buffering,
      ja.ProcessingState.ready: PlaybackEngineState.ready,
      ja.ProcessingState.completed: PlaybackEngineState.completed,
    };

    for (final entry in cases.entries) {
      test('maps ${entry.key} to ${entry.value}', () {
        expect(mapJustAudioProcessingState(entry.key), entry.value);
      });
    }
  });

  group('AudioSource', () {
    test('carries id, uri and headers', () {
      final source = AudioSource(
        id: 'track-1',
        uri: Uri.parse('https://cdn.example.com/track-1.m3u8'),
        headers: const {'X-Test': '1'},
      );
      expect(source.id, 'track-1');
      expect(source.uri.host, 'cdn.example.com');
      expect(source.headers, {'X-Test': '1'});
    });
  });

  group('PlaybackPositionEvent', () {
    test('stores position/buffered/duration', () {
      // Deliberately not `const` - a const invocation is canonicalized at
      // compile time and never shows as "hit" by line coverage tooling,
      // even though this is exactly the constructor under test.
      // ignore: prefer_const_constructors
      final event = PlaybackPositionEvent(
        position: const Duration(seconds: 10),
        bufferedPosition: const Duration(seconds: 20),
        duration: const Duration(minutes: 3),
      );
      expect(event.position, const Duration(seconds: 10));
      expect(event.bufferedPosition, const Duration(seconds: 20));
      expect(event.duration, const Duration(minutes: 3));
    });
  });

  group('AudioEngineException', () {
    test('toString includes message', () {
      // Not `const` - see comment above on PlaybackPositionEvent's test.
      // ignore: prefer_const_constructors
      final exception = AudioEngineException('boom', cause: 'codec error');
      expect(exception.toString(), contains('boom'));
      expect(exception.cause, 'codec error');
    });
  });
}
