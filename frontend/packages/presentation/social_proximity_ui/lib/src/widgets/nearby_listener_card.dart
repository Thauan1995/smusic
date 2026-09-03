import 'package:core_design_system/core_design_system.dart';
import 'package:flutter/material.dart';
import 'package:social_proximity_domain/social_proximity_domain.dart';

/// One card in the "who's nearby" list (frontend-flutter.md section 4.2).
///
/// **Defense in depth (task requirement)**: this widget never reads
/// [NearbyListener.displayName]/[NearbyListener.avatarUrl] except behind an
/// explicit `revealLevel != RevealLevel.level0` check below - even though
/// `NearbyListener`'s own constructor already guarantees those fields are
/// `null` at level 0 (see that class's doc comment), the render path here
/// does not rely on that guarantee alone. At level 0 the card shows exactly
/// security.md section 1.6's copy: "Alguém por perto está ouvindo
/// *[Faixa]*" (or a track-less variant if nothing is playing) - no name, no
/// avatar, ever.
///
/// Distance is rendered exclusively via [DistanceBucket.label] - there is
/// no numeric value anywhere in this widget's build method (structurally
/// impossible to regress into showing meters from here, since the widget
/// never touches a number for distance at all).
class NearbyListenerCard extends StatelessWidget {
  const NearbyListenerCard({super.key, required this.listener});

  final NearbyListener listener;

  bool get _revealsIdentity => listener.revealLevel != RevealLevel.level0;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final nowPlaying = listener.nowPlaying;

    return Card(
      margin: const EdgeInsets.symmetric(horizontal: SmusicSpacing.md, vertical: SmusicSpacing.xs),
      child: Padding(
        padding: const EdgeInsets.all(SmusicSpacing.md),
        child: Row(
          children: [
            _Avatar(revealsIdentity: _revealsIdentity, avatarUrl: listener.avatarUrl),
            const SizedBox(width: SmusicSpacing.md),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    _revealsIdentity ? (listener.displayName ?? 'Alguém') : 'Alguém por perto',
                    key: const Key('nearby_listener_title'),
                    style: theme.textTheme.titleMedium,
                  ),
                  const SizedBox(height: SmusicSpacing.xs),
                  Text(
                    nowPlaying != null
                        ? 'Ouvindo ${nowPlaying.trackTitle}'
                        : 'Não está ouvindo nada no momento',
                    key: const Key('nearby_listener_now_playing'),
                    style: theme.textTheme.bodyMedium,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                  ),
                ],
              ),
            ),
            const SizedBox(width: SmusicSpacing.sm),
            _DistanceBadge(bucket: listener.distanceBucket),
          ],
        ),
      ),
    );
  }
}

class _Avatar extends StatelessWidget {
  const _Avatar({required this.revealsIdentity, required this.avatarUrl});

  final bool revealsIdentity;
  final String? avatarUrl;

  @override
  Widget build(BuildContext context) {
    if (!revealsIdentity) {
      return const CircleAvatar(
        key: Key('nearby_listener_anonymous_avatar'),
        child: Icon(Icons.person_outline),
      );
    }
    final url = avatarUrl;
    if (url == null) {
      return const CircleAvatar(key: Key('nearby_listener_placeholder_avatar'), child: Icon(Icons.person));
    }
    return CircleAvatar(key: const Key('nearby_listener_avatar'), backgroundImage: NetworkImage(url));
  }
}

class _DistanceBadge extends StatelessWidget {
  const _DistanceBadge({required this.bucket});

  final DistanceBucket bucket;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Container(
      key: const Key('nearby_listener_distance_badge'),
      padding: const EdgeInsets.symmetric(horizontal: SmusicSpacing.sm, vertical: SmusicSpacing.xs),
      decoration: BoxDecoration(
        color: theme.colorScheme.secondaryContainer,
        borderRadius: BorderRadius.circular(999),
      ),
      child: Text(
        bucket.label,
        style: theme.textTheme.labelSmall?.copyWith(color: theme.colorScheme.onSecondaryContainer),
      ),
    );
  }
}
