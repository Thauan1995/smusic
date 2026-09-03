package redisstore

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"smusic/backend/internal/playback"
)

func newTestStore(t *testing.T) (*Store, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return New(client), mr
}

func TestStore_SaveLoadRoundTrip(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()

	state := playback.SessionState{
		SessionID: "s1", UserID: "u1", TrackID: "t1", PositionMs: 1000,
		IsPlaying: true, Queue: []string{"t2", "t3"}, UpdatedAt: time.Now().UTC().Truncate(time.Second),
	}
	require.NoError(t, store.Save(ctx, state, time.Minute))

	got, err := store.Load(ctx, "s1")
	require.NoError(t, err)
	assert.Equal(t, state.SessionID, got.SessionID)
	assert.Equal(t, state.UserID, got.UserID)
	assert.Equal(t, state.TrackID, got.TrackID)
	assert.Equal(t, state.PositionMs, got.PositionMs)
	assert.Equal(t, state.IsPlaying, got.IsPlaying)
	assert.Equal(t, state.Queue, got.Queue)
	assert.True(t, state.UpdatedAt.Equal(got.UpdatedAt))
}

func TestStore_Load_NotFound(t *testing.T) {
	store, _ := newTestStore(t)
	_, err := store.Load(context.Background(), "does-not-exist")
	require.ErrorIs(t, err, playback.ErrSessionNotFound)
}

func TestStore_TTLExpiry(t *testing.T) {
	store, mr := newTestStore(t)
	ctx := context.Background()

	require.NoError(t, store.Save(ctx, playback.SessionState{SessionID: "s1", UserID: "u1"}, time.Second))
	mr.FastForward(2 * time.Second)

	_, err := store.Load(ctx, "s1")
	require.ErrorIs(t, err, playback.ErrSessionNotFound)
}

func TestStore_Delete(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()

	require.NoError(t, store.Save(ctx, playback.SessionState{SessionID: "s1", UserID: "u1"}, time.Minute))
	require.NoError(t, store.Delete(ctx, "s1"))

	_, err := store.Load(ctx, "s1")
	require.ErrorIs(t, err, playback.ErrSessionNotFound)
}

func TestStore_Delete_Idempotent(t *testing.T) {
	store, _ := newTestStore(t)
	require.NoError(t, store.Delete(context.Background(), "never-existed"))
}

func TestStore_Save_ClientError(t *testing.T) {
	store, mr := newTestStore(t)
	mr.Close()
	err := store.Save(context.Background(), playback.SessionState{SessionID: "s1", UserID: "u1"}, time.Minute)
	require.Error(t, err)
}

func TestStore_Load_ClientError(t *testing.T) {
	store, mr := newTestStore(t)
	mr.Close()
	_, err := store.Load(context.Background(), "s1")
	require.Error(t, err)
	require.False(t, err == playback.ErrSessionNotFound)
}

func TestStore_Delete_ClientError(t *testing.T) {
	store, mr := newTestStore(t)
	mr.Close()
	err := store.Delete(context.Background(), "s1")
	require.Error(t, err)
}

func TestStore_Load_CorruptData(t *testing.T) {
	store, mr := newTestStore(t)
	require.NoError(t, mr.Set(sessionKey("s1"), "not-json"))
	_, err := store.Load(context.Background(), "s1")
	require.Error(t, err)
}
