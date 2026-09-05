import 'package:flutter/material.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:social_proximity_domain/social_proximity_domain.dart';

/// security.md section 1.4: "Toggle único e de acesso rápido (tela
/// inicial, não enterrado em configurações): 'Pausar descoberta'." Meant to
/// be embeddable both in [ProximityListScreen]'s app bar and in
/// `shared_navigation`'s shell (task scope item 5) - a single small widget
/// rather than a full screen, so it is truly one tap away from wherever the
/// user is in the app, not just from within the proximity feature's own
/// screens.
///
/// Renders nothing when the feature isn't enabled at all (security.md 1.1:
/// pausing only makes sense once opted in) - this keeps it safe to place
/// unconditionally in shared chrome without a separate visibility check at
/// every call site.
class PauseDiscoveryToggle extends ConsumerWidget {
  const PauseDiscoveryToggle({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final settings = ref.watch(proximityPrivacySettingsProvider).valueOrNull;
    if (settings == null || !settings.enabled) return const SizedBox.shrink();

    final isPaused = settings.paused;
    return IconButton(
      key: const Key('pause_discovery_toggle'),
      tooltip: isPaused ? 'Retomar descoberta por proximidade' : 'Pausar descoberta por proximidade',
      // Filled = in-progress (discovery active, tap to pause); outlined =
      // available action (paused, tap to resume) - see
      // .vibeflow/patterns/frontend-design-system.md's filled/outlined
      // rule, applied identically to player_screen's play/pause button.
      icon: Icon(isPaused ? Icons.play_circle_outline : Icons.pause_circle_filled),
      onPressed: () => ref.read(proximityPrivacySettingsProvider.notifier).setPaused(!isPaused),
    );
  }
}
