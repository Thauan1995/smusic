import 'package:player_domain/player_domain.dart';

class FakePlaybackSessionRepository implements PlaybackSessionRepository {
  String createdSessionId = 'session-1';
  PlaybackTrackResolution? playResult;
  PlaybackTrackResolution? nextResult;
  Object? throwOnPlay;
  Object? throwOnNext;
  Object? throwOnCreateSession;

  int pauseCalls = 0;
  int seekCalls = 0;
  int enqueueCalls = 0;
  String? lastPlayTrackId;
  int? lastSeekPositionMs;
  List<String>? lastEnqueuedTrackIds;

  @override
  Future<String> createSession({required String deviceId}) async {
    if (throwOnCreateSession != null) throw throwOnCreateSession!;
    return createdSessionId;
  }

  @override
  Future<PlaybackTrackResolution> play({
    required String sessionId,
    required String trackId,
    int? positionMs,
  }) async {
    lastPlayTrackId = trackId;
    if (throwOnPlay != null) throw throwOnPlay!;
    return playResult!;
  }

  @override
  Future<void> pause({required String sessionId}) async {
    pauseCalls++;
  }

  @override
  Future<void> seek({required String sessionId, required int positionMs}) async {
    seekCalls++;
    lastSeekPositionMs = positionMs;
  }

  @override
  Future<PlaybackTrackResolution> next({required String sessionId}) async {
    if (throwOnNext != null) throw throwOnNext!;
    return nextResult!;
  }

  @override
  Future<void> enqueue({
    required String sessionId,
    required List<String> trackIds,
    String position = 'end',
  }) async {
    enqueueCalls++;
    lastEnqueuedTrackIds = trackIds;
  }
}
