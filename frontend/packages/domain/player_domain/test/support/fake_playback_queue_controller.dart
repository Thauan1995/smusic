import 'dart:async';

import 'package:player_domain/player_domain.dart';

class FakePlaybackQueueController implements PlaybackQueueController {
  final StreamController<PlayerState> _stateController = StreamController.broadcast();
  final StreamController<QueueItem?> _nowPlayingController = StreamController.broadcast();

  List<QueueItem>? lastQueue;
  int? lastStartIndex;
  int pauseCalls = 0;
  int resumeCalls = 0;
  int skipNextCalls = 0;
  int skipPreviousCalls = 0;
  Duration? lastSeekPosition;

  @override
  Future<void> playFromQueue(List<QueueItem> queue, {required int startIndex}) async {
    lastQueue = queue;
    lastStartIndex = startIndex;
  }

  @override
  Future<void> pause() async {
    pauseCalls++;
  }

  @override
  Future<void> resume() async {
    resumeCalls++;
  }

  @override
  Future<void> skipNext() async {
    skipNextCalls++;
  }

  @override
  Future<void> skipPrevious() async {
    skipPreviousCalls++;
  }

  @override
  Future<void> seekTo(Duration position) async {
    lastSeekPosition = position;
  }

  @override
  Stream<PlayerState> get stateStream => _stateController.stream;

  @override
  Stream<QueueItem?> get nowPlayingStream => _nowPlayingController.stream;

  void emitState(PlayerState state) => _stateController.add(state);

  void emitNowPlaying(QueueItem? item) => _nowPlayingController.add(item);

  Future<void> dispose() async {
    await _stateController.close();
    await _nowPlayingController.close();
  }
}
