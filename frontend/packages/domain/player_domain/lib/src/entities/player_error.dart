/// Domain-level player failure - wraps whatever went wrong (network,
/// codec/engine error surfaced via `NativeAudioEngine`, or a backend
/// playback-session error) into one shape `player_ui` can render uniformly.
class PlayerError {
  const PlayerError(this.message, {this.cause});

  final String message;
  final Object? cause;

  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      other is PlayerError && other.message == message && other.cause == cause;

  @override
  int get hashCode => Object.hash(message, cause);

  @override
  String toString() => 'PlayerError($message)';
}
