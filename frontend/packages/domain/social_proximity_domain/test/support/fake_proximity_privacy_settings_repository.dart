import 'package:social_proximity_domain/social_proximity_domain.dart';

class FakeProximityPrivacySettingsRepository implements ProximityPrivacySettingsRepository {
  FakeProximityPrivacySettingsRepository({ProximityPrivacySettings? initial})
      : settings = initial ?? ProximityPrivacySettings.disabled();

  ProximityPrivacySettings settings;
  Object? fetchError;
  Object? updateError;
  int fetchCalls = 0;
  int grantConsentCalls = 0;
  int revokeConsentCalls = 0;

  /// Overridable clock so tests can control the consent timestamps
  /// [grantConsent] stamps, mirroring `SettingsService.GrantConsent`'s use
  /// of an injected clock server-side - the real
  /// `ProximityPrivacySettingsRepository` interface takes no arguments on
  /// [grantConsent] (it matches `POST /v1/presence/consent`'s empty body),
  /// so "what time was consent granted at" is a fake-only test seam, not
  /// part of the public contract.
  DateTime Function() now = DateTime.now;

  @override
  Future<ProximityPrivacySettings> fetch() async {
    fetchCalls++;
    if (fetchError != null) throw fetchError!;
    return settings;
  }

  @override
  Future<ProximityPrivacySettings> update(ProximityPrivacySettings newSettings) async {
    if (updateError != null) throw updateError!;
    settings = newSettings;
    return settings;
  }

  /// Mirrors `SettingsService.GrantConsent`: sets `enabled` + a fresh
  /// 6-month renewal window, never touches `paused`/`visibilityMode`.
  @override
  Future<ProximityPrivacySettings> grantConsent() async {
    grantConsentCalls++;
    if (updateError != null) throw updateError!;
    final effectiveNow = now();
    settings = settings.copyWith(
      enabled: true,
      consentGivenAt: effectiveNow,
      consentRenewalDueAt: DateTime(
        effectiveNow.year,
        effectiveNow.month + 6,
        effectiveNow.day,
        effectiveNow.hour,
        effectiveNow.minute,
        effectiveNow.second,
      ),
    );
    return settings;
  }

  /// Mirrors `SettingsService.RevokeConsent`: clears `enabled`, force-pauses.
  @override
  Future<ProximityPrivacySettings> revokeConsent() async {
    revokeConsentCalls++;
    if (updateError != null) throw updateError!;
    settings = settings.copyWith(enabled: false, paused: true);
    return settings;
  }
}
