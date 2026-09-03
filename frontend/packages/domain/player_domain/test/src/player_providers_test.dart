import 'package:player_domain/player_domain.dart';
import 'package:riverpod/riverpod.dart';
import 'package:test/test.dart';

import '../support/fake_playback_queue_controller.dart';

QueueItem _item() => QueueItem(
      trackId: 't1',
      title: 'Song',
      artistName: 'Artist',
      durationMs: 1000,
    );

void main() {
  test('playbackQueueControllerProvider throws when not overridden', () {
    final container = ProviderContainer();
    addTearDown(container.dispose);
    expect(
      () => container.read(playbackQueueControllerProvider),
      throwsUnimplementedError,
    );
  });

  group('with an overridden controller', () {
    late FakePlaybackQueueController controller;
    late ProviderContainer container;

    setUp(() {
      controller = FakePlaybackQueueController();
      container = ProviderContainer(
        overrides: [
          playbackQueueControllerProvider.overrideWithValue(controller),
        ],
      );
      addTearDown(() async {
        container.dispose();
        await controller.dispose();
      });
    });

    test('playerStateProvider forwards stateStream', () async {
      final sub = container.listen(playerStateProvider, (prev, next) {});
      addTearDown(sub.close);
      // Let the StreamProvider attach its listener.
      await Future<void>.delayed(Duration.zero);

      controller.emitState(const PlayerState.idle());
      await Future<void>.delayed(Duration.zero);

      expect(container.read(playerStateProvider).value, const PlayerState.idle());
    });

    test('nowPlayingProvider forwards nowPlayingStream', () async {
      final sub = container.listen(nowPlayingProvider, (prev, next) {});
      addTearDown(sub.close);
      await Future<void>.delayed(Duration.zero);

      controller.emitNowPlaying(_item());
      await Future<void>.delayed(Duration.zero);

      expect(container.read(nowPlayingProvider).value, _item());
    });
  });
}
