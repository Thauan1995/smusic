import 'package:riverpod/riverpod.dart';

import 'entities/player_state.dart';
import 'entities/queue_item.dart';
import 'playback_queue_controller.dart';

/// Overridden in `app/*` with `player_data`'s `JustAudioPlaybackAdapter`.
/// Same composition pattern as `auth_domain`/`library_domain` (see
/// `auth_domain`'s code-gen deviation note - applies here too).
final playbackQueueControllerProvider = Provider<PlaybackQueueController>((ref) {
  throw UnimplementedError(
    'playbackQueueControllerProvider must be overridden by app/* with a player_data implementation.',
  );
});

/// `player_ui` watches this rather than calling `stateStream.listen`
/// directly - keeps the widget layer declarative (`AsyncValue`-driven, per
/// frontend-flutter.md section 1.1) instead of manually managing a stream
/// subscription.
final playerStateProvider = StreamProvider<PlayerState>((ref) {
  return ref.watch(playbackQueueControllerProvider).stateStream;
});

final nowPlayingProvider = StreamProvider<QueueItem?>((ref) {
  return ref.watch(playbackQueueControllerProvider).nowPlayingStream;
});
