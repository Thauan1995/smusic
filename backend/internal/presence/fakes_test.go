package presence

import (
	"context"
	"sync"
	"time"
)

// --- fakePrivacySettingsRepo -------------------------------------------------

type fakePrivacySettingsRepo struct {
	mu        sync.Mutex
	rows      map[string]PrivacySettings
	getErr    error
	getErrFor string // if set, getErr only applies to this userID; empty means "every Get"
	upsertErr error
}

func newFakePrivacySettingsRepo() *fakePrivacySettingsRepo {
	return &fakePrivacySettingsRepo{rows: map[string]PrivacySettings{}}
}

func (f *fakePrivacySettingsRepo) Get(_ context.Context, userID string) (PrivacySettings, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.getErr != nil && (f.getErrFor == "" || f.getErrFor == userID) {
		return PrivacySettings{}, f.getErr
	}
	s, ok := f.rows[userID]
	if !ok {
		return PrivacySettings{}, ErrSettingsNotFound
	}
	return s, nil
}

func (f *fakePrivacySettingsRepo) Upsert(_ context.Context, s PrivacySettings) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.upsertErr != nil {
		return f.upsertErr
	}
	f.rows[s.UserID] = s
	return nil
}

func (f *fakePrivacySettingsRepo) set(s PrivacySettings) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rows[s.UserID] = s
}

// --- fakeBlockRepo -----------------------------------------------------------

type fakeBlockRepo struct {
	mu      sync.Mutex
	blocked map[[2]string]bool
	err     error
}

func newFakeBlockRepo() *fakeBlockRepo {
	return &fakeBlockRepo{blocked: map[[2]string]bool{}}
}

func (f *fakeBlockRepo) Block(_ context.Context, blockerID, blockedID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.blocked[[2]string{blockerID, blockedID}] = true
	return nil
}

func (f *fakeBlockRepo) Unblock(_ context.Context, blockerID, blockedID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	delete(f.blocked, [2]string{blockerID, blockedID})
	return nil
}

func (f *fakeBlockRepo) IsBlockedEitherWay(_ context.Context, a, b string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return false, f.err
	}
	return f.blocked[[2]string{a, b}] || f.blocked[[2]string{b, a}], nil
}

// --- fakeFollowChecker ---------------------------------------------------

type fakeFollowChecker struct {
	mu      sync.Mutex
	follows map[[2]string]bool // follower -> followee
	err     error
}

func newFakeFollowChecker() *fakeFollowChecker {
	return &fakeFollowChecker{follows: map[[2]string]bool{}}
}

func (f *fakeFollowChecker) setFollows(follower, followee string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.follows[[2]string{follower, followee}] = true
}

func (f *fakeFollowChecker) IsMutualFollow(_ context.Context, a, b string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return false, f.err
	}
	return f.follows[[2]string{a, b}] && f.follows[[2]string{b, a}], nil
}

// --- fakeMFAChecker --------------------------------------------------------

// fakeMFAChecker defaults to "verified" for every user (verified: true) so
// existing tests that exercise GrantConsent without caring about MFA don't
// need to change; tests specifically covering the MFA gate construct one
// with verified: false or a specific per-user map.
type fakeMFAChecker struct {
	mu       sync.Mutex
	verified bool            // default answer when userID isn't in perUser
	perUser  map[string]bool // overrides verified for specific users
	err      error
}

func newFakeMFAChecker(verified bool) *fakeMFAChecker {
	return &fakeMFAChecker{verified: verified, perUser: map[string]bool{}}
}

func (f *fakeMFAChecker) setVerified(userID string, verified bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.perUser[userID] = verified
}

func (f *fakeMFAChecker) HasVerifiedMFA(_ context.Context, userID string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return false, f.err
	}
	if v, ok := f.perUser[userID]; ok {
		return v, nil
	}
	return f.verified, nil
}

// --- fakeGeoIndex --------------------------------------------------------

type fakeGeoEntry struct {
	PresenceEntry
	pos GeoPosition
}

type fakeGeoIndex struct {
	mu         sync.Mutex
	entries    map[string]fakeGeoEntry
	searchErr  error
	upsertErr  error
	touchErr   error
	detailErr  error
	omitDetail map[string]bool // userIDs Detail should silently omit, simulating a TTL race between Search and Detail
}

func newFakeGeoIndex() *fakeGeoIndex {
	return &fakeGeoIndex{entries: map[string]fakeGeoEntry{}}
}

func (g *fakeGeoIndex) Upsert(_ context.Context, entry PresenceEntry, _ time.Duration) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.upsertErr != nil {
		return g.upsertErr
	}
	g.entries[entry.UserID] = fakeGeoEntry{PresenceEntry: entry, pos: entry.Position}
	return nil
}

func (g *fakeGeoIndex) Touch(_ context.Context, userID string, _ time.Duration) (GeoPosition, bool, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.touchErr != nil {
		return GeoPosition{}, false, g.touchErr
	}
	e, ok := g.entries[userID]
	if !ok {
		return GeoPosition{}, false, nil
	}
	return e.pos, true, nil
}

func (g *fakeGeoIndex) Remove(_ context.Context, userID string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.entries, userID)
	return nil
}

func (g *fakeGeoIndex) Search(_ context.Context, pos GeoPosition, radiusM float64, excludeUserID string) ([]NearbyCandidate, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.searchErr != nil {
		return nil, g.searchErr
	}
	var out []NearbyCandidate
	for id, e := range g.entries {
		if id == excludeUserID {
			continue
		}
		if DistanceMeters(pos, e.pos) <= radiusM {
			out = append(out, NearbyCandidate{UserID: id, Position: e.pos})
		}
	}
	return out, nil
}

func (g *fakeGeoIndex) Detail(_ context.Context, userIDs []string) (map[string]PresenceEntry, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.detailErr != nil {
		return nil, g.detailErr
	}
	out := make(map[string]PresenceEntry, len(userIDs))
	for _, id := range userIDs {
		if g.omitDetail[id] {
			continue // simulates the entry's TTL expiring between Search and Detail
		}
		if e, ok := g.entries[id]; ok {
			out[id] = e.PresenceEntry
		}
	}
	return out, nil
}

// --- fakeAuditLogRepo ------------------------------------------------------

type fakeAuditLogRepo struct {
	mu      sync.Mutex
	entries []AuditLogEntry
	err     error
}

func newFakeAuditLogRepo() *fakeAuditLogRepo {
	return &fakeAuditLogRepo{}
}

func (a *fakeAuditLogRepo) Append(_ context.Context, e AuditLogEntry) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.err != nil {
		return a.err
	}
	a.entries = append(a.entries, e)
	return nil
}

func (a *fakeAuditLogRepo) all() []AuditLogEntry {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]AuditLogEntry, len(a.entries))
	copy(out, a.entries)
	return out
}

// --- fakeProfileResolver ---------------------------------------------------

type fakeProfileResolver struct {
	mu       sync.Mutex
	profiles map[string]Profile
	err      error
	calls    [][]string
}

func newFakeProfileResolver() *fakeProfileResolver {
	return &fakeProfileResolver{profiles: map[string]Profile{}}
}

func (p *fakeProfileResolver) Resolve(_ context.Context, userIDs []string) (map[string]Profile, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls = append(p.calls, append([]string{}, userIDs...))
	if p.err != nil {
		return nil, p.err
	}
	out := make(map[string]Profile, len(userIDs))
	for _, id := range userIDs {
		if pr, ok := p.profiles[id]; ok {
			out[id] = pr
		}
	}
	return out, nil
}

// --- fakeRateLimiter ---------------------------------------------------

// fakeRateLimiter allows everything by default; set denyKeys to force a
// specific key to be denied (simulating an already-exhausted window).
type fakeRateLimiter struct {
	mu         sync.Mutex
	denyKeys   map[string]bool
	calls      []string
	err        error
	lastLimit  int
	lastWindow time.Duration
}

func newFakeRateLimiter() *fakeRateLimiter {
	return &fakeRateLimiter{denyKeys: map[string]bool{}}
}

func (r *fakeRateLimiter) Allow(_ context.Context, key string, limit int, window time.Duration) (bool, time.Duration, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, key)
	r.lastLimit = limit
	r.lastWindow = window
	if r.err != nil {
		return false, 0, r.err
	}
	if r.denyKeys[key] {
		return false, time.Second, nil
	}
	return true, 0, nil
}

// --- fakeIDGen (reuse idgen.Sequential via alias not needed; keep local) ---
