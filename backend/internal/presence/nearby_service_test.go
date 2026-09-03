package presence

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"smusic/backend/internal/platform/clock"
	"smusic/backend/internal/platform/idgen"
)

type nearbyDeps struct {
	settings *fakePrivacySettingsRepo
	blocks   *fakeBlockRepo
	follows  *fakeFollowChecker
	geo      *fakeGeoIndex
	audit    *fakeAuditLogRepo
	profiles *fakeProfileResolver
	pairRL   *fakeRateLimiter
	dailyRL  *fakeRateLimiter
	clock    *clock.Frozen
}

func newTestNearbyService(t *testing.T, jitter Jitterer) (*NearbyService, *nearbyDeps) {
	t.Helper()
	d := &nearbyDeps{
		settings: newFakePrivacySettingsRepo(),
		blocks:   newFakeBlockRepo(),
		follows:  newFakeFollowChecker(),
		geo:      newFakeGeoIndex(),
		audit:    newFakeAuditLogRepo(),
		profiles: newFakeProfileResolver(),
		pairRL:   newFakeRateLimiter(),
		dailyRL:  newFakeRateLimiter(),
		clock:    clock.NewFrozen(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)),
	}
	svc := NewNearbyService(d.settings, d.blocks, d.follows, d.geo, d.audit, d.profiles, d.pairRL, d.dailyRL, jitter, d.clock, idgen.NewSequential("audit"))
	return svc, d
}

func consentedSettings(userID string, clk clock.Clock, opts ...func(*PrivacySettings)) PrivacySettings {
	now := clk.Now()
	due := now.Add(ConsentValidityPeriod)
	s := PrivacySettings{
		UserID:                   userID,
		PresenceVisibility:       VisibilityEveryone,
		VisibilityRadiusM:        DefaultRadiusM,
		RevealLevel:              RevealLevelAnonymous,
		Paused:                   false,
		ProximityConsentEnabled:  true,
		ProximityConsentTS:       &now,
		ProximityConsentRenewDue: &due,
	}
	for _, o := range opts {
		o(&s)
	}
	return s
}

const originLat, originLon = -23.5505, -46.6333

var origin = GeoPosition{Lat: originLat, Lon: originLon}

// --- consent gating (invariant c: no consent -> can't connect/query) -------

func TestApplyUpdate_NoConsent_Rejected(t *testing.T) {
	svc, _ := newTestNearbyService(t, FixedJitterer{})
	_, err := svc.ApplyUpdate(context.Background(), "a", originLat, originLon, "", time.Minute)
	assert.ErrorIs(t, err, ErrConsentRequired)
}

func TestApplyUpdate_ExpiredConsent_Rejected(t *testing.T) {
	svc, d := newTestNearbyService(t, FixedJitterer{})
	past := d.clock.Now().Add(-time.Hour)
	d.settings.set(PrivacySettings{UserID: "a", ProximityConsentEnabled: true, ProximityConsentRenewDue: &past})
	_, err := svc.ApplyUpdate(context.Background(), "a", originLat, originLon, "", time.Minute)
	assert.ErrorIs(t, err, ErrConsentExpired)
}

func TestCheckConsent(t *testing.T) {
	svc, d := newTestNearbyService(t, FixedJitterer{})
	assert.ErrorIs(t, svc.CheckConsent(context.Background(), "a"), ErrConsentRequired)

	d.settings.set(consentedSettings("a", d.clock))
	assert.NoError(t, svc.CheckConsent(context.Background(), "a"))
}

func TestApplyHeartbeat_NoConsent_Rejected(t *testing.T) {
	svc, _ := newTestNearbyService(t, FixedJitterer{})
	_, err := svc.ApplyHeartbeat(context.Background(), "a", time.Minute)
	assert.ErrorIs(t, err, ErrConsentRequired)
}

var errBoomNearby = assertErrNearby("boom")

type assertErrNearby string

func (e assertErrNearby) Error() string { return string(e) }

func TestApplyUpdate_SettingsRepoError_NotWrappedAsSentinel(t *testing.T) {
	svc, d := newTestNearbyService(t, FixedJitterer{})
	d.settings.getErr = errBoomNearby
	_, err := svc.ApplyUpdate(context.Background(), "a", originLat, originLon, "", time.Minute)
	require.Error(t, err)
	assert.False(t, errors.Is(err, ErrConsentRequired))
}

func TestCheckConsent_SettingsRepoError(t *testing.T) {
	svc, d := newTestNearbyService(t, FixedJitterer{})
	d.settings.getErr = errBoomNearby
	err := svc.CheckConsent(context.Background(), "a")
	require.Error(t, err)
	assert.False(t, errors.Is(err, ErrConsentRequired))
}

func TestApplyUpdate_GeoUpsertError(t *testing.T) {
	svc, d := newTestNearbyService(t, FixedJitterer{})
	d.settings.set(consentedSettings("a", d.clock))
	d.geo.upsertErr = errBoomNearby
	_, err := svc.ApplyUpdate(context.Background(), "a", originLat, originLon, "", time.Minute)
	require.Error(t, err)
}

func TestApplyHeartbeat_SettingsRepoError(t *testing.T) {
	svc, d := newTestNearbyService(t, FixedJitterer{})
	d.settings.getErr = errBoomNearby
	_, err := svc.ApplyHeartbeat(context.Background(), "a", time.Minute)
	require.Error(t, err)
}

func TestApplyHeartbeat_GeoTouchError(t *testing.T) {
	svc, d := newTestNearbyService(t, FixedJitterer{})
	d.settings.set(consentedSettings("a", d.clock))
	d.geo.touchErr = errBoomNearby
	_, err := svc.ApplyHeartbeat(context.Background(), "a", time.Minute)
	require.Error(t, err)
}

func TestQuery_SearchError(t *testing.T) {
	svc, d := newTestNearbyService(t, FixedJitterer{})
	d.settings.set(consentedSettings("a", d.clock))
	d.geo.searchErr = errBoomNearby
	_, err := svc.ApplyUpdate(context.Background(), "a", originLat, originLon, "", time.Minute)
	require.Error(t, err)
}

func TestQuery_DailyLimiterError(t *testing.T) {
	svc, d := newTestNearbyService(t, FixedJitterer{})
	d.settings.set(consentedSettings("a", d.clock))
	d.settings.set(consentedSettings("b", d.clock))
	require.NoError(t, d.geo.Upsert(context.Background(), PresenceEntry{UserID: "b", Position: origin}, time.Minute))
	d.dailyRL.err = errBoomNearby
	_, err := svc.ApplyUpdate(context.Background(), "a", originLat, originLon, "", time.Minute)
	require.Error(t, err)
}

func TestQuery_BlockCheckError(t *testing.T) {
	svc, d := newTestNearbyService(t, FixedJitterer{})
	d.settings.set(consentedSettings("a", d.clock))
	require.NoError(t, d.geo.Upsert(context.Background(), PresenceEntry{UserID: "b", Position: origin}, time.Minute))
	d.blocks.err = errBoomNearby
	_, err := svc.ApplyUpdate(context.Background(), "a", originLat, originLon, "", time.Minute)
	require.Error(t, err)
}

func TestQuery_TargetSettingsError(t *testing.T) {
	svc, d := newTestNearbyService(t, FixedJitterer{})
	d.settings.set(consentedSettings("a", d.clock))
	require.NoError(t, d.geo.Upsert(context.Background(), PresenceEntry{UserID: "b", Position: origin}, time.Minute))
	// a's own settings lookup must succeed (it's fetched first, before the
	// query loop even starts); only the per-candidate lookup for "b" fails.
	d.settings.getErr = errBoomNearby
	d.settings.getErrFor = "b"

	_, err := svc.ApplyUpdate(context.Background(), "a", originLat, originLon, "", time.Minute)
	require.Error(t, err)
	assert.False(t, errors.Is(err, ErrSettingsNotFound))
}

func TestQuery_FollowCheckError(t *testing.T) {
	svc, d := newTestNearbyService(t, FixedJitterer{})
	d.settings.set(consentedSettings("a", d.clock))
	d.settings.set(consentedSettings("b", d.clock))
	require.NoError(t, d.geo.Upsert(context.Background(), PresenceEntry{UserID: "b", Position: origin}, time.Minute))
	d.follows.err = errBoomNearby
	_, err := svc.ApplyUpdate(context.Background(), "a", originLat, originLon, "", time.Minute)
	require.Error(t, err)
}

func TestQuery_PairLimiterError(t *testing.T) {
	svc, d := newTestNearbyService(t, FixedJitterer{})
	d.settings.set(consentedSettings("a", d.clock))
	d.settings.set(consentedSettings("b", d.clock))
	require.NoError(t, d.geo.Upsert(context.Background(), PresenceEntry{UserID: "b", Position: origin}, time.Minute))
	d.pairRL.err = errBoomNearby
	_, err := svc.ApplyUpdate(context.Background(), "a", originLat, originLon, "", time.Minute)
	require.Error(t, err)
}

func TestQuery_DetailError(t *testing.T) {
	svc, d := newTestNearbyService(t, FixedJitterer{})
	d.settings.set(consentedSettings("a", d.clock))
	d.settings.set(consentedSettings("b", d.clock))
	require.NoError(t, d.geo.Upsert(context.Background(), PresenceEntry{UserID: "b", Position: origin}, time.Minute))
	d.geo.detailErr = errBoomNearby
	_, err := svc.ApplyUpdate(context.Background(), "a", originLat, originLon, "", time.Minute)
	require.Error(t, err)
}

func TestQuery_DetailMissing_TreatedAsExpired_NotError(t *testing.T) {
	svc, d := newTestNearbyService(t, FixedJitterer{})
	d.settings.set(consentedSettings("a", d.clock))
	d.settings.set(consentedSettings("b", d.clock))
	require.NoError(t, d.geo.Upsert(context.Background(), PresenceEntry{UserID: "b", Position: origin}, time.Minute))
	// Simulate b's entry expiring (TTL race) in the window between Search
	// and Detail: Detail silently omits it rather than erroring, and the
	// candidate must simply be dropped (never surfaced, never an error).
	d.geo.omitDetail = map[string]bool{"b": true}

	results, err := svc.ApplyUpdate(context.Background(), "a", originLat, originLon, "", time.Minute)
	require.NoError(t, err)
	assert.Empty(t, results)
	assert.Empty(t, d.audit.all(), "a candidate that expired before Detail must not be audit-logged as an access")
}

func TestQuery_ProfileResolveError(t *testing.T) {
	svc, d := newTestNearbyService(t, FixedJitterer{})
	d.settings.set(consentedSettings("a", d.clock))
	d.settings.set(consentedSettings("b", d.clock, func(s *PrivacySettings) { s.RevealLevel = RevealLevelOpenDiscovery }))
	require.NoError(t, d.geo.Upsert(context.Background(), PresenceEntry{UserID: "b", Position: origin}, time.Minute))
	d.profiles.err = errBoomNearby
	_, err := svc.ApplyUpdate(context.Background(), "a", originLat, originLon, "", time.Minute)
	require.Error(t, err)
}

func TestQuery_AuditAppendError(t *testing.T) {
	svc, d := newTestNearbyService(t, FixedJitterer{})
	d.settings.set(consentedSettings("a", d.clock))
	d.settings.set(consentedSettings("b", d.clock))
	require.NoError(t, d.geo.Upsert(context.Background(), PresenceEntry{UserID: "b", Position: origin}, time.Minute))
	d.audit.err = errBoomNearby
	_, err := svc.ApplyUpdate(context.Background(), "a", originLat, originLon, "", time.Minute)
	require.Error(t, err)
}

// --- jitter is actually applied and never the raw coordinate ---------------

func TestApplyUpdate_StoresJitteredNotRawPosition(t *testing.T) {
	svc, d := newTestNearbyService(t, FixedJitterer{NorthM: 75, EastM: 0})
	d.settings.set(consentedSettings("a", d.clock))

	_, err := svc.ApplyUpdate(context.Background(), "a", originLat, originLon, "", time.Minute)
	require.NoError(t, err)

	stored := d.geo.entries["a"].pos
	assert.NotEqual(t, origin, stored, "the raw position must never be the position stored in the index")
	assert.InDelta(t, 75, DistanceMeters(origin, stored), 0.5)
}

func TestApplyUpdate_Paused_NotIndexed_ButStillQueries(t *testing.T) {
	svc, d := newTestNearbyService(t, FixedJitterer{})
	d.settings.set(consentedSettings("a", d.clock, func(s *PrivacySettings) { s.Paused = true }))
	require.NoError(t, d.geo.Upsert(context.Background(), PresenceEntry{UserID: "b", Position: origin}, time.Minute))
	d.settings.set(consentedSettings("b", d.clock))

	results, err := svc.ApplyUpdate(context.Background(), "a", originLat, originLon, "", time.Minute)
	require.NoError(t, err)

	_, ok := d.geo.entries["a"]
	assert.False(t, ok, "a paused user must not be inserted into the index")
	require.Len(t, results, 1, "a paused user can still see others near them")
	assert.Equal(t, "b", results[0].UserID)
}

// --- invariant (f): mutual radius is an intersection, never a union -------

func TestQuery_RadiusIsIntersectionNotUnion(t *testing.T) {
	svc, d := newTestNearbyService(t, FixedJitterer{})

	// distance = 500m. requester's radius (150m) alone would exclude; target's
	// radius (15000m) alone would include. A union would show it; the
	// required intersection semantics must exclude it.
	d.settings.set(consentedSettings("a", d.clock, func(s *PrivacySettings) { s.VisibilityRadiusM = 150 }))
	d.settings.set(consentedSettings("b", d.clock, func(s *PrivacySettings) { s.VisibilityRadiusM = 15000 }))
	bPos := offsetMeters(origin, 500, 0)
	require.NoError(t, d.geo.Upsert(context.Background(), PresenceEntry{UserID: "b", Position: bPos}, time.Minute))

	results, err := svc.ApplyUpdate(context.Background(), "a", originLat, originLon, "", time.Minute)
	require.NoError(t, err)
	assert.Empty(t, results, "must be excluded: requester's own 150m radius doesn't reach a 500m target even though the target's radius would")
}

func TestQuery_RadiusIntersection_ReverseDirectionAlsoExcludes(t *testing.T) {
	svc, d := newTestNearbyService(t, FixedJitterer{})

	// Same 500m distance, radii swapped: requester's radius (15000m) alone
	// would include; target's own radius (150m) alone would exclude.
	d.settings.set(consentedSettings("a", d.clock, func(s *PrivacySettings) { s.VisibilityRadiusM = 15000 }))
	d.settings.set(consentedSettings("b", d.clock, func(s *PrivacySettings) { s.VisibilityRadiusM = 150 }))
	bPos := offsetMeters(origin, 500, 0)
	require.NoError(t, d.geo.Upsert(context.Background(), PresenceEntry{UserID: "b", Position: bPos}, time.Minute))

	results, err := svc.ApplyUpdate(context.Background(), "a", originLat, originLon, "", time.Minute)
	require.NoError(t, err)
	assert.Empty(t, results, "must be excluded: the TARGET's own radius must also be respected, not just the requester's")
}

func TestQuery_RadiusIntersection_IncludedWhenBothSatisfy(t *testing.T) {
	svc, d := newTestNearbyService(t, FixedJitterer{})
	d.settings.set(consentedSettings("a", d.clock, func(s *PrivacySettings) { s.VisibilityRadiusM = 1000 }))
	d.settings.set(consentedSettings("b", d.clock, func(s *PrivacySettings) { s.VisibilityRadiusM = 1000 }))
	bPos := offsetMeters(origin, 500, 0)
	require.NoError(t, d.geo.Upsert(context.Background(), PresenceEntry{UserID: "b", Position: bPos}, time.Minute))

	results, err := svc.ApplyUpdate(context.Background(), "a", originLat, originLon, "", time.Minute)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, Bucket2, results[0].Bucket) // 500m falls in [150,1000)
}

// --- invariant (d): blocking is respected in both directions ---------------

func TestQuery_BlockedByRequester_Excluded(t *testing.T) {
	svc, d := newTestNearbyService(t, FixedJitterer{})
	d.settings.set(consentedSettings("a", d.clock))
	d.settings.set(consentedSettings("b", d.clock))
	require.NoError(t, d.geo.Upsert(context.Background(), PresenceEntry{UserID: "b", Position: origin}, time.Minute))
	require.NoError(t, d.blocks.Block(context.Background(), "a", "b"))

	results, err := svc.ApplyUpdate(context.Background(), "a", originLat, originLon, "", time.Minute)
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestQuery_BlockedByTarget_ExcludedFromRequesterView(t *testing.T) {
	svc, d := newTestNearbyService(t, FixedJitterer{})
	d.settings.set(consentedSettings("a", d.clock))
	d.settings.set(consentedSettings("b", d.clock))
	require.NoError(t, d.geo.Upsert(context.Background(), PresenceEntry{UserID: "b", Position: origin}, time.Minute))
	// b blocked a (reverse direction) -- a must still not see b.
	require.NoError(t, d.blocks.Block(context.Background(), "b", "a"))

	results, err := svc.ApplyUpdate(context.Background(), "a", originLat, originLon, "", time.Minute)
	require.NoError(t, err)
	assert.Empty(t, results, "blocking must be symmetric in effect regardless of who blocked whom")
}

// --- consent/pause/visibility of the TARGET also gate visibility -----------

func TestQuery_TargetWithoutConsent_Excluded(t *testing.T) {
	svc, d := newTestNearbyService(t, FixedJitterer{})
	d.settings.set(consentedSettings("a", d.clock))
	// b has no settings row at all -> default, no consent.
	require.NoError(t, d.geo.Upsert(context.Background(), PresenceEntry{UserID: "b", Position: origin}, time.Minute))

	results, err := svc.ApplyUpdate(context.Background(), "a", originLat, originLon, "", time.Minute)
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestQuery_TargetConsentExpired_Excluded(t *testing.T) {
	svc, d := newTestNearbyService(t, FixedJitterer{})
	d.settings.set(consentedSettings("a", d.clock))
	past := d.clock.Now().Add(-time.Hour)
	// b has consent enabled but its renewal is overdue -- HasActiveConsent
	// must gate the TARGET too, not just the requester (CheckConsent/
	// ApplyUpdate already gate the requester; this is the per-candidate path
	// inside query()).
	d.settings.set(PrivacySettings{
		UserID: "b", PresenceVisibility: VisibilityEveryone, VisibilityRadiusM: DefaultRadiusM,
		ProximityConsentEnabled: true, ProximityConsentRenewDue: &past,
	})
	require.NoError(t, d.geo.Upsert(context.Background(), PresenceEntry{UserID: "b", Position: origin}, time.Minute))

	results, err := svc.ApplyUpdate(context.Background(), "a", originLat, originLon, "", time.Minute)
	require.NoError(t, err)
	assert.Empty(t, results, "a target whose consent has expired must not be shown, even if still in the live index")
}

func TestQuery_TargetPaused_Excluded(t *testing.T) {
	svc, d := newTestNearbyService(t, FixedJitterer{})
	d.settings.set(consentedSettings("a", d.clock))
	d.settings.set(consentedSettings("b", d.clock, func(s *PrivacySettings) { s.Paused = true }))
	require.NoError(t, d.geo.Upsert(context.Background(), PresenceEntry{UserID: "b", Position: origin}, time.Minute))

	results, err := svc.ApplyUpdate(context.Background(), "a", originLat, originLon, "", time.Minute)
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestQuery_TargetInvisible_Excluded(t *testing.T) {
	svc, d := newTestNearbyService(t, FixedJitterer{})
	d.settings.set(consentedSettings("a", d.clock))
	d.settings.set(consentedSettings("b", d.clock, func(s *PrivacySettings) { s.PresenceVisibility = VisibilityInvisible }))
	require.NoError(t, d.geo.Upsert(context.Background(), PresenceEntry{UserID: "b", Position: origin}, time.Minute))

	results, err := svc.ApplyUpdate(context.Background(), "a", originLat, originLon, "", time.Minute)
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestQuery_FriendsOnly_ExcludedWithoutMutualFollow(t *testing.T) {
	svc, d := newTestNearbyService(t, FixedJitterer{})
	d.settings.set(consentedSettings("a", d.clock))
	d.settings.set(consentedSettings("b", d.clock, func(s *PrivacySettings) { s.PresenceVisibility = VisibilityFriendsOnly }))
	require.NoError(t, d.geo.Upsert(context.Background(), PresenceEntry{UserID: "b", Position: origin}, time.Minute))

	results, err := svc.ApplyUpdate(context.Background(), "a", originLat, originLon, "", time.Minute)
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestQuery_FriendsOnly_IncludedWithMutualFollow(t *testing.T) {
	svc, d := newTestNearbyService(t, FixedJitterer{})
	d.settings.set(consentedSettings("a", d.clock))
	d.settings.set(consentedSettings("b", d.clock, func(s *PrivacySettings) { s.PresenceVisibility = VisibilityFriendsOnly }))
	require.NoError(t, d.geo.Upsert(context.Background(), PresenceEntry{UserID: "b", Position: origin}, time.Minute))
	d.follows.setFollows("a", "b")
	d.follows.setFollows("b", "a")

	results, err := svc.ApplyUpdate(context.Background(), "a", originLat, originLon, "", time.Minute)
	require.NoError(t, err)
	require.Len(t, results, 1)
}

func TestQuery_FriendsOnly_OneSidedFollow_NotMutual_Excluded(t *testing.T) {
	svc, d := newTestNearbyService(t, FixedJitterer{})
	d.settings.set(consentedSettings("a", d.clock))
	d.settings.set(consentedSettings("b", d.clock, func(s *PrivacySettings) { s.PresenceVisibility = VisibilityFriendsOnly }))
	require.NoError(t, d.geo.Upsert(context.Background(), PresenceEntry{UserID: "b", Position: origin}, time.Minute))
	d.follows.setFollows("a", "b") // only one direction

	results, err := svc.ApplyUpdate(context.Background(), "a", originLat, originLon, "", time.Minute)
	require.NoError(t, err)
	assert.Empty(t, results)
}

// --- reveal levels (security.md §1.6) --------------------------------------

func TestQuery_RevealLevel0_Default_Anonymous(t *testing.T) {
	svc, d := newTestNearbyService(t, FixedJitterer{})
	d.settings.set(consentedSettings("a", d.clock))
	d.settings.set(consentedSettings("b", d.clock)) // RevealLevelAnonymous by default
	require.NoError(t, d.geo.Upsert(context.Background(), PresenceEntry{UserID: "b", Position: origin}, time.Minute))
	d.profiles.profiles["b"] = Profile{DisplayName: "Bob", AvatarURL: "http://x/bob.png"}

	results, err := svc.ApplyUpdate(context.Background(), "a", originLat, originLon, "", time.Minute)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Empty(t, results[0].DisplayName)
	assert.Empty(t, results[0].AvatarURL)
}

func TestQuery_MutualConnection_AlwaysRevealsIdentity(t *testing.T) {
	svc, d := newTestNearbyService(t, FixedJitterer{})
	d.settings.set(consentedSettings("a", d.clock))
	d.settings.set(consentedSettings("b", d.clock)) // reveal level 0, but mutual follow overrides
	require.NoError(t, d.geo.Upsert(context.Background(), PresenceEntry{UserID: "b", Position: origin}, time.Minute))
	d.profiles.profiles["b"] = Profile{DisplayName: "Bob", AvatarURL: "http://x/bob.png"}
	d.follows.setFollows("a", "b")
	d.follows.setFollows("b", "a")

	results, err := svc.ApplyUpdate(context.Background(), "a", originLat, originLon, "", time.Minute)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "Bob", results[0].DisplayName)
	assert.Equal(t, "http://x/bob.png", results[0].AvatarURL)
}

func TestQuery_RevealLevel2_OpenDiscovery_RevealsToNonConnections(t *testing.T) {
	svc, d := newTestNearbyService(t, FixedJitterer{})
	d.settings.set(consentedSettings("a", d.clock))
	d.settings.set(consentedSettings("b", d.clock, func(s *PrivacySettings) { s.RevealLevel = RevealLevelOpenDiscovery }))
	require.NoError(t, d.geo.Upsert(context.Background(), PresenceEntry{UserID: "b", Position: origin}, time.Minute))
	d.profiles.profiles["b"] = Profile{DisplayName: "Bob"}

	results, err := svc.ApplyUpdate(context.Background(), "a", originLat, originLon, "", time.Minute)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "Bob", results[0].DisplayName)
}

func TestQuery_RevealLevel1_WithoutConnection_StaysAnonymous(t *testing.T) {
	svc, d := newTestNearbyService(t, FixedJitterer{})
	d.settings.set(consentedSettings("a", d.clock))
	d.settings.set(consentedSettings("b", d.clock, func(s *PrivacySettings) { s.RevealLevel = RevealLevelConnections }))
	require.NoError(t, d.geo.Upsert(context.Background(), PresenceEntry{UserID: "b", Position: origin}, time.Minute))
	d.profiles.profiles["b"] = Profile{DisplayName: "Bob"}

	results, err := svc.ApplyUpdate(context.Background(), "a", originLat, originLon, "", time.Minute)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Empty(t, results[0].DisplayName, "reveal level 1 alone (without a connection) must not reveal identity")
}

// --- track sharing -----------------------------------------------------

func TestQuery_ShareTrack_PropagatesTrackID(t *testing.T) {
	svc, d := newTestNearbyService(t, FixedJitterer{})
	d.settings.set(consentedSettings("a", d.clock))
	d.settings.set(consentedSettings("b", d.clock))
	require.NoError(t, d.geo.Upsert(context.Background(), PresenceEntry{UserID: "b", Position: origin, TrackID: "track-1", ShareTrack: true}, time.Minute))

	results, err := svc.ApplyUpdate(context.Background(), "a", originLat, originLon, "", time.Minute)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "track-1", results[0].TrackID)
}

func TestQuery_ShareTrackFalse_HidesTrackID(t *testing.T) {
	svc, d := newTestNearbyService(t, FixedJitterer{})
	d.settings.set(consentedSettings("a", d.clock))
	d.settings.set(consentedSettings("b", d.clock))
	require.NoError(t, d.geo.Upsert(context.Background(), PresenceEntry{UserID: "b", Position: origin, TrackID: "track-1", ShareTrack: false}, time.Minute))

	results, err := svc.ApplyUpdate(context.Background(), "a", originLat, originLon, "", time.Minute)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Empty(t, results[0].TrackID)
}

// --- invariant (e): rate limiting is applied --------------------------------

func TestQuery_PairRateLimitExceeded_ExcludesTarget(t *testing.T) {
	svc, d := newTestNearbyService(t, FixedJitterer{})
	d.settings.set(consentedSettings("a", d.clock))
	d.settings.set(consentedSettings("b", d.clock))
	require.NoError(t, d.geo.Upsert(context.Background(), PresenceEntry{UserID: "b", Position: origin}, time.Minute))
	d.pairRL.denyKeys[pairKey("a", "b")] = true

	results, err := svc.ApplyUpdate(context.Background(), "a", originLat, originLon, "", time.Minute)
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestQuery_PairRateLimit_KeyIsDirectional(t *testing.T) {
	svc, d := newTestNearbyService(t, FixedJitterer{})
	d.settings.set(consentedSettings("a", d.clock))
	d.settings.set(consentedSettings("b", d.clock))
	require.NoError(t, d.geo.Upsert(context.Background(), PresenceEntry{UserID: "b", Position: origin}, time.Minute))
	// Deny the reverse direction only (b querying about a) -- must not
	// affect a querying about b.
	d.pairRL.denyKeys[pairKey("b", "a")] = true

	results, err := svc.ApplyUpdate(context.Background(), "a", originLat, originLon, "", time.Minute)
	require.NoError(t, err)
	assert.Len(t, results, 1)
}

func TestQuery_DailyRateLimitExhausted_ReturnsNoResultsNoError(t *testing.T) {
	svc, d := newTestNearbyService(t, FixedJitterer{})
	d.settings.set(consentedSettings("a", d.clock))
	d.settings.set(consentedSettings("b", d.clock))
	require.NoError(t, d.geo.Upsert(context.Background(), PresenceEntry{UserID: "b", Position: origin}, time.Minute))
	d.dailyRL.denyKeys[dailyKey("a")] = true

	results, err := svc.ApplyUpdate(context.Background(), "a", originLat, originLon, "", time.Minute)
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestQuery_DailyRateLimitCheckedOncePerQuery_NotPerCandidate(t *testing.T) {
	svc, d := newTestNearbyService(t, FixedJitterer{})
	d.settings.set(consentedSettings("a", d.clock))
	for _, id := range []string{"b", "c", "d"} {
		d.settings.set(consentedSettings(id, d.clock))
		require.NoError(t, d.geo.Upsert(context.Background(), PresenceEntry{UserID: id, Position: origin}, time.Minute))
	}

	_, err := svc.ApplyUpdate(context.Background(), "a", originLat, originLon, "", time.Minute)
	require.NoError(t, err)

	count := 0
	for _, k := range d.dailyRL.calls {
		if k == dailyKey("a") {
			count++
		}
	}
	assert.Equal(t, 1, count, "the daily budget must be spent once per query event, not once per candidate returned")
}

// --- audit log (security.md §1.8) -------------------------------------------

func TestQuery_WritesAuditLogEntryPerIncludedResult(t *testing.T) {
	svc, d := newTestNearbyService(t, FixedJitterer{})
	d.settings.set(consentedSettings("a", d.clock))
	d.settings.set(consentedSettings("b", d.clock))
	require.NoError(t, d.geo.Upsert(context.Background(), PresenceEntry{UserID: "b", Position: origin}, time.Minute))

	results, err := svc.ApplyUpdate(context.Background(), "a", originLat, originLon, "", time.Minute)
	require.NoError(t, err)
	require.Len(t, results, 1)

	entries := d.audit.all()
	require.Len(t, entries, 1)
	assert.Equal(t, "a", entries[0].RequesterID)
	assert.Equal(t, "b", entries[0].TargetID)
	assert.Equal(t, results[0].Bucket, entries[0].Bucket)
	assert.Equal(t, d.clock.Now(), entries[0].OccurredAt)
	assert.NotEmpty(t, entries[0].Endpoint)
}

func TestQuery_NoAuditLogForFilteredOutCandidates(t *testing.T) {
	svc, d := newTestNearbyService(t, FixedJitterer{})
	d.settings.set(consentedSettings("a", d.clock))
	d.settings.set(consentedSettings("b", d.clock))
	require.NoError(t, d.geo.Upsert(context.Background(), PresenceEntry{UserID: "b", Position: origin}, time.Minute))
	require.NoError(t, d.blocks.Block(context.Background(), "a", "b"))

	_, err := svc.ApplyUpdate(context.Background(), "a", originLat, originLon, "", time.Minute)
	require.NoError(t, err)
	assert.Empty(t, d.audit.all())
}

// --- heartbeat path ----------------------------------------------------

func TestApplyHeartbeat_NoExistingEntry_ReturnsNilNoError(t *testing.T) {
	svc, d := newTestNearbyService(t, FixedJitterer{})
	d.settings.set(consentedSettings("a", d.clock))
	results, err := svc.ApplyHeartbeat(context.Background(), "a", time.Minute)
	require.NoError(t, err)
	assert.Nil(t, results)
}

func TestApplyHeartbeat_RenewsAndQueries(t *testing.T) {
	svc, d := newTestNearbyService(t, FixedJitterer{})
	d.settings.set(consentedSettings("a", d.clock))
	d.settings.set(consentedSettings("b", d.clock))
	require.NoError(t, d.geo.Upsert(context.Background(), PresenceEntry{UserID: "a", Position: origin}, time.Minute))
	require.NoError(t, d.geo.Upsert(context.Background(), PresenceEntry{UserID: "b", Position: origin}, time.Minute))

	results, err := svc.ApplyHeartbeat(context.Background(), "a", time.Minute)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "b", results[0].UserID)
}

// --- visibility frame / disconnect --------------------------------------

func TestSetVisibility_Invisible_RemovesFromIndex(t *testing.T) {
	svc, d := newTestNearbyService(t, FixedJitterer{})
	require.NoError(t, d.geo.Upsert(context.Background(), PresenceEntry{UserID: "a", Position: origin}, time.Minute))

	require.NoError(t, svc.SetVisibility(context.Background(), "a", VisibilityInvisible))
	_, ok, _ := d.geo.Touch(context.Background(), "a", time.Minute)
	assert.False(t, ok)
}

func TestSetVisibility_Visible_NoIndexSideEffect(t *testing.T) {
	svc, d := newTestNearbyService(t, FixedJitterer{})
	require.NoError(t, d.geo.Upsert(context.Background(), PresenceEntry{UserID: "a", Position: origin}, time.Minute))

	require.NoError(t, svc.SetVisibility(context.Background(), "a", VisibilityEveryone))
	_, ok, _ := d.geo.Touch(context.Background(), "a", time.Minute)
	assert.True(t, ok)
}

func TestDisconnect_RemovesFromIndex(t *testing.T) {
	svc, d := newTestNearbyService(t, FixedJitterer{})
	require.NoError(t, d.geo.Upsert(context.Background(), PresenceEntry{UserID: "a", Position: origin}, time.Minute))

	require.NoError(t, svc.Disconnect(context.Background(), "a"))
	_, ok, _ := d.geo.Touch(context.Background(), "a", time.Minute)
	assert.False(t, ok)
}

// --- invariant (a)/(g): buckets never leak coordinates, and presence is
// never modeled as data that COULD be persisted durably --------------------

// TestNearbyResult_StructurallyCannotCarryCoordinates asserts, by
// reflecting over the type (not just by behavior), that the data returned
// to a client has no field that could carry a raw coordinate, exact
// distance, or geohash — security.md §1.2. This is stronger than a
// behavioral test: it holds regardless of what any call site does.
func TestNearbyResult_StructurallyCannotCarryCoordinates(t *testing.T) {
	typ := reflect.TypeOf(NearbyResult{})
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		assert.NotEqualf(t, reflect.Float64, f.Type.Kind(), "field %s must not be a float64 (could carry a coordinate or exact distance)", f.Name)
		lower := strings.ToLower(f.Name)
		assert.NotContains(t, lower, "lat")
		assert.NotContains(t, lower, "lon")
		assert.NotContains(t, lower, "geohash")
		assert.NotContains(t, lower, "distance_m")
	}
}

// TestPrivacySettings_NeverPersistsCoordinates: PrivacySettings is the
// durable (Postgres) record. It must never gain a lat/lon field — durable
// storage of raw location is exactly what security.md §1.5 forbids.
func TestPrivacySettings_NeverPersistsCoordinates(t *testing.T) {
	typ := reflect.TypeOf(PrivacySettings{})
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		lower := strings.ToLower(f.Name)
		assert.NotContains(t, lower, "lat")
		assert.NotContains(t, lower, "lon")
		assert.NotContains(t, lower, "geohash")
		assert.NotContains(t, lower, "position")
	}
}

// TestAuditLogEntry_NeverPersistsCoordinates: the durable audit log
// (security.md §1.8) stores only the bucket, never a coordinate.
func TestAuditLogEntry_NeverPersistsCoordinates(t *testing.T) {
	typ := reflect.TypeOf(AuditLogEntry{})
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		lower := strings.ToLower(f.Name)
		assert.NotContains(t, lower, "lat")
		assert.NotContains(t, lower, "lon")
		assert.NotContains(t, lower, "geohash")
		assert.NotContains(t, lower, "position")
	}
}

// TestQuery_BucketNeverRevealsExactDistance_AcrossManyJitteredQueries
// simulates repeated proximity queries (as an attacker attempting
// triangulation would) with REAL jitter and asserts every single bucket
// returned is one of the four defined buckets — never BucketNone, never
// out of range — even though the underlying jittered distance varies
// between calls. Combined with the rate limiter (tested separately), this
// is the mitigation security.md §1.2 describes.
func TestQuery_BucketNeverRevealsExactDistance_AcrossManyJitteredQueries(t *testing.T) {
	svc, d := newTestNearbyService(t, RandJitterer{})
	d.settings.set(consentedSettings("a", d.clock))
	d.settings.set(consentedSettings("b", d.clock))
	// Place b at a real (never-exposed) distance of 300m from origin.
	bTruePos := offsetMeters(origin, 300, 0)
	require.NoError(t, d.geo.Upsert(context.Background(), PresenceEntry{UserID: "b", Position: bTruePos}, time.Minute))

	seen := map[DistanceBucket]int{}
	for i := 0; i < 200; i++ {
		// Disable the rate limiters for this stress test so every
		// iteration actually reaches the bucket computation (rate limiting
		// is covered by its own dedicated tests above).
		results, err := svc.ApplyUpdate(context.Background(), "a", originLat, originLon, "", time.Minute)
		require.NoError(t, err)
		if len(results) == 1 {
			seen[results[0].Bucket]++
			assert.Contains(t, []DistanceBucket{Bucket1, Bucket2, Bucket3, Bucket4}, results[0].Bucket)
		}
	}
	assert.NotEmpty(t, seen, "expected at least some queries to succeed and return a bucket")
}
