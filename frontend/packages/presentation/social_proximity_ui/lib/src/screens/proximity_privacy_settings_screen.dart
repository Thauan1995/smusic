import 'package:core_design_system/core_design_system.dart';
import 'package:flutter/material.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:social_proximity_domain/social_proximity_domain.dart';

/// The dedicated privacy settings screen (task scope item 2 - "não
/// escondida em submenu"): opt-in/opt-out, quick pause, radius slider,
/// reveal-level selector, and the consent-renewal indication (security.md
/// section 1).
///
/// [onSetUpMfa], if provided, is called (with this screen's [BuildContext])
/// when enabling the feature hits `ProximityExceptionKind.mfaRequired`
/// (security.md §2's TOTP step-up gate on `SettingsService.GrantConsent`) -
/// it should push an MFA enrollment screen and resolve to `true` once the
/// user verifies a code, so this screen can retry enabling. Never navigates
/// itself (this package has no `go_router` dependency, matching every other
/// screen here - see `shared_navigation`'s `app_router.dart` for the actual
/// route wiring); a null callback (or one that resolves to anything but
/// `true`) simply leaves the switch off, same as before this gate existed.
class ProximityPrivacySettingsScreen extends ConsumerWidget {
  const ProximityPrivacySettingsScreen({super.key, this.onSetUpMfa});

  final Future<bool?> Function(BuildContext context)? onSetUpMfa;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final settingsAsync = ref.watch(proximityPrivacySettingsProvider);
    // Deliberately data-first, not `settingsAsync.when(...)`: a mutation
    // failure (e.g. the mfaRequired gate below) still carries the last-
    // known settings via `copyWithPrevious`, and re-rendering the whole
    // screen as a generic error would hide the switch itself right when
    // the user most needs to see why it didn't turn on. The full-screen
    // error/loading states are reserved for when there is truly no
    // settings value yet (the very first fetch).
    final settings = settingsAsync.valueOrNull;

    return Scaffold(
      appBar: AppBar(title: const Text('Privacidade da descoberta por proximidade')),
      body: settings != null
          ? _SettingsBody(settings: settings, onSetUpMfa: onSetUpMfa)
          : settingsAsync.isLoading
              ? const Center(child: CircularProgressIndicator())
              : EmptyState(
                  icon: Icons.error_outline,
                  message: 'Não foi possível carregar suas configurações.',
                  actionLabel: 'Tentar de novo',
                  onAction: () => ref.invalidate(proximityPrivacySettingsProvider),
                ),
    );
  }
}

class _SettingsBody extends ConsumerWidget {
  const _SettingsBody({required this.settings, this.onSetUpMfa});

  final ProximityPrivacySettings settings;
  final Future<bool?> Function(BuildContext context)? onSetUpMfa;

  Future<void> _handleEnabledChanged(
    BuildContext context,
    ProximityPrivacySettingsNotifier notifier,
    bool value,
  ) async {
    if (!value) {
      notifier.disableFeature();
      return;
    }
    try {
      await notifier.enableFeature();
    } on ProximityException catch (e) {
      if (e.kind != ProximityExceptionKind.mfaRequired) return;
      if (onSetUpMfa == null || !context.mounted) return;
      final verified = await onSetUpMfa!(context);
      if (verified == true && context.mounted) {
        await notifier.enableFeature();
      }
    }
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final notifier = ref.read(proximityPrivacySettingsProvider.notifier);
    final needsRenewal = settings.needsConsentRenewal();

    return ListView(
      padding: const EdgeInsets.all(SmusicSpacing.md),
      children: [
        SwitchListTile(
          key: const Key('proximity_enabled_switch'),
          title: const Text('Descoberta por proximidade'),
          subtitle: const Text('Veja e seja visto por pessoas ouvindo música perto de você.'),
          value: settings.enabled,
          onChanged: (value) => _handleEnabledChanged(context, notifier, value),
        ),
        if (settings.enabled) ...[
          SwitchListTile(
            key: const Key('proximity_paused_switch'),
            title: const Text('Pausar descoberta'),
            subtitle: const Text(
              'Você continua ouvindo música normalmente; só some da lista de descoberta.',
            ),
            value: settings.paused,
            onChanged: notifier.setPaused,
          ),
          const SizedBox(height: SmusicSpacing.sm),
          if (needsRenewal)
            Card(
              key: const Key('proximity_consent_renewal_banner'),
              color: Theme.of(context).colorScheme.errorContainer,
              child: ListTile(
                leading: const Icon(Icons.warning_amber),
                title: const Text('Renove seu consentimento'),
                subtitle: const Text(
                  'Por segurança, pedimos para confirmar novamente a cada 6 meses.',
                ),
                trailing: TextButton(
                  key: const Key('proximity_renew_consent_button'),
                  onPressed: notifier.renewConsent,
                  child: const Text('Renovar'),
                ),
              ),
            )
          else if (settings.consentRenewalDueAt != null)
            Padding(
              padding: const EdgeInsets.symmetric(vertical: SmusicSpacing.sm),
              child: Text(
                'Consentimento válido até ${_formatDate(settings.consentRenewalDueAt!)}.',
                key: const Key('proximity_consent_renewal_date'),
                style: Theme.of(context).textTheme.bodySmall,
              ),
            ),
          const SizedBox(height: SmusicSpacing.lg),
          Text('Raio de descoberta', style: Theme.of(context).textTheme.titleMedium),
          Text(
            'Até onde você aceita ser encontrado (não é o quanto você enxerga).',
            style: Theme.of(context).textTheme.bodySmall,
          ),
          _RadiusSelector(value: settings.radius, onChanged: notifier.setRadius),
          const SizedBox(height: SmusicSpacing.lg),
          Text('Quem pode me ver', style: Theme.of(context).textTheme.titleMedium),
          const SizedBox(height: SmusicSpacing.sm),
          _VisibilitySelector(value: settings.visibilityMode, onChanged: notifier.setVisibilityMode),
          const SizedBox(height: SmusicSpacing.lg),
          Text('Nível de revelação para desconhecidos', style: Theme.of(context).textTheme.titleMedium),
          const SizedBox(height: SmusicSpacing.sm),
          _RevealLevelSelector(value: settings.maxRevealLevel, onChanged: notifier.setMaxRevealLevel),
        ],
      ],
    );
  }
}

String _formatDate(DateTime date) {
  final day = date.day.toString().padLeft(2, '0');
  final month = date.month.toString().padLeft(2, '0');
  return '$day/$month/${date.year}';
}

class _RadiusSelector extends StatelessWidget {
  const _RadiusSelector({required this.value, required this.onChanged});

  final ProximityRadius value;
  final ValueChanged<ProximityRadius> onChanged;

  @override
  Widget build(BuildContext context) {
    final values = ProximityRadius.values;
    final index = values.indexOf(value);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Slider(
          key: const Key('proximity_radius_slider'),
          value: index.toDouble(),
          min: 0,
          max: (values.length - 1).toDouble(),
          divisions: values.length - 1,
          label: value.label,
          onChanged: (raw) => onChanged(values[raw.round()]),
        ),
        Text(value.label, key: const Key('proximity_radius_label')),
      ],
    );
  }
}

class _VisibilitySelector extends StatelessWidget {
  const _VisibilitySelector({required this.value, required this.onChanged});

  final ProximityVisibilityMode value;
  final ValueChanged<ProximityVisibilityMode> onChanged;

  static const _labels = {
    ProximityVisibilityMode.invisible: 'Invisível',
    ProximityVisibilityMode.friendsOnly: 'Só amigos',
    ProximityVisibilityMode.everyone: 'Todos',
  };

  @override
  Widget build(BuildContext context) {
    return SegmentedButton<ProximityVisibilityMode>(
      key: const Key('proximity_visibility_selector'),
      segments: [
        for (final mode in ProximityVisibilityMode.values)
          ButtonSegment(value: mode, label: Text(_labels[mode]!)),
      ],
      selected: {value},
      onSelectionChanged: (selection) => onChanged(selection.first),
    );
  }
}

class _RevealLevelSelector extends StatelessWidget {
  const _RevealLevelSelector({required this.value, required this.onChanged});

  final RevealLevel value;
  final ValueChanged<RevealLevel> onChanged;

  static const _labels = {
    RevealLevel.level0: 'Anônimo',
    RevealLevel.level1: 'Nome para conexões',
    RevealLevel.level2: 'Nome para todos',
  };

  /// security.md section 1.6: raising to level 2 ("descoberta aberta") is a
  /// second, separate explicit consent - never applied from a single tap
  /// without this confirmation.
  Future<void> _handleSelection(BuildContext context, RevealLevel level) async {
    if (level == RevealLevel.level2) {
      final confirmed = await showDialog<bool>(
        context: context,
        builder: (dialogContext) => AlertDialog(
          title: const Text('Ativar descoberta aberta?'),
          content: const Text(
            'Pessoas que não são suas conexões também verão seu nome e foto na '
            'descoberta por proximidade, dentro do seu raio configurado.',
          ),
          actions: [
            TextButton(
              key: const Key('proximity_reveal_level2_cancel'),
              onPressed: () => Navigator.of(dialogContext).pop(false),
              child: const Text('Cancelar'),
            ),
            TextButton(
              key: const Key('proximity_reveal_level2_confirm'),
              onPressed: () => Navigator.of(dialogContext).pop(true),
              child: const Text('Confirmar'),
            ),
          ],
        ),
      );
      if (confirmed != true) return;
    }
    onChanged(level);
  }

  @override
  Widget build(BuildContext context) {
    return SegmentedButton<RevealLevel>(
      key: const Key('proximity_reveal_level_selector'),
      segments: [
        for (final level in RevealLevel.values)
          ButtonSegment(value: level, label: Text(_labels[level]!)),
      ],
      selected: {value},
      onSelectionChanged: (selection) => _handleSelection(context, selection.first),
    );
  }
}
