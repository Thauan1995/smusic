package presence

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"smusic/backend/internal/platform/clock"
	"smusic/backend/internal/platform/idgen"
)

// NearbyResult is what a client is allowed to learn about one nearby user —
// security.md §1.2's bucket-only contract plus §1.6's reveal-level-gated
// identity. There is no field here (nor anywhere upstream of it) capable of
// carrying a coordinate, geohash, or exact distance: DistanceBucket is the
// only positional information in the type.
type NearbyResult struct {
	UserID      string
	DisplayName string // "" unless reveal level >= 1 for this viewer/target pair
	AvatarURL   string // "" unless reveal level >= 1
	Bucket      DistanceBucket
	TrackID     string // "" unless the target opted into presence_share_track
}

// NearbyService is the WS-facing query engine: it is where every single
// control from security.md §1 is actually enforced, in one place, so it can
// be audited and tested as one unit (per the task's explicit ask for
// dedicated privacy-invariant tests). internal/presence/ws and
// cmd/presence-server call this; they contain no privacy logic of their
// own.
type NearbyService struct {
	settings     PrivacySettingsRepository
	blocks       BlockRepository
	follows      FollowChecker
	geo          GeoIndex
	audit        AuditLogRepository
	profiles     ProfileResolver
	pairLimiter  RateLimiter
	dailyLimiter RateLimiter
	jitter       Jitterer
	clock        clock.Clock
	ids          idgen.Generator
}

// NewNearbyService constructs a NearbyService from its dependencies —
// every one an interface, per backend-go.md §7, so the entire privacy
// pipeline is unit-testable with in-memory fakes and no real Postgres/Redis.
func NewNearbyService(
	settings PrivacySettingsRepository,
	blocks BlockRepository,
	follows FollowChecker,
	geo GeoIndex,
	audit AuditLogRepository,
	profiles ProfileResolver,
	pairLimiter, dailyLimiter RateLimiter,
	jitter Jitterer,
	clk clock.Clock,
	ids idgen.Generator,
) *NearbyService {
	return &NearbyService{
		settings: settings, blocks: blocks, follows: follows, geo: geo, audit: audit, profiles: profiles,
		pairLimiter: pairLimiter, dailyLimiter: dailyLimiter, jitter: jitter, clock: clk, ids: ids,
	}
}

// searchPadding widens the geo-index radius query beyond the requester's
// configured visibility radius by up to two jitter radii — the requester's
// own stored position and the candidate's stored position can each be
// displaced by up to JitterRadiusM in an adversarial direction, so a
// genuine in-range candidate could otherwise be missed by a Search() call
// bounded exactly at the configured radius. The subsequent per-candidate
// distance check (against the true minimum of both users' radii, computed
// from the same jittered positions) is what actually enforces the§1.3
// mutual-radius gate — this padding only affects which candidates are
// *considered*, never which are *shown*.
const searchPadding = 2 * JitterRadiusM

// settingsOrDefault reads settings for userID, treating "never configured"
// as DefaultPrivacySettings rather than an error — consistent with
// SettingsService.Get.
func (n *NearbyService) settingsOrDefault(ctx context.Context, userID string) (PrivacySettings, error) {
	st, err := n.settings.Get(ctx, userID)
	if err != nil {
		if errors.Is(err, ErrSettingsNotFound) {
			return DefaultPrivacySettings(userID), nil
		}
		return PrivacySettings{}, fmt.Errorf("presence: get settings: %w", err)
	}
	return st, nil
}

// CheckConsent is called by the WS handshake handler BEFORE upgrading the
// connection (item 7 of the task: "o backend deve REJEITAR conexões WS de
// presença de usuários sem consentimento ativo e válido"). It never mutates
// state.
func (n *NearbyService) CheckConsent(ctx context.Context, userID string) error {
	st, err := n.settingsOrDefault(ctx, userID)
	if err != nil {
		return err
	}
	return checkConsent(st, n.clock.Now())
}

// checkConsent distinguishes "never granted" from "granted but expired" —
// both WS-handshake rejection (CheckConsent) and every mid-session
// re-validation (ApplyUpdate, ApplyHeartbeat) use this same rule so a
// client always gets a consistent, specific reason.
func checkConsent(st PrivacySettings, now time.Time) error {
	if !st.ProximityConsentEnabled {
		return ErrConsentRequired
	}
	if !st.HasActiveConsent(now) {
		return ErrConsentExpired
	}
	return nil
}

// ApplyUpdate processes one client "update" frame (backend-go.md §4): it
// jitters the raw coordinate exactly once (the raw value is a local
// variable in this call and is never passed anywhere else, never stored —
// security.md §1.5/§4.3), upserts the requester's own (jittered-only)
// presence entry with a renewed TTL unless they're paused/invisible, and
// returns their current nearby list, filtered through every control in
// security.md §1.
func (n *NearbyService) ApplyUpdate(ctx context.Context, requesterID string, rawLat, rawLon float64, trackID string, ttl time.Duration) ([]NearbyResult, error) {
	requesterSettings, err := n.settingsOrDefault(ctx, requesterID)
	if err != nil {
		return nil, err
	}
	now := n.clock.Now()
	if err := checkConsent(requesterSettings, now); err != nil {
		return nil, err
	}

	jittered, err := n.jitter.Jitter(GeoPosition{Lat: rawLat, Lon: rawLon})
	if err != nil {
		return nil, fmt.Errorf("presence: apply update: %w", err)
	}

	if requesterSettings.Paused || requesterSettings.PresenceVisibility == VisibilityInvisible {
		_ = n.geo.Remove(ctx, requesterID) // best-effort; see SettingsService.removeFromIndex's rationale
	} else {
		entry := PresenceEntry{
			UserID:     requesterID,
			Position:   jittered,
			TrackID:    trackID,
			ShareTrack: requesterSettings.PresenceShareTrack,
			Visibility: requesterSettings.PresenceVisibility,
			UpdatedAt:  now,
		}
		if err := n.geo.Upsert(ctx, entry, ttl); err != nil {
			return nil, fmt.Errorf("presence: upsert presence entry: %w", err)
		}
	}

	return n.query(ctx, requesterID, requesterSettings, jittered, now)
}

// ApplyHeartbeat processes a client "heartbeat" frame, which per
// backend-go.md §4 carries no coordinates: it renews the TTL of the
// requester's existing (already-jittered) stored position and returns a
// freshly filtered nearby list computed from that position. If the
// requester has no live entry (never sent "update" yet, or their previous
// entry already expired/was removed), it still validates consent and
// returns an empty result rather than erroring — the client SDK is
// expected to send "update" first.
func (n *NearbyService) ApplyHeartbeat(ctx context.Context, requesterID string, ttl time.Duration) ([]NearbyResult, error) {
	requesterSettings, err := n.settingsOrDefault(ctx, requesterID)
	if err != nil {
		return nil, err
	}
	now := n.clock.Now()
	if err := checkConsent(requesterSettings, now); err != nil {
		return nil, err
	}

	pos, ok, err := n.geo.Touch(ctx, requesterID, ttl)
	if err != nil {
		return nil, fmt.Errorf("presence: touch presence entry: %w", err)
	}
	if !ok {
		return nil, nil
	}
	return n.query(ctx, requesterID, requesterSettings, pos, now)
}

// SetVisibility handles the WS "visibility" frame. Per backend-go.md §4's
// protocol, mode is one of "visible"/"invisible"/"friends_only". Only
// "invisible" has an immediate index side effect here — matching
// security.md §1.4's fast, session-local "pause" control; "visible" and
// "friends_only" are governed by the durable presence_visibility setting
// (via the REST /v1/presence/settings endpoint, SettingsService.Update),
// so this frame accepts them for protocol compliance without erroring but
// takes no index action for them — a documented scope simplification (see
// README's "Desvios da spec").
func (n *NearbyService) SetVisibility(ctx context.Context, userID, mode string) error {
	if mode == VisibilityInvisible {
		return n.geo.Remove(ctx, userID)
	}
	return nil
}

// Disconnect removes userID from the live index on WS close — presence is
// tied to an active connection's heartbeats; there is no reason to keep a
// disconnected user's last-known position discoverable for the remainder
// of its TTL.
func (n *NearbyService) Disconnect(ctx context.Context, userID string) error {
	return n.geo.Remove(ctx, userID)
}

// candidate is the internal, mid-pipeline representation of one prospect
// that survived the cheap filters (block, target consent/pause/visibility,
// mutual radius, rate limit) and is worth a Detail() hydration + Profile
// lookup.
type candidate struct {
	userID string
	dist   float64
	mutual bool
	target PrivacySettings
}

// query runs the full security.md §1 filter pipeline for requesterID,
// standing at position myPos (already jittered), against the geo index —
// shared by ApplyUpdate and ApplyHeartbeat.
func (n *NearbyService) query(ctx context.Context, requesterID string, requesterSettings PrivacySettings, myPos GeoPosition, now time.Time) ([]NearbyResult, error) {
	searchRadius := float64(requesterSettings.VisibilityRadiusM) + searchPadding
	raw, err := n.geo.Search(ctx, myPos, searchRadius, requesterID)
	if err != nil {
		return nil, fmt.Errorf("presence: search: %w", err)
	}
	if len(raw) == 0 {
		return nil, nil
	}

	// security.md §1.2/§1.8: daily budget is checked once per query, not
	// once per candidate — it caps how many *queries* a user issues per
	// day, and one heartbeat/update is one query regardless of how many
	// candidates it happens to return.
	dailyAllowed, _, err := n.dailyLimiter.Allow(ctx, dailyKey(requesterID), DailyQueryLimit, DailyQueryWindow)
	if err != nil {
		return nil, fmt.Errorf("presence: daily rate limit: %w", err)
	}
	if !dailyAllowed {
		return nil, nil
	}

	survivors := make([]candidate, 0, len(raw))
	for _, c := range raw {
		if c.UserID == requesterID {
			// coverage:ignore — every GeoIndex.Search implementation this
			// package ships (fakeGeoIndex in tests, redisstore.Store in
			// production) already excludes excludeUserID from its own
			// results per the interface's documented contract, so this
			// defensive check can't be exercised through any real
			// implementation without violating that contract first. Kept as
			// a second layer of defense (a future GeoIndex implementation
			// that forgot the exclusion would still be safe here), not
			// because it's expected to fire in practice.
			continue
		}

		blocked, err := n.blocks.IsBlockedEitherWay(ctx, requesterID, c.UserID)
		if err != nil {
			return nil, fmt.Errorf("presence: block check: %w", err)
		}
		if blocked {
			continue
		}

		targetSettings, err := n.settingsOrDefault(ctx, c.UserID)
		if err != nil {
			return nil, err
		}
		if targetSettings.Paused || targetSettings.PresenceVisibility == VisibilityInvisible {
			continue
		}
		if !targetSettings.HasActiveConsent(now) {
			continue
		}

		mutual, err := n.follows.IsMutualFollow(ctx, requesterID, c.UserID)
		if err != nil {
			return nil, fmt.Errorf("presence: follow check: %w", err)
		}
		if targetSettings.PresenceVisibility == VisibilityFriendsOnly && !mutual {
			continue
		}

		// security.md §1.3: mutual radius is an INTERSECTION, never a
		// union — visible only if within BOTH the requester's and the
		// target's configured radius.
		dist := DistanceMeters(myPos, c.Position)
		maxRadius := math.Min(float64(requesterSettings.VisibilityRadiusM), float64(targetSettings.VisibilityRadiusM))
		if dist > maxRadius {
			continue
		}

		allowed, _, err := n.pairLimiter.Allow(ctx, pairKey(requesterID, c.UserID), PairQueryLimit, PairQueryWindow)
		if err != nil {
			return nil, fmt.Errorf("presence: pair rate limit: %w", err)
		}
		if !allowed {
			continue
		}

		survivors = append(survivors, candidate{userID: c.UserID, dist: dist, mutual: mutual, target: targetSettings})
	}
	if len(survivors) == 0 {
		return nil, nil
	}

	ids := make([]string, len(survivors))
	for i, c := range survivors {
		ids[i] = c.userID
	}
	details, err := n.geo.Detail(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("presence: detail: %w", err)
	}

	// Identity hydration only happens for the final, fully-filtered set —
	// data-architecture.md §4.3's minimization principle — and only for
	// those that actually need it (reveal level >= 1).
	needProfile := make([]string, 0, len(survivors))
	for _, c := range survivors {
		if c.mutual || c.target.RevealLevel == RevealLevelOpenDiscovery {
			needProfile = append(needProfile, c.userID)
		}
	}
	var profiles map[string]Profile
	if len(needProfile) > 0 {
		profiles, err = n.profiles.Resolve(ctx, needProfile)
		if err != nil {
			return nil, fmt.Errorf("presence: resolve profiles: %w", err)
		}
	}

	results := make([]NearbyResult, 0, len(survivors))
	for _, c := range survivors {
		detail, found := details[c.userID]
		if !found {
			continue // expired between Search and Detail — treat as gone, not an error
		}

		bucket := BucketFor(c.dist)
		if bucket == BucketNone {
			// coverage:ignore — requires c.dist to land in the razor-thin
			// floating-point gap at exactly the 15000m ceiling (dist <=
			// maxRadius from the gate above, maxRadius's own ceiling is
			// 15000, and BucketFor(15000) is the one value in [0,15000]
			// that maps to BucketNone) — not reproducible deterministically
			// through real haversine distance math without depending on
			// exact float64 rounding. BucketFor's own boundary behavior,
			// including exactly this value, IS directly unit-tested
			// (TestBucketFor_Boundaries in bucket_test.go). Kept as a
			// second layer of defense against ever leaking BucketNone to a
			// client, not because it's expected to fire in practice.
			continue
		}

		res := NearbyResult{UserID: c.userID, Bucket: bucket}
		if c.mutual || c.target.RevealLevel == RevealLevelOpenDiscovery {
			if p, ok := profiles[c.userID]; ok {
				res.DisplayName = p.DisplayName
				res.AvatarURL = p.AvatarURL
			}
		}
		if detail.ShareTrack {
			res.TrackID = detail.TrackID
		}

		if err := n.audit.Append(ctx, AuditLogEntry{
			ID: n.ids.NewID(), RequesterID: requesterID, TargetID: c.userID,
			OccurredAt: now, Bucket: bucket, Endpoint: "ws:/v1/presence/connect",
		}); err != nil {
			return nil, fmt.Errorf("presence: append audit log: %w", err)
		}

		results = append(results, res)
	}
	return results, nil
}
