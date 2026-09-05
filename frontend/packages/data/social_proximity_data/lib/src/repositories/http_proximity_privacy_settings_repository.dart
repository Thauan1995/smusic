import 'package:core_networking/core_networking.dart';
import 'package:social_proximity_domain/social_proximity_domain.dart';

import '../dto/proximity_dtos.dart';

/// `social_proximity_domain.ProximityPrivacySettingsRepository`
/// implementation against `backend/internal/presence/api/handlers.go` -
/// mounted on smusic-core (not presence-service, per that file's package
/// doc: settings/consent/pause/block are low-frequency, Postgres-backed
/// account configuration, not the latency-sensitive presence data plane).
///
/// Real, confirmed endpoints (this class originally guessed
/// `GET/POST /v1/presence/privacy` against an undocumented resource before
/// `backend/internal/presence` existed - see `ProximityDtos`'s doc comment
/// for the full list of what changed and why):
/// - `GET  /v1/presence/settings` -> [ProximityDtos.settingsFromJson].
/// - `PUT  /v1/presence/settings` (body: [ProximityDtos.settingsToJson]) ->
///   same shape.
/// - `POST /v1/presence/consent` (empty body) -> same shape - grants *or*
///   renews consent (`SettingsService.GrantConsent`), backing both
///   [grantConsent] here.
/// - `DELETE /v1/presence/consent` (empty body) -> same shape - revokes
///   consent and force-pauses server-side (`SettingsService.
///   RevokeConsent`), backing [revokeConsent].
///
/// `handlers.go` also exposes dedicated `POST /v1/presence/pause`/
/// `/resume` and `POST/DELETE /v1/presence/blocks/{user_id}` routes.
/// Deliberately NOT wired here: [update] already reaches the same pause
/// effect via `PUT /v1/presence/settings`'s `paused` field (task scope item
/// 2's quick pause toggle only needs *a* working path, not every possible
/// one), and a block-list UI is not part of this slice's explicit scope
/// (task scope items 1-7 do not list one) - both flagged as a deliberate,
/// documented scope decision rather than a silent omission.
class HttpProximityPrivacySettingsRepository implements ProximityPrivacySettingsRepository {
  HttpProximityPrivacySettingsRepository(this._client);

  final ApiClient _client;

  @override
  Future<ProximityPrivacySettings> fetch() {
    return _wrap(() async {
      final response = await _client.get('/v1/presence/settings');
      return ProximityDtos.settingsFromJson(response);
    });
  }

  @override
  Future<ProximityPrivacySettings> update(ProximityPrivacySettings settings) {
    return _wrap(() async {
      final response = await _client.put(
        '/v1/presence/settings',
        data: ProximityDtos.settingsToJson(settings),
      );
      return ProximityDtos.settingsFromJson(response);
    });
  }

  @override
  Future<ProximityPrivacySettings> grantConsent() {
    return _wrap(() async {
      final response = await _client.post('/v1/presence/consent');
      return ProximityDtos.settingsFromJson(response);
    });
  }

  @override
  Future<ProximityPrivacySettings> revokeConsent() {
    return _wrap(() async {
      final response = await _client.delete('/v1/presence/consent');
      return ProximityDtos.settingsFromJson(response);
    });
  }

  Future<T> _wrap<T>(Future<T> Function() body) async {
    try {
      return await body();
    } on ApiException catch (e) {
      throw ProximityException(_kindFor(e), message: e.message);
    }
  }

  ProximityExceptionKind _kindFor(ApiException e) {
    if (e.code == 'mfa_required') return ProximityExceptionKind.mfaRequired;
    if (e.isUnauthorized) return ProximityExceptionKind.unauthorized;
    if (e.isNetworkError) return ProximityExceptionKind.network;
    return ProximityExceptionKind.unknown;
  }
}
