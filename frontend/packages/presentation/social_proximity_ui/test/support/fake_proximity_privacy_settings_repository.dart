import 'package:social_proximity_domain/social_proximity_domain.dart';

/// Local copy of `social_proximity_domain`'s test-only fake - see
/// `FakeProximityFeedRepository`'s doc comment for why this isn't shared
/// across package boundaries via a `test/` import.
class FakeProximityPrivacySettingsRepository implements ProximityPrivacySettingsRepository {
  FakeProximityPrivacySettingsRepository({ProximityPrivacySettings? initial})
      : settings = initial ?? ProximityPrivacySettings.disabled();

  ProximityPrivacySettings settings;
  Object? updateError;
  Object? fetchError;
  int grantConsentCalls = 0;
  int revokeConsentCalls = 0;
  int updateCalls = 0;
  int fetchCalls = 0;
  DateTime Function() now = DateTime.now;

  @override
  Future<ProximityPrivacySettings> fetch() async {
    fetchCalls++;
    if (fetchError != null) throw fetchError!;
    return settings;
  }

  @override
  Future<ProximityPrivacySettings> update(ProximityPrivacySettings newSettings) async {
    updateCalls++;
    if (updateError != null) throw updateError!;
    settings = newSettings;
    return settings;
  }

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
      ),
    );
    return settings;
  }

  @override
  Future<ProximityPrivacySettings> revokeConsent() async {
    revokeConsentCalls++;
    if (updateError != null) throw updateError!;
    settings = settings.copyWith(enabled: false, paused: true);
    return settings;
  }
}
