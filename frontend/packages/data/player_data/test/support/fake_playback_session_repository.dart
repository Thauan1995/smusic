import 'package:player_domain/player_domain.dart';

class FakePlaybackSessionRepository implements PlaybackSessionRepository {
  String createdSessionId = 'session-1';
  final Map<String, PlaybackTrackResolution> playResultByTrackId = {};
  final List<PlaybackTrackResolution> nextResults = [];
  Object? throwOnPlay;
  Object? throwOnNext;
  Object? throwOnCreateSession;

  int createSessionCalls = 0;
  int pauseCalls = 0;
  int seekCalls = 0;
  int nextCalls = 0;
  String? lastSeekSessionId;
  int? lastSeekPositionMs;
  String? lastPlayTrackId;
  int? lastPlayPositionMs;

  @override
  Future<String> createSession({required String deviceId}) async {
    createSessionCalls++;
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
    lastPlayPositionMs = positionMs;
    if (throwOnPlay != null) throw throwOnPlay!;
    return playResultByTrackId[trackId]!;
  }

  @override
  Future<void> pause({required String sessionId}) async {
    pauseCalls++;
  }

  @override
  Future<void> seek({required String sessionId, required int positionMs}) async {
    seekCalls++;
    lastSeekSessionId = sessionId;
    lastSeekPositionMs = positionMs;
  }

  @override
  Future<PlaybackTrackResolution> next({required String sessionId}) async {
    if (throwOnNext != null) throw throwOnNext!;
    final result = nextResults[nextCalls];
    nextCalls++;
    return result;
  }

  @override
  Future<void> enqueue({
    required String sessionId,
    required List<String> trackIds,
    String position = 'end',
  }) async {}
}
