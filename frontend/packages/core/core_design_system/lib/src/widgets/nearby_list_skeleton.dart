import 'package:flutter/material.dart';

import '../tokens/spacing.dart';
import 'skeleton_box.dart';

/// Skeleton matching one "who's nearby" card's shape (circular avatar +
/// title/subtitle + a trailing distance-bucket badge) - meaningfully
/// different from [TrackRowSkeleton]'s row (square artwork, no trailing
/// badge), per .vibeflow/specs/skeleton-loading-player-and-proximity.md's
/// instruction to check `nearby_listener_card.dart`'s actual layout before
/// reusing the track-row shape.
class NearbyListenerSkeleton extends StatelessWidget {
  const NearbyListenerSkeleton({super.key});

  @override
  Widget build(BuildContext context) {
    return const Card(
      margin: EdgeInsets.symmetric(horizontal: SmusicSpacing.md, vertical: SmusicSpacing.xs),
      child: Padding(
        padding: EdgeInsets.all(SmusicSpacing.md),
        child: Row(
          children: [
            SkeletonBox(
              width: 40,
              height: 40,
              borderRadius: BorderRadius.all(Radius.circular(20)),
            ),
            SizedBox(width: SmusicSpacing.md),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                mainAxisSize: MainAxisSize.min,
                children: [
                  SkeletonBox(width: double.infinity, height: 14),
                  SizedBox(height: SmusicSpacing.xs),
                  SkeletonBox(width: 100, height: 12),
                ],
              ),
            ),
            SizedBox(width: SmusicSpacing.sm),
            SkeletonBox(
              width: 64,
              height: 20,
              borderRadius: BorderRadius.all(Radius.circular(999)),
            ),
          ],
        ),
      ),
    );
  }
}

/// A vertical list of [NearbyListenerSkeleton], for `nearbyFeedProvider`'s
/// loading branch - the "who's nearby" list-of-cards equivalent of
/// [TrackListSkeleton].
class NearbyListSkeleton extends StatelessWidget {
  const NearbyListSkeleton({super.key, this.itemCount = 6});

  final int itemCount;

  @override
  Widget build(BuildContext context) {
    return ListView.builder(
      itemCount: itemCount,
      itemBuilder: (context, index) => const NearbyListenerSkeleton(),
    );
  }
}
