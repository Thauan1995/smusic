import 'package:social_proximity_domain/social_proximity_domain.dart';
import 'package:test/test.dart';

void main() {
  test('disabled() is opted-out, invisible, default radius/level0, not paused', () {
    final settings = ProximityPrivacySettings.disabled();
    expect(settings.enabled, isFalse);
    expect(settings.visibilityMode, ProximityVisibilityMode.invisible);
    expect(settings.radius, ProximityRadius.defaultValue);
    expect(settings.maxRevealLevel, RevealLevel.level0);
    expect(settings.paused, isFalse);
    expect(settings.consentGivenAt, isNull);
    expect(settings.consentRenewalDueAt, isNull);
    expect(settings.isActive, isFalse);
  });

  group('needsConsentRenewal', () {
    test('false when consentRenewalDueAt is null', () {
      expect(ProximityPrivacySettings.disabled().needsConsentRenewal(), isFalse);
    });

    test('false while renewal date is still in the future', () {
      final settings = ProximityPrivacySettings.disabled().copyWith(
        enabled: true,
        consentRenewalDueAt: DateTime(2026, 6, 1),
      );
      expect(settings.needsConsentRenewal(now: DateTime(2026, 1, 1)), isFalse);
    });

    test('true once the renewal date has passed (or is exactly now)', () {
      final settings = ProximityPrivacySettings.disabled().copyWith(
        enabled: true,
        consentRenewalDueAt: DateTime(2026, 1, 1),
      );
      expect(settings.needsConsentRenewal(now: DateTime(2026, 1, 1)), isTrue);
      expect(settings.needsConsentRenewal(now: DateTime(2026, 6, 1)), isTrue);
    });
  });

  group('isActive', () {
    ProximityPrivacySettings active({
      bool paused = false,
      DateTime? renewalDue,
    }) =>
        ProximityPrivacySettings.disabled().copyWith(
          enabled: true,
          paused: paused,
          consentRenewalDueAt: renewalDue ?? DateTime.now().add(const Duration(days: 30)),
        );

    test('true when enabled, not paused, consent not lapsed', () {
      expect(active().isActive, isTrue);
    });

    test('false when not enabled', () {
      expect(ProximityPrivacySettings.disabled().isActive, isFalse);
    });

    test('false when paused', () {
      expect(active(paused: true).isActive, isFalse);
    });

    test('false when consent has lapsed', () {
      final lapsed = active(renewalDue: DateTime.now().subtract(const Duration(days: 1)));
      expect(lapsed.isActive, isFalse);
    });
  });

  test('copyWith only overrides provided fields', () {
    final base = ProximityPrivacySettings.disabled();
    final updated = base.copyWith(radius: ProximityRadius.km15);
    expect(updated.radius, ProximityRadius.km15);
    expect(updated.enabled, base.enabled);
    expect(updated.visibilityMode, base.visibilityMode);
  });

  test('equality/hashCode compare all fields', () {
    final a = ProximityPrivacySettings.disabled();
    final b = ProximityPrivacySettings.disabled();
    final c = a.copyWith(enabled: true);
    expect(a, b);
    expect(a.hashCode, b.hashCode);
    expect(a, isNot(c));
  });
}
