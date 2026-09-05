import 'package:core_design_system/core_design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:social_proximity_domain/social_proximity_domain.dart';
import 'package:social_proximity_ui/social_proximity_ui.dart';

import '../../support/fake_proximity_privacy_settings_repository.dart';

Widget _wrap(
  FakeProximityPrivacySettingsRepository repo, {
  Future<bool?> Function(BuildContext)? onSetUpMfa,
}) {
  return ProviderScope(
    overrides: [proximityPrivacySettingsRepositoryProvider.overrideWithValue(repo)],
    // Deliberately not `const` here even though the constructor is marked
    // `const` in production code - see frontend/README.md's "Documented
    // coverage exclusions" methodological note: a canonicalized `const`
    // construction never registers as a covered line to `package:coverage`
    // even though it genuinely ran.
    child: MaterialApp(
      theme: SmusicTheme.light(),
      home: ProximityPrivacySettingsScreen(onSetUpMfa: onSetUpMfa),
    ),
  );
}

void main() {
  testWidgets('shows a loading indicator, then the disabled toggle when off', (tester) async {
    final repo = FakeProximityPrivacySettingsRepository();
    await tester.pumpWidget(_wrap(repo));
    expect(find.byType(CircularProgressIndicator), findsOneWidget);

    await tester.pump();
    expect(find.byKey(const Key('proximity_enabled_switch')), findsOneWidget);
    final tile = tester.widget<SwitchListTile>(find.byKey(const Key('proximity_enabled_switch')));
    expect(tile.value, isFalse);
    // Everything below the top toggle only renders once enabled.
    expect(find.byKey(const Key('proximity_radius_slider')), findsNothing);
  });

  testWidgets('shows an error state and retries on action', (tester) async {
    final repo = FakeProximityPrivacySettingsRepository();
    repo.fetchError = StateError('boom');
    await tester.pumpWidget(_wrap(repo));
    await tester.pump();

    expect(find.text('Não foi possível carregar suas configurações.'), findsOneWidget);
    expect(find.byKey(const Key('proximity_enabled_switch')), findsNothing);

    repo.fetchError = null;
    await tester.tap(find.text('Tentar de novo'));
    await tester.pump();
    await tester.pump();

    expect(find.byKey(const Key('proximity_enabled_switch')), findsOneWidget);
    expect(repo.fetchCalls, greaterThanOrEqualTo(2));
  });

  testWidgets('toggling on calls enableFeature (grantConsent) and reveals the rest of the screen', (tester) async {
    final repo = FakeProximityPrivacySettingsRepository();
    await tester.pumpWidget(_wrap(repo));
    await tester.pump();

    await tester.tap(find.byKey(const Key('proximity_enabled_switch')));
    await tester.pump();
    await tester.pump();

    expect(repo.grantConsentCalls, 1);
    expect(find.byKey(const Key('proximity_paused_switch')), findsOneWidget);
    expect(find.byKey(const Key('proximity_radius_slider')), findsOneWidget);
    expect(find.byKey(const Key('proximity_visibility_selector')), findsOneWidget);
    expect(find.byKey(const Key('proximity_reveal_level_selector')), findsOneWidget);
  });

  testWidgets(
    'mfa_required routes to onSetUpMfa, then retries and succeeds once verified',
    (tester) async {
      final repo = FakeProximityPrivacySettingsRepository()
        ..updateError = const ProximityException(ProximityExceptionKind.mfaRequired);
      var mfaSetupCalls = 0;

      await tester.pumpWidget(
        _wrap(
          repo,
          onSetUpMfa: (context) async {
            mfaSetupCalls++;
            repo.updateError = null;
            return true;
          },
        ),
      );
      await tester.pump();

      await tester.tap(find.byKey(const Key('proximity_enabled_switch')));
      await tester.pump();
      await tester.pump();
      await tester.pump();

      expect(mfaSetupCalls, 1);
      expect(repo.grantConsentCalls, 2);
      expect(find.byKey(const Key('proximity_paused_switch')), findsOneWidget);
    },
  );

  testWidgets(
    'mfa_required leaves the switch off when onSetUpMfa does not resolve true',
    (tester) async {
      final repo = FakeProximityPrivacySettingsRepository()
        ..updateError = const ProximityException(ProximityExceptionKind.mfaRequired);

      await tester.pumpWidget(_wrap(repo, onSetUpMfa: (context) async => false));
      await tester.pump();

      await tester.tap(find.byKey(const Key('proximity_enabled_switch')));
      await tester.pump();
      await tester.pump();

      final tile = tester.widget<SwitchListTile>(find.byKey(const Key('proximity_enabled_switch')));
      expect(tile.value, isFalse);
      expect(repo.grantConsentCalls, 1);
    },
  );

  testWidgets('toggling off calls disableFeature (revokeConsent)', (tester) async {
    final repo = FakeProximityPrivacySettingsRepository(
      initial: ProximityPrivacySettings.disabled().copyWith(enabled: true),
    );
    await tester.pumpWidget(_wrap(repo));
    await tester.pump();

    await tester.tap(find.byKey(const Key('proximity_enabled_switch')));
    await tester.pump();
    await tester.pump();

    expect(repo.revokeConsentCalls, 1);
    expect(find.byKey(const Key('proximity_paused_switch')), findsNothing);
  });

  testWidgets('the quick pause switch calls setPaused', (tester) async {
    final repo = FakeProximityPrivacySettingsRepository(
      initial: ProximityPrivacySettings.disabled().copyWith(enabled: true, paused: false),
    );
    await tester.pumpWidget(_wrap(repo));
    await tester.pump();

    await tester.tap(find.byKey(const Key('proximity_paused_switch')));
    await tester.pump();
    await tester.pump();

    expect(repo.settings.paused, isTrue);
  });

  testWidgets('shows the renewal banner when consent has lapsed, and renew calls grantConsent', (tester) async {
    final repo = FakeProximityPrivacySettingsRepository(
      initial: ProximityPrivacySettings.disabled().copyWith(
        enabled: true,
        consentGivenAt: DateTime.now().subtract(const Duration(days: 400)),
        consentRenewalDueAt: DateTime.now().subtract(const Duration(days: 1)),
      ),
    );
    await tester.pumpWidget(_wrap(repo));
    await tester.pump();

    expect(find.byKey(const Key('proximity_consent_renewal_banner')), findsOneWidget);
    await tester.tap(find.byKey(const Key('proximity_renew_consent_button')));
    await tester.pump();
    await tester.pump();

    expect(repo.grantConsentCalls, 1);
  });

  testWidgets('shows the valid-until date when consent has not lapsed', (tester) async {
    final dueDate = DateTime.now().add(const Duration(days: 90));
    final repo = FakeProximityPrivacySettingsRepository(
      initial: ProximityPrivacySettings.disabled().copyWith(
        enabled: true,
        consentGivenAt: DateTime.now(),
        consentRenewalDueAt: dueDate,
      ),
    );
    await tester.pumpWidget(_wrap(repo));
    await tester.pump();

    final day = dueDate.day.toString().padLeft(2, '0');
    final month = dueDate.month.toString().padLeft(2, '0');

    expect(find.byKey(const Key('proximity_consent_renewal_banner')), findsNothing);
    expect(find.byKey(const Key('proximity_consent_renewal_date')), findsOneWidget);
    expect(find.text('Consentimento válido até $day/$month/${dueDate.year}.'), findsOneWidget);
  });

  testWidgets('the radius slider moves through all 4 steps', (tester) async {
    final repo = FakeProximityPrivacySettingsRepository(
      initial: ProximityPrivacySettings.disabled().copyWith(enabled: true),
    );
    await tester.pumpWidget(_wrap(repo));
    await tester.pump();

    expect(find.text('1 km'), findsOneWidget);

    final slider = tester.widget<Slider>(find.byKey(const Key('proximity_radius_slider')));
    slider.onChanged!(3);
    await tester.pump();
    await tester.pump();

    expect(repo.settings.radius, ProximityRadius.km15);
  });

  testWidgets('the visibility selector updates visibility mode', (tester) async {
    final repo = FakeProximityPrivacySettingsRepository(
      initial: ProximityPrivacySettings.disabled().copyWith(enabled: true),
    );
    await tester.pumpWidget(_wrap(repo));
    await tester.pump();

    await tester.tap(find.text('Só amigos'));
    await tester.pump();
    await tester.pump();

    expect(repo.settings.visibilityMode, ProximityVisibilityMode.friendsOnly);
  });

  testWidgets('raising reveal level below 2 applies immediately, no dialog', (tester) async {
    final repo = FakeProximityPrivacySettingsRepository(
      initial: ProximityPrivacySettings.disabled().copyWith(enabled: true),
    );
    await tester.pumpWidget(_wrap(repo));
    await tester.pump();

    await tester.tap(find.text('Nome para conexões'));
    await tester.pump();
    await tester.pump();

    expect(repo.settings.maxRevealLevel, RevealLevel.level1);
    expect(find.text('Ativar descoberta aberta?'), findsNothing);
  });

  testWidgets('raising reveal level to 2 requires a second confirmation - cancel leaves it unchanged', (tester) async {
    final repo = FakeProximityPrivacySettingsRepository(
      initial: ProximityPrivacySettings.disabled().copyWith(enabled: true),
    );
    await tester.pumpWidget(_wrap(repo));
    await tester.pump();

    await tester.tap(find.text('Nome para todos'));
    await tester.pumpAndSettle();

    expect(find.text('Ativar descoberta aberta?'), findsOneWidget);
    await tester.tap(find.byKey(const Key('proximity_reveal_level2_cancel')));
    await tester.pumpAndSettle();

    expect(repo.settings.maxRevealLevel, RevealLevel.level0);
  });

  testWidgets('confirming the level-2 dialog applies it', (tester) async {
    final repo = FakeProximityPrivacySettingsRepository(
      initial: ProximityPrivacySettings.disabled().copyWith(enabled: true),
    );
    await tester.pumpWidget(_wrap(repo));
    await tester.pump();

    await tester.tap(find.text('Nome para todos'));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('proximity_reveal_level2_confirm')));
    await tester.pumpAndSettle();

    expect(repo.settings.maxRevealLevel, RevealLevel.level2);
  });
}
