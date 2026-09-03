import 'proximity_radius.dart';
import 'proximity_visibility_mode.dart';
import 'reveal_level.dart';

/// security.md section 1's full consent/visibility/radius/reveal/pause
/// model, and section 7's suggested schema
/// (`proximity_consent_enabled`, `proximity_consent_ts`,
/// `proximity_consent_renew_due`, `visibility_radius_m`, `reveal_level`,
/// `paused_bool`).
class ProximityPrivacySettings {
  const ProximityPrivacySettings({
    required this.enabled,
    required this.visibilityMode,
    required this.radius,
    required this.maxRevealLevel,
    required this.paused,
    this.consentGivenAt,
    this.consentRenewalDueAt,
  });

  /// security.md 1.1: the feature "nasce desligada para todo usuário,
  /// inclusive contas novas" - [disabled] is the only sane default.
  factory ProximityPrivacySettings.disabled() => const ProximityPrivacySettings(
        enabled: false,
        visibilityMode: ProximityVisibilityMode.invisible,
        radius: ProximityRadius.defaultValue,
        maxRevealLevel: RevealLevel.level0,
        paused: false,
      );

  /// Whether the base feature opt-in (security.md 1.1) is active. `false`
  /// means the feature is entirely off, independent of [paused] (which only
  /// makes sense to read when [enabled] is `true` - security.md 1.4's
  /// "pausar" is a *within-feature* quick toggle, not the opt-in itself).
  final bool enabled;

  final ProximityVisibilityMode visibilityMode;

  /// security.md 1.3: "limite de quem pode me ver" (not how far this user
  /// can see others - mutual visibility is the intersection, computed
  /// server-side).
  final ProximityRadius radius;

  /// The user's configured ceiling for what non-connections may see - see
  /// [RevealLevel]'s doc comment. Mutual connections always get at least
  /// [RevealLevel.level1] server-side regardless of this value.
  final RevealLevel maxRevealLevel;

  /// security.md 1.4: "Pausar descoberta" - quick, reversible, does not
  /// affect playback, does not require re-consenting when turned back on
  /// (unlike letting the 6-month consent lapse).
  final bool paused;

  final DateTime? consentGivenAt;

  /// security.md 1.1: "o consentimento expira a cada 6 meses". `null` only
  /// when [enabled] has never been turned on.
  final DateTime? consentRenewalDueAt;

  /// True once [consentRenewalDueAt] has passed - the UI must treat this
  /// the same as freshly-disabled for any *processing* decision (security.md
  /// 1.1: "Consentimento silenciosamente 'sempre válido' ... é um risco de
  /// auditoria"), while still surfacing a distinct "renew" prompt rather
  /// than the plain opt-in value screen, per the task's "indicação de
  /// quando o consentimento precisa ser renovado" requirement.
  bool needsConsentRenewal({DateTime? now}) {
    final due = consentRenewalDueAt;
    if (due == null) return false;
    return !(now ?? DateTime.now()).isBefore(due);
  }

  /// Whether the feed should actually be running right now: opted in,
  /// consent still valid, and not paused. `social_proximity_domain`'s
  /// `NearbyFeedNotifier` is the single place that reads this to decide
  /// whether to connect/disconnect the repository.
  bool get isActive => enabled && !paused && !needsConsentRenewal();

  ProximityPrivacySettings copyWith({
    bool? enabled,
    ProximityVisibilityMode? visibilityMode,
    ProximityRadius? radius,
    RevealLevel? maxRevealLevel,
    bool? paused,
    DateTime? consentGivenAt,
    DateTime? consentRenewalDueAt,
  }) {
    return ProximityPrivacySettings(
      enabled: enabled ?? this.enabled,
      visibilityMode: visibilityMode ?? this.visibilityMode,
      radius: radius ?? this.radius,
      maxRevealLevel: maxRevealLevel ?? this.maxRevealLevel,
      paused: paused ?? this.paused,
      consentGivenAt: consentGivenAt ?? this.consentGivenAt,
      consentRenewalDueAt: consentRenewalDueAt ?? this.consentRenewalDueAt,
    );
  }

  @override
  bool operator ==(Object other) =>
      other is ProximityPrivacySettings &&
      other.enabled == enabled &&
      other.visibilityMode == visibilityMode &&
      other.radius == radius &&
      other.maxRevealLevel == maxRevealLevel &&
      other.paused == paused &&
      other.consentGivenAt == consentGivenAt &&
      other.consentRenewalDueAt == consentRenewalDueAt;

  @override
  int get hashCode => Object.hash(
        enabled,
        visibilityMode,
        radius,
        maxRevealLevel,
        paused,
        consentGivenAt,
        consentRenewalDueAt,
      );
}
