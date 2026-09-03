import '../entities/proximity_privacy_settings.dart';

/// Implemented by `social_proximity_data`'s
/// `HttpProximityPrivacySettingsRepository` against
/// `backend/internal/presence/api/handlers.go`'s real, confirmed REST
/// contract: `GET/PUT /v1/presence/settings`, `POST/DELETE
/// /v1/presence/consent` (see that class's doc comment for the endpoint
/// list and `social_proximity_data`'s `ProximityDtos` for the verified
/// field-level mapping).
abstract interface class ProximityPrivacySettingsRepository {
  Future<ProximityPrivacySettings> fetch();

  /// `PUT /v1/presence/settings`: visibility mode, radius, max reveal
  /// level, paused. Never touches consent - see [grantConsent]/
  /// [revokeConsent].
  Future<ProximityPrivacySettings> update(ProximityPrivacySettings settings);

  /// `POST /v1/presence/consent` (empty body): security.md 1.1's initial
  /// opt-in *and* its 6-month re-confirmation are the same backend
  /// operation - `SettingsService.GrantConsent` "enables (or renews)" -
  /// so this one method backs both `ProximityPrivacySettingsNotifier.
  /// enableFeature` and `.renewConsent`. Per that service's doc comment, it
  /// never implicitly changes visibility/paused - granting consent alone
  /// does not make the caller discoverable.
  Future<ProximityPrivacySettings> grantConsent();

  /// `DELETE /v1/presence/consent`: security.md 1.1 §5º/1.7's "revogação: 1
  /// toque, efeito imediato". Per `SettingsService.RevokeConsent`, this
  /// also force-pauses server-side as defense in depth.
  Future<ProximityPrivacySettings> revokeConsent();
}
