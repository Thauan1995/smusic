/// Domain-level playback-session failure. `player_data` translates
/// transport errors from `PlaybackSessionRepository`'s implementation into
/// this type (same rationale as `auth_domain.AuthException`).
class PlayerException implements Exception {
  const PlayerException(this.kind, {this.message});

  final PlayerExceptionKind kind;
  final String? message;

  @override
  String toString() =>
      'PlayerException(${kind.name}${message != null ? ': $message' : ''})';
}

enum PlayerExceptionKind { sessionNotFound, network, unauthorized, unknown }
