import 'package:core_design_system/core_design_system.dart';
import 'package:flutter/material.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:player_domain/player_domain.dart';

/// Persistent playback bar docked above the navigation shell
/// (frontend-flutter.md section 3.5), shown whenever something is loaded
/// into the queue. Hidden entirely (`SizedBox.shrink`) when nothing is
/// playing - never renders empty chrome.
class MiniPlayerBar extends ConsumerWidget {
  const MiniPlayerBar({super.key, this.onExpand});

  final VoidCallback? onExpand;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final nowPlaying = ref.watch(nowPlayingProvider).valueOrNull;
    final playerState = ref.watch(playerStateProvider).valueOrNull;

    if (nowPlaying == null || playerState == null || playerState is PlayerIdle) {
      return const SizedBox.shrink();
    }

    final isPlaying = playerState is PlayerPlaying;
    final isBuffering = playerState is PlayerBuffering;

    return Material(
      color: Theme.of(context).colorScheme.surfaceContainerHighest,
      child: InkWell(
        onTap: onExpand,
        child: Padding(
          padding: const EdgeInsets.symmetric(
            horizontal: SmusicSpacing.md,
            vertical: SmusicSpacing.sm,
          ),
          child: Row(
            children: [
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Text(nowPlaying.title, maxLines: 1, overflow: TextOverflow.ellipsis),
                    Text(
                      nowPlaying.artistName,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: Theme.of(context).textTheme.bodySmall,
                    ),
                  ],
                ),
              ),
              if (isBuffering)
                const SizedBox(
                  width: 24,
                  height: 24,
                  child: CircularProgressIndicator(strokeWidth: 2),
                )
              else
                IconButton(
                  key: const Key('mini_player_play_pause_button'),
                  icon: Icon(isPlaying ? Icons.pause : Icons.play_arrow),
                  onPressed: () {
                    final controller = ref.read(playbackQueueControllerProvider);
                    isPlaying ? controller.pause() : controller.resume();
                  },
                ),
              IconButton(
                key: const Key('mini_player_next_button'),
                icon: const Icon(Icons.skip_next),
                onPressed: () => ref.read(playbackQueueControllerProvider).skipNext(),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
