import 'package:riverpod/riverpod.dart';
import 'package:social_proximity_domain/social_proximity_domain.dart';
import 'package:test/test.dart';

import '../support/fake_proximity_privacy_settings_repository.dart';

void main() {
  late FakeProximityPrivacySettingsRepository repository;
  late ProviderContainer container;

  setUp(() {
    repository = FakeProximityPrivacySettingsRepository();
    container = ProviderContainer(
      overrides: [proximityPrivacySettingsRepositoryProvider.overrideWithValue(repository)],
    );
    addTearDown(container.dispose);
  });

  test('build() fetches from the repository', () async {
    repository.settings = ProximityPrivacySettings.disabled().copyWith(radius: ProximityRadius.km5);
    final settings = await container.read(proximityPrivacySettingsProvider.future);
    expect(settings.radius, ProximityRadius.km5);
    expect(repository.fetchCalls, 1);
  });

  test(
    'enableFeature on a true first activation (no prior consent) defaults visibility to everyone',
    () async {
      repository.now = () => DateTime(2026, 1, 15, 10);
      // No prior configuration: repository starts at ProximityPrivacySettings.disabled(),
      // whose consentGivenAt is null - this is a brand-new account that has never
      // granted proximity consent before.
      await container.read(proximityPrivacySettingsProvider.future);
      await container.read(proximityPrivacySettingsProvider.notifier).enableFeature();

      final settings = container.read(proximityPrivacySettingsProvider).value!;
      expect(settings.enabled, isTrue);
      expect(settings.paused, isFalse);
      expect(settings.visibilityMode, ProximityVisibilityMode.everyone);
      expect(settings.consentGivenAt, DateTime(2026, 1, 15, 10));
      expect(settings.consentRenewalDueAt, DateTime(2026, 7, 15, 10));
      expect(repository.grantConsentCalls, 1);
    },
  );

  test(
    'enableFeature on reactivation preserves a previously saved visibility (does not reset it)',
    () async {
      // Mirrors a returning user: they granted consent before (consentGivenAt is
      // already set), explicitly chose friendsOnly, then paused/disabled the
      // feature - backend's RevokeConsent never resets presence_visibility, so
      // friendsOnly is still what fetch() returns here.
      repository.settings = ProximityPrivacySettings.disabled().copyWith(
        visibilityMode: ProximityVisibilityMode.friendsOnly,
        consentGivenAt: DateTime(2025, 6, 1, 9),
        consentRenewalDueAt: DateTime(2025, 12, 1, 9),
      );
      repository.now = () => DateTime(2026, 1, 15, 10);
      await container.read(proximityPrivacySettingsProvider.future);
      await container.read(proximityPrivacySettingsProvider.notifier).enableFeature();

      final settings = container.read(proximityPrivacySettingsProvider).value!;
      expect(settings.enabled, isTrue);
      expect(settings.paused, isFalse);
      expect(
        settings.visibilityMode,
        ProximityVisibilityMode.friendsOnly,
        reason: 'reactivating must preserve the last explicit visibility choice, not reset it',
      );
      expect(settings.consentGivenAt, DateTime(2026, 1, 15, 10));
      expect(settings.consentRenewalDueAt, DateTime(2026, 7, 15, 10));
      expect(repository.grantConsentCalls, 1);
    },
  );

  test('enableFeature rolls month overflow into the next year', () async {
    repository.now = () => DateTime(2026, 9, 1);
    await container.read(proximityPrivacySettingsProvider.future);
    await container.read(proximityPrivacySettingsProvider.notifier).enableFeature();
    final settings = container.read(proximityPrivacySettingsProvider).value!;
    expect(settings.consentRenewalDueAt, DateTime(2027, 3, 1));
  });

  test('disableFeature revokes consent (which also force-pauses)', () async {
    repository.settings = ProximityPrivacySettings.disabled().copyWith(enabled: true, paused: false);
    await container.read(proximityPrivacySettingsProvider.future);
    await container.read(proximityPrivacySettingsProvider.notifier).disableFeature();
    final settings = container.read(proximityPrivacySettingsProvider).value!;
    expect(settings.enabled, isFalse);
    expect(settings.paused, isTrue);
    expect(repository.revokeConsentCalls, 1);
  });

  test('setPaused toggles the quick pause', () async {
    await container.read(proximityPrivacySettingsProvider.future);
    await container.read(proximityPrivacySettingsProvider.notifier).setPaused(true);
    expect(container.read(proximityPrivacySettingsProvider).value!.paused, isTrue);
  });

  test('setVisibilityMode updates visibility', () async {
    await container.read(proximityPrivacySettingsProvider.future);
    await container
        .read(proximityPrivacySettingsProvider.notifier)
        .setVisibilityMode(ProximityVisibilityMode.friendsOnly);
    expect(
      container.read(proximityPrivacySettingsProvider).value!.visibilityMode,
      ProximityVisibilityMode.friendsOnly,
    );
  });

  test('setRadius updates radius', () async {
    await container.read(proximityPrivacySettingsProvider.future);
    await container.read(proximityPrivacySettingsProvider.notifier).setRadius(ProximityRadius.m150);
    expect(container.read(proximityPrivacySettingsProvider).value!.radius, ProximityRadius.m150);
  });

  test('setMaxRevealLevel updates the reveal ceiling', () async {
    await container.read(proximityPrivacySettingsProvider.future);
    await container
        .read(proximityPrivacySettingsProvider.notifier)
        .setMaxRevealLevel(RevealLevel.level2);
    expect(
      container.read(proximityPrivacySettingsProvider).value!.maxRevealLevel,
      RevealLevel.level2,
    );
  });

  test('renewConsent delegates to grantConsent', () async {
    await container.read(proximityPrivacySettingsProvider.future);
    await container.read(proximityPrivacySettingsProvider.notifier).renewConsent();
    expect(repository.grantConsentCalls, 1);
  });

  test('a failed mutation surfaces as AsyncError', () async {
    await container.read(proximityPrivacySettingsProvider.future);
    repository.updateError = StateError('network down');
    await container.read(proximityPrivacySettingsProvider.notifier).setPaused(true);
    expect(container.read(proximityPrivacySettingsProvider).hasError, isTrue);
  });

  test('a failed enableFeature (grantConsent step) surfaces as AsyncError', () async {
    await container.read(proximityPrivacySettingsProvider.future);
    repository.updateError = StateError('network down');
    await container.read(proximityPrivacySettingsProvider.notifier).enableFeature();
    expect(container.read(proximityPrivacySettingsProvider).hasError, isTrue);
    expect(repository.grantConsentCalls, 1);
  });
}
