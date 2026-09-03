import 'entities/player_state.dart';
import 'entities/queue_item.dart';

/// The only playback surface `presentation/player_ui` knows about
/// (frontend-flutter.md section 2.1: "PlaybackQueueController é a única
/// superfície que presentation/player_ui conhece"). Implemented once, in
/// `player_data`, by a class that translates these calls into
/// `NativeAudioEngine` calls plus `PlaybackSessionRepository` calls to keep
/// the backend's session state in sync.
///
/// DEVIATION FROM SPEC: the illustrative interface in
/// frontend-flutter.md section 2.1 lists only `playFromQueue`/`skipNext`/
/// `skipPrevious`/`seekTo` plus the two streams - it has no explicit
/// pause/resume. The task's concrete scope ("play/pause/seek/next/previous
/// funcionando fim a fim") requires commanding pause and resume distinctly
/// from starting a new queue, so `pause()`/`resume()` are added here as a
/// necessary, additive extension - no method from the spec's snippet was
/// removed or changed.
abstract interface class PlaybackQueueController {
  Future<void> playFromQueue(List<QueueItem> queue, {required int startIndex});

  Future<void> pause();

  Future<void> resume();

  Future<void> skipNext();

  Future<void> skipPrevious();

  Future<void> seekTo(Duration position);

  Stream<PlayerState> get stateStream;

  Stream<QueueItem?> get nowPlayingStream;
}
