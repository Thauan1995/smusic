import 'package:core_design_system/core_design_system.dart';
import 'package:flutter/material.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:social_proximity_domain/social_proximity_domain.dart';

/// frontend-flutter.md section 4.4: "Se negado, a UI mostra estado vazio
/// explicando o motivo, com CTA para abrir configurações do SO ... nunca
/// insiste com prompt repetido." [LocationPermissionState.deniedForever]/
/// `restricted` route to [LocationPermissionNotifier.openAppSettings];
/// every other non-granted state offers a manual retry
/// ([LocationPermissionNotifier.request]) - the OS prompt is *never*
/// triggered without this explicit tap, so re-rendering this widget after a
/// denial cannot itself re-prompt.
class ProximityPermissionGate extends ConsumerWidget {
  const ProximityPermissionGate({super.key, required this.permission});

  final LocationPermissionState permission;

  bool get _isPermanentlyBlocked =>
      permission == LocationPermissionState.deniedForever ||
      permission == LocationPermissionState.restricted;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return Scaffold(
      appBar: AppBar(title: const Text('Perto de você')),
      body: EmptyState(
        icon: Icons.location_off,
        message: _isPermanentlyBlocked
            ? 'A permissão de localização está desativada nas configurações do '
                'sistema. Ative-a para ver quem está por perto.'
            : 'Precisamos da sua localização aproximada para mostrar quem está '
                'por perto.',
        actionLabel: _isPermanentlyBlocked ? 'Abrir configurações' : 'Permitir localização',
        onAction: () {
          final notifier = ref.read(locationPermissionProvider.notifier);
          if (_isPermanentlyBlocked) {
            notifier.openAppSettings();
          } else {
            notifier.request();
          }
        },
      ),
    );
  }
}
