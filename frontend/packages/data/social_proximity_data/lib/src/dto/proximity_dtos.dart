import 'package:social_proximity_domain/social_proximity_domain.dart';

/// Mapping between the presence feature's real wire shapes and
/// `social_proximity_domain` entities.
///
/// This file was originally written against backend-go.md section 4's
/// *illustrative* WS snippet and security.md section 7's *field list* only
/// (before `backend/internal/presence` existed), so several mappings were
/// flagged as ASSUMPTIONs needing confirmation. `backend/internal/presence`
/// (the real, parallel-track implementation) now exists and is the
/// authoritative contract - the mappings below are verified line-by-line
/// against it, not guessed:
///
/// - **`distance_bucket` wire codes**: `backend/internal/presence/ws/
///   protocol.go`'s `bucketCode` function is the actual source - the 4
///   codes are `under_150m`, `150m_1km`, `1km_5km`, `5km_15km` (NOT the
///   `lt_150m` this file originally assumed - that was a real, confirmed
///   bug, fixed here).
/// - **`reveal_level` on each `users[]` entry**: still genuinely
///   unspecified on the wire - `protocol.go`'s `userFrame` has no
///   `reveal_level` field at all, only `display_name`/`avatar_url`
///   presence-or-absence (the backend nulls those out server-side per
///   reveal level, but never tells the client *which* level applied).
///   [_revealLevelFromJson] still reads an optional `reveal_level` int if a
///   future backend revision adds one, and otherwise infers level 0 vs.
///   level 1+ from identity-field presence (never infers level 2 from
///   silence - see the method doc). Flagged for the backend specialist:
///   consider adding an explicit `reveal_level` to `userFrame` so the
///   client's defense-in-depth inference isn't needed.
/// - **`now_playing` on an *other* user's entry**: `protocol.go`'s
///   `nowPlayingOut` only carries `track_id` (no human-readable
///   `title`/`artist_name` the client could render as "ouvindo *Track*"
///   without a separate catalog lookup this slice doesn't implement).
///   Flagged for the backend specialist: either enrich `nowPlayingOut` with
///   `title`/`artist_name`, or confirm the client is expected to resolve
///   `track_id` via `library_domain`'s catalog repository itself (cross-
///   feature dependency, out of this slice's scope either way - see the
///   task report). [_nowPlayingFromJson] reads `title`/`artist_name` if
///   present (forward-compatible with the first option) and otherwise
///   treats the entry as track-less (never renders a bare id as a title).
/// - **REST privacy settings resource**: `backend/internal/presence/api/
///   handlers.go` (mounted on smusic-core, not presence-service - see that
///   file's package doc) is the real, confirmed contract:
///   `GET/PUT /v1/presence/settings`, `POST/DELETE /v1/presence/consent`,
///   `POST /v1/presence/pause`/`/resume`. The JSON field names below
///   (`presence_visibility`, `proximity_consent_enabled`,
///   `visibility_radius_m`, `reveal_level`, `paused`, ...) are copied
///   verbatim from `handlers.go`'s `settingsResponse`/
///   `updateSettingsRequest` structs - this file originally guessed
///   different names (`enabled`, `visibility_mode`, `radius_m`,
///   `max_reveal_level`) against an undocumented endpoint; those were real,
///   confirmed bugs (a PUT with the old field names would 400, since
///   `httpx.DecodeJSON` calls `DisallowUnknownFields()`), fixed here.
/// - **`presence_visibility` wire values differ between WS and REST**: a
///   confirmed, real backend asymmetry (not a frontend bug) - the WS
///   `visibility` frame's `mode` uses `"visible"` for the permissive value
///   (`nearby_service.go`'s doc comment, matching backend-go.md's
///   illustrative snippet), while the REST `presence_visibility` field uses
///   `"everyone"` for the same concept (`domain.go`'s
///   `VisibilityEveryone = "everyone"`). [visibilityModeToWire]/
///   [visibilityModeFromWire] stay `"visible"`-based for the WS frame (used
///   by `WebSocketProximityFeedRepository.setVisibility`);
///   [presenceVisibilityToWire]/[presenceVisibilityFromWire] are the
///   separate `"everyone"`-based pair for the REST settings resource (used
///   by `HttpProximityPrivacySettingsRepository`). Flagged for the backend
///   specialist as worth reconciling into one literal, but until then the
///   client intentionally speaks both dialects rather than guessing one is
///   wrong.
/// - **`presence_share_track`**: a real field on `handlers.go`'s
///   `settingsResponse`/`updateSettingsRequest` with no equivalent on
///   `social_proximity_domain`'s `ProximityPrivacySettings` and no UI
///   control in this slice's explicit scope (task scope item 2 lists radius
///   slider, reveal-level selector, pause toggle, activation toggle,
///   consent-renewal indication - not a "share track" toggle). Deliberately
///   left unmapped in both directions: [settingsFromJson] ignores the key
///   on read, and [settingsToJson] never emits it, so a client-initiated
///   `PUT` never touches the field (the backend's optional-pointer
///   `UpdateSettingsInput.PresenceShareTrack` means an omitted key leaves
///   the stored value unchanged) - a documented scope decision, not a
///   silent data-loss bug.
abstract final class ProximityDtos {
  static List<NearbyListener> listenersFromUsersJson(List<dynamic> usersJson) {
    return usersJson
        .whereType<Map<String, dynamic>>()
        .map(_listenerFromJson)
        .toList(growable: false);
  }

  static NearbyListener _listenerFromJson(Map<String, dynamic> json) {
    final displayName = json['display_name'] as String?;
    final avatarUrl = json['avatar_url'] as String?;
    return NearbyListener(
      userId: json['user_id'] as String,
      distanceBucket: _distanceBucketFromWire(json['distance_bucket']),
      revealLevel: _revealLevelFromJson(
        json['reveal_level'],
        hasIdentity: displayName != null || avatarUrl != null,
      ),
      displayName: displayName,
      avatarUrl: avatarUrl,
      nowPlaying: _nowPlayingFromJson(json['now_playing']),
    );
  }

  static RevealLevel _revealLevelFromJson(Object? raw, {required bool hasIdentity}) {
    if (raw is int) {
      switch (raw) {
        case 0:
          return RevealLevel.level0;
        case 1:
          return RevealLevel.level1;
        case 2:
          return RevealLevel.level2;
      }
    }
    return hasIdentity ? RevealLevel.level1 : RevealLevel.level0;
  }

  /// `backend/internal/presence/ws/protocol.go`'s `bucketCode` function -
  /// the actual wire codes (verified against that source, not guessed).
  static const Map<String, DistanceBucket> _distanceBucketWireCodes = {
    'under_150m': DistanceBucket.veryClose,
    '150m_1km': DistanceBucket.neighborhood,
    '1km_5km': DistanceBucket.region,
    '5km_15km': DistanceBucket.city,
  };

  /// Defense in depth: an unrecognized value (including, hypothetically, a
  /// raw numeric distance the backend was never supposed to send per
  /// security.md section 1.2) never throws and is never echoed back
  /// verbatim - it falls back to [DistanceBucket.city], the *least*
  /// precise bucket, so a contract bug degrades privacy-safely.
  static DistanceBucket _distanceBucketFromWire(Object? value) {
    if (value is String) {
      final mapped = _distanceBucketWireCodes[value];
      if (mapped != null) return mapped;
    }
    return DistanceBucket.city;
  }

  static NowPlayingSnapshot? _nowPlayingFromJson(Object? raw) {
    if (raw is! Map<String, dynamic>) return null;
    final title = raw['title'] as String?;
    if (title == null) return null;
    return NowPlayingSnapshot(trackTitle: title, artistName: raw['artist_name'] as String?);
  }

  /// The client's own outbound `now_playing` shape
  /// (`ws/protocol.go`'s `nowPlayingIn`: `{track_id, position_ms}`),
  /// distinct from [_nowPlayingFromJson] above (see the class doc
  /// comment's flagged asymmetry). `social_proximity_domain`'s
  /// `NowPlayingSnapshot` doesn't carry a `trackId`/position -
  /// `WebSocketProximityFeedRepository` keeps those alongside the snapshot
  /// separately (see its doc comment) and passes them into this method
  /// directly.
  static Map<String, dynamic> updateNowPlayingToJson({
    required String trackId,
    required int positionMs,
  }) {
    return {'track_id': trackId, 'position_ms': positionMs};
  }

  // ---- WS `visibility` frame mode (backend/internal/presence/ws) ----
  //
  // `nearby_service.go`'s `SetVisibility` doc comment: mode is one of
  // "visible" | "invisible" | "friends_only" on the WS wire - distinct
  // literal set from the REST `presence_visibility` field below (see class
  // doc comment).

  static ProximityVisibilityMode visibilityModeFromWire(Object? value) {
    switch (value) {
      case 'invisible':
        return ProximityVisibilityMode.invisible;
      case 'friends_only':
        return ProximityVisibilityMode.friendsOnly;
      case 'visible':
        return ProximityVisibilityMode.everyone;
      default:
        return ProximityVisibilityMode.invisible;
    }
  }

  static String visibilityModeToWire(ProximityVisibilityMode mode) {
    switch (mode) {
      case ProximityVisibilityMode.invisible:
        return 'invisible';
      case ProximityVisibilityMode.friendsOnly:
        return 'friends_only';
      case ProximityVisibilityMode.everyone:
        return 'visible';
    }
  }

  // ---- REST `presence_visibility` (backend/internal/presence/domain.go:
  // VisibilityInvisible/VisibilityFriendsOnly/VisibilityEveryone) ----

  static ProximityVisibilityMode presenceVisibilityFromWire(Object? value) {
    switch (value) {
      case 'invisible':
        return ProximityVisibilityMode.invisible;
      case 'friends_only':
        return ProximityVisibilityMode.friendsOnly;
      case 'everyone':
        return ProximityVisibilityMode.everyone;
      default:
        return ProximityVisibilityMode.invisible;
    }
  }

  static String presenceVisibilityToWire(ProximityVisibilityMode mode) {
    switch (mode) {
      case ProximityVisibilityMode.invisible:
        return 'invisible';
      case ProximityVisibilityMode.friendsOnly:
        return 'friends_only';
      case ProximityVisibilityMode.everyone:
        return 'everyone';
    }
  }

  // ---- privacy settings REST payload (backend/internal/presence/api/
  // handlers.go's settingsResponse / updateSettingsRequest) ----

  /// Parses the shape common to `GET /v1/presence/settings`,
  /// `PUT /v1/presence/settings`, `POST /v1/presence/consent` and
  /// `DELETE /v1/presence/consent` responses (`handlers.go`'s
  /// `toSettingsResponse` - all four endpoints return the same
  /// `settingsResponse` shape).
  static ProximityPrivacySettings settingsFromJson(Map<String, dynamic> json) {
    return ProximityPrivacySettings(
      enabled: json['proximity_consent_enabled'] as bool? ?? false,
      visibilityMode: presenceVisibilityFromWire(json['presence_visibility']),
      radius: _radiusFromMeters(json['visibility_radius_m']),
      maxRevealLevel: _revealLevelFromSettingsJson(json['reveal_level']),
      paused: json['paused'] as bool? ?? false,
      consentGivenAt: _parseDate(json['proximity_consent_ts']),
      consentRenewalDueAt: _parseDate(json['proximity_consent_renew_due']),
    );
  }

  /// Body for `PUT /v1/presence/settings` (`handlers.go`'s
  /// `updateSettingsRequest`) - deliberately only the 4 fields that struct
  /// actually declares. `proximity_consent_enabled`/`_ts`/`_renew_due` are
  /// NEVER included here: they are not settable via this endpoint at all
  /// (`updateSettingsRequest` has no such fields), consent is exclusively
  /// managed via `POST`/`DELETE /v1/presence/consent`
  /// ([HttpProximityPrivacySettingsRepository.grantConsent]/
  /// [HttpProximityPrivacySettingsRepository.revokeConsent]). Including an
  /// unrecognized key here would not be silently ignored - `handlers.go`'s
  /// `updateSettings` calls `httpx.DecodeJSON`, which sets
  /// `json.Decoder.DisallowUnknownFields()`, so a stray key 400s the whole
  /// request. This was a real, confirmed bug in this file's original
  /// version (which sent `enabled`/`visibility_mode`/`radius_m`/
  /// `max_reveal_level`/consent-date keys - none of which
  /// `updateSettingsRequest` recognizes), fixed here.
  static Map<String, dynamic> settingsToJson(ProximityPrivacySettings settings) {
    return {
      'presence_visibility': presenceVisibilityToWire(settings.visibilityMode),
      'visibility_radius_m': settings.radius.meters,
      'reveal_level': settings.maxRevealLevel.index,
      'paused': settings.paused,
    };
  }

  static ProximityRadius _radiusFromMeters(Object? value) {
    final meters = value is int ? value : (value is double ? value.round() : null);
    switch (meters) {
      case 150:
        return ProximityRadius.m150;
      case 1000:
        return ProximityRadius.km1;
      case 5000:
        return ProximityRadius.km5;
      case 15000:
        return ProximityRadius.km15;
      default:
        return ProximityRadius.defaultValue;
    }
  }

  static RevealLevel _revealLevelFromSettingsJson(Object? value) {
    switch (value) {
      case 0:
        return RevealLevel.level0;
      case 1:
        return RevealLevel.level1;
      case 2:
        return RevealLevel.level2;
      default:
        return RevealLevel.level0;
    }
  }

  static DateTime? _parseDate(Object? value) {
    if (value is String) return DateTime.tryParse(value);
    return null;
  }
}
