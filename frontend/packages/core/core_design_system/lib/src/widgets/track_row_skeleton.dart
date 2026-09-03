import 'package:flutter/material.dart';

import '../tokens/spacing.dart';
import 'skeleton_box.dart';

/// Skeleton matching the shape of a track/album/playlist row (artwork +
/// title + subtitle), shared by `library_ui` and `player_ui` so both
/// features get the exact same loading shape with zero per-feature skeleton
/// code (frontend-flutter.md section 3.4).
class TrackRowSkeleton extends StatelessWidget {
  const TrackRowSkeleton({super.key});

  @override
  Widget build(BuildContext context) {
    return const Padding(
      padding: EdgeInsets.symmetric(
        horizontal: SmusicSpacing.md,
        vertical: SmusicSpacing.sm,
      ),
      child: Row(
        children: [
          SkeletonBox(
            width: 48,
            height: 48,
            borderRadius: BorderRadius.all(Radius.circular(4)),
          ),
          SizedBox(width: SmusicSpacing.md),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              mainAxisSize: MainAxisSize.min,
              children: [
                SkeletonBox(width: double.infinity, height: 14),
                SizedBox(height: SmusicSpacing.xs),
                SkeletonBox(width: 120, height: 12),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

/// A vertical list of [TrackRowSkeleton], sized to plausibly fill a screen
/// while loading (used by `library_ui`'s `AsyncValue.loading` branch).
class TrackListSkeleton extends StatelessWidget {
  const TrackListSkeleton({super.key, this.itemCount = 8});

  final int itemCount;

  @override
  Widget build(BuildContext context) {
    return ListView.builder(
      itemCount: itemCount,
      itemBuilder: (context, index) => const TrackRowSkeleton(),
    );
  }
}
