package redisstore

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"smusic/backend/internal/presence"
)

func newTestStore(t *testing.T) (*Store, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return New(client), mr
}

func TestStore_UpsertThenDetail(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()

	entry := presence.PresenceEntry{
		UserID: "u1", Position: presence.GeoPosition{Lat: -23.5505, Lon: -46.6333},
		TrackID: "t1", ShareTrack: true, Visibility: presence.VisibilityEveryone,
		UpdatedAt: time.Now().UTC().Truncate(time.Second),
	}
	require.NoError(t, s.Upsert(ctx, entry, time.Minute))

	details, err := s.Detail(ctx, []string{"u1"})
	require.NoError(t, err)
	require.Contains(t, details, "u1")
	assert.Equal(t, "t1", details["u1"].TrackID)
	assert.True(t, details["u1"].ShareTrack)
	assert.Equal(t, presence.VisibilityEveryone, details["u1"].Visibility)
}

func TestStore_Detail_MissingUsersOmitted(t *testing.T) {
	s, _ := newTestStore(t)
	details, err := s.Detail(context.Background(), []string{"nope"})
	require.NoError(t, err)
	assert.Empty(t, details)
}

func TestStore_Detail_EmptyInput(t *testing.T) {
	s, _ := newTestStore(t)
	details, err := s.Detail(context.Background(), nil)
	require.NoError(t, err)
	assert.Empty(t, details)
}

func TestStore_Touch_RenewsTTLAndReturnsPosition(t *testing.T) {
	s, mr := newTestStore(t)
	ctx := context.Background()
	pos := presence.GeoPosition{Lat: 10, Lon: 20}
	require.NoError(t, s.Upsert(ctx, presence.PresenceEntry{UserID: "u1", Position: pos}, time.Minute))

	mr.FastForward(50 * time.Second)

	got, ok, err := s.Touch(ctx, "u1", time.Minute)
	require.NoError(t, err)
	require.True(t, ok)
	assert.InDelta(t, pos.Lat, got.Lat, 0.001)
	assert.InDelta(t, pos.Lon, got.Lon, 0.001)

	mr.FastForward(50 * time.Second) // would have expired without the renewal above
	_, ok2, err := s.Touch(ctx, "u1", time.Minute)
	require.NoError(t, err)
	assert.True(t, ok2, "TTL should have been renewed by the first Touch")
}

func TestStore_Touch_ExpiredOrMissing_NotOK(t *testing.T) {
	s, _ := newTestStore(t)
	_, ok, err := s.Touch(context.Background(), "ghost", time.Minute)
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestStore_Touch_TTLExpiryViaFastForward(t *testing.T) {
	s, mr := newTestStore(t)
	ctx := context.Background()
	require.NoError(t, s.Upsert(ctx, presence.PresenceEntry{UserID: "u1", Position: presence.GeoPosition{Lat: 1, Lon: 1}}, 10*time.Second))
	mr.FastForward(11 * time.Second)

	_, ok, err := s.Touch(ctx, "u1", time.Minute)
	require.NoError(t, err)
	assert.False(t, ok, "entry should have expired")
}

func TestStore_Remove(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()
	require.NoError(t, s.Upsert(ctx, presence.PresenceEntry{UserID: "u1", Position: presence.GeoPosition{Lat: 1, Lon: 1}}, time.Minute))

	require.NoError(t, s.Remove(ctx, "u1"))

	_, ok, err := s.Touch(ctx, "u1", time.Minute)
	require.NoError(t, err)
	assert.False(t, ok)

	details, err := s.Detail(ctx, []string{"u1"})
	require.NoError(t, err)
	assert.Empty(t, details)
}

func TestStore_Remove_Idempotent(t *testing.T) {
	s, _ := newTestStore(t)
	assert.NoError(t, s.Remove(context.Background(), "never-existed"))
}

func TestStore_Search_FindsWithinRadiusExcludesSelf(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()

	origin := presence.GeoPosition{Lat: -23.5505, Lon: -46.6333}
	near := presence.GeoPosition{Lat: -23.5510, Lon: -46.6333} // a few hundred meters
	far := presence.GeoPosition{Lat: -22.9068, Lon: -43.1729}  // Rio, ~357km away

	require.NoError(t, s.Upsert(ctx, presence.PresenceEntry{UserID: "me", Position: origin}, time.Minute))
	require.NoError(t, s.Upsert(ctx, presence.PresenceEntry{UserID: "near", Position: near}, time.Minute))
	require.NoError(t, s.Upsert(ctx, presence.PresenceEntry{UserID: "far", Position: far}, time.Minute))

	results, err := s.Search(ctx, origin, 15000, "me")
	require.NoError(t, err)

	ids := map[string]bool{}
	for _, r := range results {
		ids[r.UserID] = true
	}
	assert.True(t, ids["near"])
	assert.False(t, ids["far"])
	assert.False(t, ids["me"], "search must exclude the requester")
}

func TestStore_Search_ReturnsCoordinates(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()
	pos := presence.GeoPosition{Lat: -23.5505, Lon: -46.6333}
	require.NoError(t, s.Upsert(ctx, presence.PresenceEntry{UserID: "u1", Position: pos}, time.Minute))

	results, err := s.Search(ctx, pos, 1000, "")
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.InDelta(t, pos.Lat, results[0].Position.Lat, 0.001)
	assert.InDelta(t, pos.Lon, results[0].Position.Lon, 0.001)
}

func TestStore_Search_NoResults(t *testing.T) {
	s, _ := newTestStore(t)
	results, err := s.Search(context.Background(), presence.GeoPosition{Lat: 0, Lon: 0}, 100, "")
	require.NoError(t, err)
	assert.Empty(t, results)
}

// --- Redis-command-failure branches -----------------------------------
//
// miniredis.SetError makes every subsequent command fail until cleared,
// which is a blunt instrument (it can't isolate "the Nth command in a
// multi-command method fails but the (N-1)th succeeded" the way real fault
// injection could) -- so these tests cover each method's FIRST Redis call
// failing. The remaining "later call in the same method fails after an
// earlier one already succeeded" branches are the same category
// internal/platform/cache/ratelimiter.go already documents as
// coverage:ignore (not reproducible with miniredis, no per-command fault
// injection) -- same justification applies here and is noted at each
// remaining branch's call site in geoindex.go.

func TestStore_Upsert_GeoAddError(t *testing.T) {
	s, mr := newTestStore(t)
	mr.SetError("boom")
	err := s.Upsert(context.Background(), presence.PresenceEntry{UserID: "u1", Position: presence.GeoPosition{Lat: 1, Lon: 1}}, time.Minute)
	assert.Error(t, err)
}

func TestStore_Touch_GetError(t *testing.T) {
	s, mr := newTestStore(t)
	require.NoError(t, s.Upsert(context.Background(), presence.PresenceEntry{UserID: "u1", Position: presence.GeoPosition{Lat: 1, Lon: 1}}, time.Minute))
	mr.SetError("boom")
	_, _, err := s.Touch(context.Background(), "u1", time.Minute)
	assert.Error(t, err)
}

func TestStore_Remove_ZRemError(t *testing.T) {
	s, mr := newTestStore(t)
	mr.SetError("boom")
	err := s.Remove(context.Background(), "u1")
	assert.Error(t, err)
}

func TestStore_Search_GeoSearchError(t *testing.T) {
	s, mr := newTestStore(t)
	mr.SetError("boom")
	_, err := s.Search(context.Background(), presence.GeoPosition{Lat: 0, Lon: 0}, 100, "")
	assert.Error(t, err)
}

// TestStore_Touch_DetailHashAliveButGeoMemberGone exercises Touch's
// defensive branch for when the two structures have (briefly) diverged:
// the detail hash is still alive but the GEO sorted-set member is already
// gone (e.g. an explicit Remove raced with this Touch).
func TestStore_Touch_DetailHashAliveButGeoMemberGone(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()
	require.NoError(t, s.Upsert(ctx, presence.PresenceEntry{UserID: "u1", Position: presence.GeoPosition{Lat: 1, Lon: 1}}, time.Minute))

	// Remove only the GEO member, leaving the detail hash (with its TTL)
	// alive -- simulating the exact race Touch's comment describes.
	require.NoError(t, s.client.ZRem(ctx, geoKey, "u1").Err())

	_, ok, err := s.Touch(ctx, "u1", time.Minute)
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestStore_Detail_MGetError(t *testing.T) {
	s, mr := newTestStore(t)
	mr.SetError("boom")
	_, err := s.Detail(context.Background(), []string{"u1"})
	assert.Error(t, err)
}
