import 'package:core_networking/core_networking.dart';
import 'package:player_domain/player_domain.dart';

import '../dto/playback_dtos.dart';

class HttpPlaybackSessionRepository implements PlaybackSessionRepository {
  HttpPlaybackSessionRepository(this._client);

  final ApiClient _client;

  @override
  Future<String> createSession({required String deviceId}) {
    return _wrap(() async {
      final response = await _client.post(
        '/v1/playback/sessions',
        data: {'device_id': deviceId},
      );
      return response['session_id'] as String;
    });
  }

  @override
  Future<PlaybackTrackResolution> play({
    required String sessionId,
    required String trackId,
    int? positionMs,
  }) {
    return _wrap(() async {
      final response = await _client.post(
        '/v1/playback/sessions/$sessionId/play',
        data: {
          'track_id': trackId,
          if (positionMs != null) 'position_ms': positionMs,
        },
      );
      return PlaybackDtos.trackResolutionFromPlayResponse(
        response,
        trackId: trackId,
      );
    });
  }

  @override
  Future<void> pause({required String sessionId}) {
    return _wrap(() async {
      await _client.post('/v1/playback/sessions/$sessionId/pause');
    });
  }

  @override
  Future<void> seek({required String sessionId, required int positionMs}) {
    return _wrap(() async {
      await _client.post(
        '/v1/playback/sessions/$sessionId/seek',
        data: {'position_ms': positionMs},
      );
    });
  }

  @override
  Future<PlaybackTrackResolution> next({required String sessionId}) {
    return _wrap(() async {
      final response =
          await _client.post('/v1/playback/sessions/$sessionId/next');
      return PlaybackDtos.trackResolutionFromNextResponse(response);
    });
  }

  @override
  Future<void> enqueue({
    required String sessionId,
    required List<String> trackIds,
    String position = 'end',
  }) {
    return _wrap(() async {
      await _client.post(
        '/v1/playback/sessions/$sessionId/queue',
        data: {'track_ids': trackIds, 'position': position},
      );
    });
  }

  Future<T> _wrap<T>(Future<T> Function() body) async {
    try {
      return await body();
    } on ApiException catch (e) {
      throw PlayerException(_kindFor(e), message: e.message);
    }
  }

  PlayerExceptionKind _kindFor(ApiException e) {
    if (e.isUnauthorized) return PlayerExceptionKind.unauthorized;
    if (e.isNotFound) return PlayerExceptionKind.sessionNotFound;
    if (e.isNetworkError) return PlayerExceptionKind.network;
    return PlayerExceptionKind.unknown;
  }
}
