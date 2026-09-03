import 'distance_bucket.dart';
import 'now_playing_snapshot.dart';
import 'reveal_level.dart';

/// A single nearby listener as rendered to the current user - the merged
/// result of `nearby_update`/`resync_full` per backend-go.md section 4.
///
/// **Defense in depth (task requirement, security.md section 1.6):** the
/// factory constructor enforces reveal level 0's "no name, no avatar" rule
/// itself, in the domain entity, independent of whatever
/// `social_proximity_data`'s DTO parsing does. Even if a backend bug (or a
/// future contract change) put a name in the wire payload for a level-0
/// entry, [displayName]/[avatarUrl] are unconditionally nulled out below -
/// there is no code path in this class that can produce a level-0
/// [NearbyListener] carrying a name or avatar. `social_proximity_ui`'s
/// cards additionally never read [displayName]/[avatarUrl] except behind an
/// explicit `revealLevel != RevealLevel.level0` check, so the guarantee is
/// enforced twice (entity constriction + UI render), per the task's
/// "defesa em profundidade" requirement.
class NearbyListener {
  factory NearbyListener({
    required String userId,
    required DistanceBucket distanceBucket,
    required RevealLevel revealLevel,
    String? displayName,
    String? avatarUrl,
    NowPlayingSnapshot? nowPlaying,
  }) {
    final revealsIdentity = revealLevel != RevealLevel.level0;
    return NearbyListener._(
      userId: userId,
      distanceBucket: distanceBucket,
      revealLevel: revealLevel,
      displayName: revealsIdentity ? displayName : null,
      avatarUrl: revealsIdentity ? avatarUrl : null,
      nowPlaying: nowPlaying,
    );
  }

  const NearbyListener._({
    required this.userId,
    required this.distanceBucket,
    required this.revealLevel,
    required this.displayName,
    required this.avatarUrl,
    required this.nowPlaying,
  });

  final String userId;
  final DistanceBucket distanceBucket;
  final RevealLevel revealLevel;

  /// Always `null` when [revealLevel] is [RevealLevel.level0] - see class
  /// doc comment.
  final String? displayName;

  /// Always `null` when [revealLevel] is [RevealLevel.level0] - see class
  /// doc comment.
  final String? avatarUrl;

  final NowPlayingSnapshot? nowPlaying;

  @override
  bool operator ==(Object other) =>
      other is NearbyListener &&
      other.userId == userId &&
      other.distanceBucket == distanceBucket &&
      other.revealLevel == revealLevel &&
      other.displayName == displayName &&
      other.avatarUrl == avatarUrl &&
      other.nowPlaying == nowPlaying;

  @override
  int get hashCode => Object.hash(
        userId,
        distanceBucket,
        revealLevel,
        displayName,
        avatarUrl,
        nowPlaying,
      );
}
