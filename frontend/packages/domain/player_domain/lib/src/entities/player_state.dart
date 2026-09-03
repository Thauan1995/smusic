import 'player_error.dart';
import 'queue_item.dart';

/// The player's state machine, engine-independent per
/// docs/architecture/frontend-flutter.md section 2.1 - `player_domain`
/// never sees `just_audio`, only this sealed hierarchy. Implemented as
/// hand-written Dart 3 sealed classes rather than `freezed` (documented
/// deviation, see frontend/README.md): exhaustive `switch` pattern matching
/// over this hierarchy works natively in Dart 3 without code-gen, and the
/// shape here is simple enough (5 fixed variants, no nested unions) that
/// freezed's boilerplate reduction isn't worth an extra build_runner step
/// for Fatia 1.
sealed class PlayerState {
  const factory PlayerState.idle() = PlayerIdle;
  const factory PlayerState.buffering(QueueItem current) = PlayerBuffering;
  const factory PlayerState.playing(QueueItem current, Duration position) =
      PlayerPlaying;
  const factory PlayerState.paused(QueueItem current, Duration position) =
      PlayerPaused;
  const factory PlayerState.error(PlayerError error) = PlayerErrorState;
}

class PlayerIdle implements PlayerState {
  const PlayerIdle();

  @override
  bool operator ==(Object other) => other is PlayerIdle;

  @override
  int get hashCode => (PlayerIdle).hashCode;
}

class PlayerBuffering implements PlayerState {
  const PlayerBuffering(this.current);

  final QueueItem current;

  @override
  bool operator ==(Object other) =>
      other is PlayerBuffering && other.current == current;

  @override
  int get hashCode => Object.hash(PlayerBuffering, current);
}

class PlayerPlaying implements PlayerState {
  const PlayerPlaying(this.current, this.position);

  final QueueItem current;
  final Duration position;

  @override
  bool operator ==(Object other) =>
      other is PlayerPlaying &&
      other.current == current &&
      other.position == position;

  @override
  int get hashCode => Object.hash(PlayerPlaying, current, position);
}

class PlayerPaused implements PlayerState {
  const PlayerPaused(this.current, this.position);

  final QueueItem current;
  final Duration position;

  @override
  bool operator ==(Object other) =>
      other is PlayerPaused &&
      other.current == current &&
      other.position == position;

  @override
  int get hashCode => Object.hash(PlayerPaused, current, position);
}

class PlayerErrorState implements PlayerState {
  const PlayerErrorState(this.error);

  final PlayerError error;

  @override
  bool operator ==(Object other) =>
      other is PlayerErrorState && other.error == error;

  @override
  int get hashCode => Object.hash(PlayerErrorState, error);
}
