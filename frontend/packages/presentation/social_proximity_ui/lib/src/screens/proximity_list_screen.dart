import 'package:core_design_system/core_design_system.dart';
import 'package:flutter/material.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:social_proximity_domain/social_proximity_domain.dart';

import '../widgets/animated_nearby_list.dart';
import '../widgets/connection_status_banner.dart';
import '../widgets/pause_discovery_toggle.dart';
import 'proximity_permission_gate.dart';
import 'proximity_value_screen.dart';

/// The "who's nearby" list screen (frontend-flutter.md section 4.2, list
/// mode - the default, no-map mode) and the orchestrator of the opt-in/
/// permission flow that gates it (section 4.4): not opted in (or consent
/// lapsed) -> [ProximityValueScreen]; opted in but OS permission missing ->
/// [ProximityPermissionGate]; both satisfied -> the live list, fed by
/// `nearbyFeedProvider`.
class ProximityListScreen extends ConsumerWidget {
  const ProximityListScreen({super.key, this.onOpenSettings});

  final VoidCallback? onOpenSettings;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final settingsAsync = ref.watch(proximityPrivacySettingsProvider);
    final settings = settingsAsync.valueOrNull;

    if (settingsAsync.isLoading && settings == null) {
      return const Scaffold(body: Center(child: CircularProgressIndicator()));
    }

    if (settingsAsync.hasError && settings == null) {
      return Scaffold(
        appBar: AppBar(title: const Text('Perto de você')),
        body: EmptyState(
          icon: Icons.error_outline,
          message: 'Não foi possível carregar suas configurações de privacidade.',
          actionLabel: 'Tentar de novo',
          onAction: () => ref.invalidate(proximityPrivacySettingsProvider),
        ),
      );
    }

    final needsRenewal = settings?.needsConsentRenewal() ?? false;
    if (settings == null || !settings.enabled || needsRenewal) {
      return ProximityValueScreen(
        isRenewal: needsRenewal,
        onOptedIn: () => ref.read(locationPermissionProvider.notifier).request(),
      );
    }

    final permission =
        ref.watch(locationPermissionProvider).valueOrNull ?? LocationPermissionState.notRequested;
    if (permission != LocationPermissionState.granted) {
      return ProximityPermissionGate(permission: permission);
    }

    final feedAsync = ref.watch(nearbyFeedProvider);

    return Scaffold(
      appBar: AppBar(
        title: const Text('Perto de você'),
        actions: [
          const PauseDiscoveryToggle(),
          IconButton(
            key: const Key('proximity_open_settings_button'),
            icon: const Icon(Icons.tune),
            tooltip: 'Configurações de privacidade',
            onPressed: onOpenSettings,
          ),
        ],
      ),
      body: feedAsync.when(
        // Settings' own earlier loading gate (above) stays a plain
        // spinner - it's a short "which screen do we even show" decision,
        // not list content (see .vibeflow/specs/
        // skeleton-loading-player-and-proximity.md's anti-scope on
        // form/decision screens). This is the actual "who's nearby" list.
        loading: () => const NearbyListSkeleton(),
        error: (error, stackTrace) => EmptyState(
          icon: Icons.error_outline,
          message: 'Não foi possível carregar quem está por perto.',
          actionLabel: 'Tentar de novo',
          onAction: () => ref.invalidate(nearbyFeedProvider),
        ),
        data: (state) => Column(
          children: [
            ConnectionStatusBanner(connectionState: state.connectionState),
            Expanded(
              child: state.listeners.isEmpty
                  ? const EmptyState(
                      icon: Icons.explore_off,
                      message: 'Ninguém por perto ouvindo música no momento.',
                    )
                  : AnimatedNearbyList(listeners: state.listeners),
            ),
          ],
        ),
      ),
    );
  }
}
