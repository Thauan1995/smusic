package ws

import (
	"context"
	"sync"
	"time"

	"smusic/backend/internal/presence"
)

// This file provides minimal, concurrency-safe fakes of every
// presence.* repository/index interface, used only to build a real
// NearbyService/Hub for this package's end-to-end WS handler tests. The
// actual privacy-filter *behavior* is exhaustively tested in
// internal/presence's own test suite (in-package, with equivalent fakes);
// here the goal is only to exercise the WS transport (handshake, frame
// parsing, connection lifecycle) against a realistic, working service.

type fakeSettingsRepoWS struct {
	mu     sync.Mutex
	rows   map[string]presence.PrivacySettings
	getErr error
}

func newFakeSettingsRepo() *fakeSettingsRepoWS {
	return &fakeSettingsRepoWS{rows: map[string]presence.PrivacySettings{}}
}

func (f *fakeSettingsRepoWS) Get(_ context.Context, userID string) (presence.PrivacySettings, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.getErr != nil {
		return presence.PrivacySettings{}, f.getErr
	}
	s, ok := f.rows[userID]
	if !ok {
		return presence.PrivacySettings{}, presence.ErrSettingsNotFound
	}
	return s, nil
}

func (f *fakeSettingsRepoWS) Upsert(_ context.Context, s presence.PrivacySettings) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rows[s.UserID] = s
	return nil
}

type fakeBlockRepoWS struct{}

func newFakeBlockRepoWS() fakeBlockRepoWS { return fakeBlockRepoWS{} }

func (fakeBlockRepoWS) Block(context.Context, string, string) error   { return nil }
func (fakeBlockRepoWS) Unblock(context.Context, string, string) error { return nil }
func (fakeBlockRepoWS) IsBlockedEitherWay(context.Context, string, string) (bool, error) {
	return false, nil
}

type fakeFollowCheckerWS struct{}

func newFakeFollowCheckerWS() fakeFollowCheckerWS { return fakeFollowCheckerWS{} }

func (fakeFollowCheckerWS) IsMutualFollow(context.Context, string, string) (bool, error) {
	return false, nil
}

type geoEntryWS struct {
	entry presence.PresenceEntry
	pos   presence.GeoPosition
}

type fakeGeoIndexWS struct {
	mu      sync.Mutex
	entries map[string]geoEntryWS
}

func newFakeGeoIndexWS() *fakeGeoIndexWS { return &fakeGeoIndexWS{entries: map[string]geoEntryWS{}} }

func (g *fakeGeoIndexWS) Upsert(_ context.Context, entry presence.PresenceEntry, _ time.Duration) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.entries[entry.UserID] = geoEntryWS{entry: entry, pos: entry.Position}
	return nil
}

func (g *fakeGeoIndexWS) Touch(_ context.Context, userID string, _ time.Duration) (presence.GeoPosition, bool, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	e, ok := g.entries[userID]
	if !ok {
		return presence.GeoPosition{}, false, nil
	}
	return e.pos, true, nil
}

func (g *fakeGeoIndexWS) Remove(_ context.Context, userID string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.entries, userID)
	return nil
}

func (g *fakeGeoIndexWS) Search(_ context.Context, pos presence.GeoPosition, radiusM float64, exclude string) ([]presence.NearbyCandidate, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	var out []presence.NearbyCandidate
	for id, e := range g.entries {
		if id == exclude {
			continue
		}
		if presence.DistanceMeters(pos, e.pos) <= radiusM {
			out = append(out, presence.NearbyCandidate{UserID: id, Position: e.pos})
		}
	}
	return out, nil
}

func (g *fakeGeoIndexWS) Detail(_ context.Context, ids []string) (map[string]presence.PresenceEntry, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	out := make(map[string]presence.PresenceEntry, len(ids))
	for _, id := range ids {
		if e, ok := g.entries[id]; ok {
			out[id] = e.entry
		}
	}
	return out, nil
}

type fakeAuditRepoWS struct{}

func newFakeAuditRepoWS() fakeAuditRepoWS { return fakeAuditRepoWS{} }

func (fakeAuditRepoWS) Append(context.Context, presence.AuditLogEntry) error { return nil }

type fakeProfileResolverWS struct{}

func newFakeProfileResolverWS() fakeProfileResolverWS { return fakeProfileResolverWS{} }

func (fakeProfileResolverWS) Resolve(_ context.Context, ids []string) (map[string]presence.Profile, error) {
	return map[string]presence.Profile{}, nil
}

type fakeRateLimiterWS struct{}

func newFakeRateLimiterWS() fakeRateLimiterWS { return fakeRateLimiterWS{} }

func (fakeRateLimiterWS) Allow(context.Context, string, int, time.Duration) (bool, time.Duration, error) {
	return true, 0, nil
}
