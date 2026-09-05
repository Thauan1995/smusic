import 'package:flutter/material.dart';

import '../tokens/spacing.dart';
import 'skeleton_box.dart';

/// Skeleton matching the "Now Playing" screen's layout (album art + title/
/// artist text + seek bar + transport row) - a single now-playing view
/// isn't a list of rows, so [TrackRowSkeleton]'s shape doesn't fit; this is
/// its own composition of [SkeletonBox] primitives, per
/// .vibeflow/specs/skeleton-loading-player-and-proximity.md. Deliberately
/// simple (a handful of static rectangles, no per-element stagger) so it
/// doesn't itself become visual noise on a screen that can legitimately
/// re-render its loading state in quick succession.
class NowPlayingSkeleton extends StatelessWidget {
  const NowPlayingSkeleton({super.key});

  @override
  Widget build(BuildContext context) {
    return const Padding(
      padding: EdgeInsets.all(SmusicSpacing.lg),
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          SkeletonBox(
            width: 240,
            height: 240,
            borderRadius: BorderRadius.all(Radius.circular(8)),
          ),
          SizedBox(height: SmusicSpacing.lg),
          SkeletonBox(width: 180, height: 20),
          SizedBox(height: SmusicSpacing.xs),
          SkeletonBox(width: 120, height: 14),
          SizedBox(height: SmusicSpacing.lg),
          SkeletonBox(width: double.infinity, height: 4),
          SizedBox(height: SmusicSpacing.md),
          Row(
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              SkeletonBox(
                width: 36,
                height: 36,
                borderRadius: BorderRadius.all(Radius.circular(18)),
              ),
              SizedBox(width: SmusicSpacing.md),
              SkeletonBox(
                width: 56,
                height: 56,
                borderRadius: BorderRadius.all(Radius.circular(28)),
              ),
              SizedBox(width: SmusicSpacing.md),
              SkeletonBox(
                width: 36,
                height: 36,
                borderRadius: BorderRadius.all(Radius.circular(18)),
              ),
            ],
          ),
        ],
      ),
    );
  }
}
