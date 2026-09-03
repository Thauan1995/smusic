import 'package:core_design_system/core_design_system.dart';
import 'package:flutter/material.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:player_domain/player_domain.dart';

import '../duration_format.dart';

/// Expanded ("now playing") player screen (task scope item 5). Play/pause/
/// seek/next/previous act directly on `PlaybackQueueController` -
/// `player_ui` never touches `NativeAudioEngine` or
/// `PlaybackSessionRepository` directly, only the one controller surface
/// (frontend-flutter.md section 2.1).
class PlayerScreen extends ConsumerWidget {
  const PlayerScreen({super.key, this.onClose});

  final VoidCallback? onClose;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final playerStateAsync = ref.watch(playerStateProvider);

    return Scaffold(
      appBar: AppBar(
        leading: IconButton(
          icon: const Icon(Icons.expand_more),
          onPressed: onClose,
        ),
        title: const Text('Now Playing'),
      ),
      body: playerStateAsync.when(
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (error, stackTrace) => const EmptyState(
          message: 'Playback error. Please try again.',
          icon: Icons.error_outline,
        ),
        data: (state) => _PlayerBody(state: state),
      ),
    );
  }
}

class _PlayerBody extends ConsumerWidget {
  const _PlayerBody({required this.state});

  final PlayerState state;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return switch (state) {
      PlayerIdle() => const EmptyState(
          message: 'Nothing is playing right now.',
          icon: Icons.music_off,
        ),
      PlayerErrorState(:final error) => EmptyState(
          message: error.message,
          icon: Icons.error_outline,
        ),
      PlayerBuffering(:final current) => _NowPlayingBody(
          title: current.title,
          artist: current.artistName,
          position: Duration.zero,
          duration: current.duration,
          isBuffering: true,
          isPlaying: false,
        ),
      PlayerPlaying(:final current, :final position) => _NowPlayingBody(
          title: current.title,
          artist: current.artistName,
          position: position,
          duration: current.duration,
          isBuffering: false,
          isPlaying: true,
        ),
      PlayerPaused(:final current, :final position) => _NowPlayingBody(
          title: current.title,
          artist: current.artistName,
          position: position,
          duration: current.duration,
          isBuffering: false,
          isPlaying: false,
        ),
    };
  }
}

class _NowPlayingBody extends ConsumerWidget {
  const _NowPlayingBody({
    required this.title,
    required this.artist,
    required this.position,
    required this.duration,
    required this.isBuffering,
    required this.isPlaying,
  });

  final String title;
  final String artist;
  final Duration position;
  final Duration duration;
  final bool isBuffering;
  final bool isPlaying;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final controller = ref.read(playbackQueueControllerProvider);
    final clampedPositionMs = position.inMilliseconds
        .clamp(0, duration.inMilliseconds == 0 ? 1 : duration.inMilliseconds)
        .toDouble();

    return Padding(
      padding: const EdgeInsets.all(SmusicSpacing.lg),
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Container(
            width: 240,
            height: 240,
            decoration: BoxDecoration(
              color: Theme.of(context).colorScheme.secondaryContainer,
              borderRadius: BorderRadius.circular(8),
            ),
            child: Icon(
              Icons.music_note,
              size: 96,
              color: Theme.of(context).colorScheme.onSecondaryContainer,
            ),
          ),
          const SizedBox(height: SmusicSpacing.lg),
          Text(title, style: Theme.of(context).textTheme.titleLarge, textAlign: TextAlign.center),
          const SizedBox(height: SmusicSpacing.xs),
          Text(artist, style: Theme.of(context).textTheme.bodyMedium, textAlign: TextAlign.center),
          const SizedBox(height: SmusicSpacing.lg),
          Slider(
            key: const Key('player_seek_slider'),
            value: clampedPositionMs,
            max: duration.inMilliseconds == 0 ? 1 : duration.inMilliseconds.toDouble(),
            onChanged: isBuffering
                ? null
                : (value) => controller.seekTo(Duration(milliseconds: value.round())),
          ),
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: SmusicSpacing.sm),
            child: Row(
              mainAxisAlignment: MainAxisAlignment.spaceBetween,
              children: [
                Text(formatDuration(position)),
                Text(formatDuration(duration)),
              ],
            ),
          ),
          const SizedBox(height: SmusicSpacing.md),
          Row(
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              IconButton(
                key: const Key('player_previous_button'),
                iconSize: 36,
                icon: const Icon(Icons.skip_previous),
                onPressed: isBuffering ? null : controller.skipPrevious,
              ),
              const SizedBox(width: SmusicSpacing.md),
              if (isBuffering)
                const SizedBox(
                  width: 56,
                  height: 56,
                  child: CircularProgressIndicator(),
                )
              else
                IconButton(
                  key: const Key('player_play_pause_button'),
                  iconSize: 56,
                  icon: Icon(isPlaying ? Icons.pause_circle_filled : Icons.play_circle_filled),
                  onPressed: isPlaying ? controller.pause : controller.resume,
                ),
              const SizedBox(width: SmusicSpacing.md),
              IconButton(
                key: const Key('player_next_button'),
                iconSize: 36,
                icon: const Icon(Icons.skip_next),
                onPressed: isBuffering ? null : controller.skipNext,
              ),
            ],
          ),
        ],
      ),
    );
  }
}
