import 'package:core_design_system/core_design_system.dart';
import 'package:flutter/material.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:social_proximity_domain/social_proximity_domain.dart';

/// security.md section 1.1 / frontend-flutter.md section 4.4: the opt-in
/// value screen shown **before** the OS location-permission prompt,
/// explaining in plain language what the feature does. Its CTA only
/// persists the feature's own consent (`enableFeature`) - it deliberately
/// does **not** request OS location permission itself; [onOptedIn] is
/// called once that succeeds, and the caller (`ProximityListScreen`) is
/// responsible for requesting OS permission next. Keeping those two steps
/// separate mirrors security.md 1.1's point that feature consent and OS
/// location permission are independent axes that "podem estar ativas em
/// combinações diferentes".
class ProximityValueScreen extends ConsumerWidget {
  const ProximityValueScreen({super.key, this.onOptedIn, this.isRenewal = false});

  final VoidCallback? onOptedIn;

  /// security.md 1.1's 6-month re-confirmation - same screen, adjusted
  /// copy/CTA label, rather than a separate screen (the value proposition
  /// being re-confirmed is identical).
  final bool isRenewal;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final settingsState = ref.watch(proximityPrivacySettingsProvider);
    final isSubmitting = settingsState.isLoading && settingsState.hasValue;

    return Scaffold(
      appBar: AppBar(title: const Text('Descoberta por proximidade')),
      body: SafeArea(
        child: Center(
          child: ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 480),
            child: SingleChildScrollView(
              padding: const EdgeInsets.all(SmusicSpacing.lg),
              child: Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  const Icon(Icons.wifi_tethering, size: 56),
                  const SizedBox(height: SmusicSpacing.lg),
                  Text(
                    isRenewal
                        ? 'Confirme para continuar usando a descoberta por proximidade'
                        : 'Veja quem está ouvindo música perto de você',
                    style: Theme.of(context).textTheme.headlineSmall,
                    textAlign: TextAlign.center,
                  ),
                  const SizedBox(height: SmusicSpacing.md),
                  const _ValueBullet(
                    text: 'Sua localização aproximada é usada continuamente enquanto a '
                        'descoberta estiver ativa.',
                  ),
                  const _ValueBullet(
                    text: 'A música que você está ouvindo pode ficar visível para pessoas '
                        'por perto.',
                  ),
                  const _ValueBullet(
                    text: 'Raio de descoberta inicial: 1 km - você pode ajustar isso a '
                        'qualquer momento nas configurações de privacidade.',
                  ),
                  const _ValueBullet(
                    text: 'Você pode pausar ou desativar a descoberta a qualquer momento, '
                        'com efeito imediato.',
                  ),
                  const SizedBox(height: SmusicSpacing.lg),
                  SmusicPrimaryButton(
                    label: isRenewal ? 'Confirmar e continuar' : 'Ativar descoberta por proximidade',
                    isLoading: isSubmitting,
                    onPressed: () async {
                      await ref.read(proximityPrivacySettingsProvider.notifier).enableFeature();
                      onOptedIn?.call();
                    },
                  ),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }
}

class _ValueBullet extends StatelessWidget {
  const _ValueBullet({required this.text});

  final String text;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: SmusicSpacing.xs),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const Icon(Icons.check_circle_outline, size: 18),
          const SizedBox(width: SmusicSpacing.sm),
          Expanded(child: Text(text, style: Theme.of(context).textTheme.bodyMedium)),
        ],
      ),
    );
  }
}
