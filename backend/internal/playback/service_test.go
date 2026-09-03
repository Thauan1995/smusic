package playback

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"smusic/backend/internal/platform/clock"
	"smusic/backend/internal/platform/idgen"
)

var errBoom = errors.New("boom")

const sessionTTL = 24 * time.Hour

type testDeps struct {
	store    *fakeStateStore
	resolver *fakeResolver
	tracks   *fakeTrackChecker
	events   *fakeEventRecorder
	clock    *clock.Frozen
}

func newTestService(t *testing.T, existingTracks ...string) (*Service, *testDeps) {
	t.Helper()
	d := &testDeps{
		store:    newFakeStateStore(),
		resolver: &fakeResolver{},
		tracks:   newFakeTrackChecker(existingTracks...),
		events:   &fakeEventRecorder{},
		clock:    clock.NewFrozen(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)),
	}
	svc := NewService(d.store, d.resolver, d.tracks, d.events, d.clock, idgen.NewSequential("id"), sessionTTL)
	return svc, d
}

// --- CreateSession ---

func TestCreateSession_Success(t *testing.T) {
	svc, _ := newTestService(t)
	s, err := svc.CreateSession(context.Background(), "u1", "dev1")
	require.NoError(t, err)
	assert.NotEmpty(t, s.SessionID)
	assert.Equal(t, "u1", s.UserID)
	assert.Equal(t, "dev1", s.DeviceID)
	assert.False(t, s.IsPlaying)
}

func TestCreateSession_MissingUser(t *testing.T) {
	svc, _ := newTestService(t)
	_, err := svc.CreateSession(context.Background(), "", "")
	assert.ErrorIs(t, err, ErrInvalidInput)
}

func TestCreateSession_StoreError(t *testing.T) {
	svc, d := newTestService(t)
	d.store.saveErr = errBoom
	_, err := svc.CreateSession(context.Background(), "u1", "")
	require.Error(t, err)
}

// --- GetState ---

func TestGetState_Success(t *testing.T) {
	svc, _ := newTestService(t)
	s, err := svc.CreateSession(context.Background(), "u1", "")
	require.NoError(t, err)

	got, err := svc.GetState(context.Background(), s.SessionID, "u1")
	require.NoError(t, err)
	assert.Equal(t, s.SessionID, got.SessionID)
}

func TestGetState_MissingArgs(t *testing.T) {
	svc, _ := newTestService(t)
	_, err := svc.GetState(context.Background(), "", "u1")
	assert.ErrorIs(t, err, ErrInvalidInput)
	_, err = svc.GetState(context.Background(), "s1", "")
	assert.ErrorIs(t, err, ErrInvalidInput)
}

func TestGetState_NotFound(t *testing.T) {
	svc, _ := newTestService(t)
	_, err := svc.GetState(context.Background(), "nope", "u1")
	assert.ErrorIs(t, err, ErrSessionNotFound)
}

func TestGetState_LoadOtherError(t *testing.T) {
	svc, d := newTestService(t)
	d.store.loadErr = errBoom
	_, err := svc.GetState(context.Background(), "s1", "u1")
	require.Error(t, err)
	assert.False(t, errors.Is(err, ErrSessionNotFound))
}

func TestGetState_Forbidden(t *testing.T) {
	svc, _ := newTestService(t)
	s, err := svc.CreateSession(context.Background(), "u1", "")
	require.NoError(t, err)

	_, err = svc.GetState(context.Background(), s.SessionID, "u2")
	assert.ErrorIs(t, err, ErrForbidden)
}

// --- Play ---

func TestPlay_Success(t *testing.T) {
	svc, d := newTestService(t, "t1")
	s, err := svc.CreateSession(context.Background(), "u1", "")
	require.NoError(t, err)

	result, err := svc.Play(context.Background(), s.SessionID, "u1", "t1", 5000)
	require.NoError(t, err)
	assert.Equal(t, "t1", result.TrackID)
	assert.Contains(t, result.StreamURL, "t1")
	assert.True(t, result.State.IsPlaying)
	assert.Equal(t, 5000, result.State.PositionMs)
	require.Len(t, d.events.events, 1)
	assert.Equal(t, "u1", d.events.events[0].UserID)
}

func TestPlay_MissingTrackID(t *testing.T) {
	svc, _ := newTestService(t)
	s, err := svc.CreateSession(context.Background(), "u1", "")
	require.NoError(t, err)
	_, err = svc.Play(context.Background(), s.SessionID, "u1", "", 0)
	assert.ErrorIs(t, err, ErrInvalidInput)
}

func TestPlay_NegativePosition(t *testing.T) {
	svc, _ := newTestService(t, "t1")
	s, err := svc.CreateSession(context.Background(), "u1", "")
	require.NoError(t, err)
	_, err = svc.Play(context.Background(), s.SessionID, "u1", "t1", -1)
	assert.ErrorIs(t, err, ErrInvalidInput)
}

func TestPlay_SessionNotFound(t *testing.T) {
	svc, _ := newTestService(t, "t1")
	_, err := svc.Play(context.Background(), "nope", "u1", "t1", 0)
	assert.ErrorIs(t, err, ErrSessionNotFound)
}

func TestPlay_Forbidden(t *testing.T) {
	svc, _ := newTestService(t, "t1")
	s, err := svc.CreateSession(context.Background(), "u1", "")
	require.NoError(t, err)
	_, err = svc.Play(context.Background(), s.SessionID, "u2", "t1", 0)
	assert.ErrorIs(t, err, ErrForbidden)
}

func TestPlay_TrackNotFound(t *testing.T) {
	svc, _ := newTestService(t)
	s, err := svc.CreateSession(context.Background(), "u1", "")
	require.NoError(t, err)
	_, err = svc.Play(context.Background(), s.SessionID, "u1", "nope", 0)
	assert.ErrorIs(t, err, ErrTrackNotFound)
}

func TestPlay_TrackCheckerError(t *testing.T) {
	svc, d := newTestService(t)
	s, err := svc.CreateSession(context.Background(), "u1", "")
	require.NoError(t, err)
	d.tracks.err = errBoom
	_, err = svc.Play(context.Background(), s.SessionID, "u1", "t1", 0)
	require.Error(t, err)
}

func TestPlay_ResolverError(t *testing.T) {
	svc, d := newTestService(t, "t1")
	s, err := svc.CreateSession(context.Background(), "u1", "")
	require.NoError(t, err)
	d.resolver.resolveErr = errBoom
	_, err = svc.Play(context.Background(), s.SessionID, "u1", "t1", 0)
	require.Error(t, err)
}

func TestPlay_SaveError(t *testing.T) {
	svc, d := newTestService(t, "t1")
	s, err := svc.CreateSession(context.Background(), "u1", "")
	require.NoError(t, err)
	d.store.saveErr = errBoom
	_, err = svc.Play(context.Background(), s.SessionID, "u1", "t1", 0)
	require.Error(t, err)
}

func TestPlay_EventRecordError(t *testing.T) {
	svc, d := newTestService(t, "t1")
	s, err := svc.CreateSession(context.Background(), "u1", "")
	require.NoError(t, err)
	d.events.err = errBoom
	_, err = svc.Play(context.Background(), s.SessionID, "u1", "t1", 0)
	require.Error(t, err)
}

// --- Pause ---

func TestPause_Success(t *testing.T) {
	svc, _ := newTestService(t, "t1")
	s, err := svc.CreateSession(context.Background(), "u1", "")
	require.NoError(t, err)
	_, err = svc.Play(context.Background(), s.SessionID, "u1", "t1", 0)
	require.NoError(t, err)

	got, err := svc.Pause(context.Background(), s.SessionID, "u1")
	require.NoError(t, err)
	assert.False(t, got.IsPlaying)
}

func TestPause_NotFound(t *testing.T) {
	svc, _ := newTestService(t)
	_, err := svc.Pause(context.Background(), "nope", "u1")
	assert.ErrorIs(t, err, ErrSessionNotFound)
}

func TestPause_Forbidden(t *testing.T) {
	svc, _ := newTestService(t)
	s, err := svc.CreateSession(context.Background(), "u1", "")
	require.NoError(t, err)
	_, err = svc.Pause(context.Background(), s.SessionID, "u2")
	assert.ErrorIs(t, err, ErrForbidden)
}

func TestPause_SaveError(t *testing.T) {
	svc, d := newTestService(t)
	s, err := svc.CreateSession(context.Background(), "u1", "")
	require.NoError(t, err)
	d.store.saveErr = errBoom
	_, err = svc.Pause(context.Background(), s.SessionID, "u1")
	require.Error(t, err)
}

// --- Seek ---

func TestSeek_Success(t *testing.T) {
	svc, _ := newTestService(t)
	s, err := svc.CreateSession(context.Background(), "u1", "")
	require.NoError(t, err)

	got, err := svc.Seek(context.Background(), s.SessionID, "u1", 42_000)
	require.NoError(t, err)
	assert.Equal(t, 42_000, got.PositionMs)
}

func TestSeek_NegativePosition(t *testing.T) {
	svc, _ := newTestService(t)
	s, err := svc.CreateSession(context.Background(), "u1", "")
	require.NoError(t, err)
	_, err = svc.Seek(context.Background(), s.SessionID, "u1", -1)
	assert.ErrorIs(t, err, ErrInvalidInput)
}

func TestSeek_NotFound(t *testing.T) {
	svc, _ := newTestService(t)
	_, err := svc.Seek(context.Background(), "nope", "u1", 0)
	assert.ErrorIs(t, err, ErrSessionNotFound)
}

func TestSeek_Forbidden(t *testing.T) {
	svc, _ := newTestService(t)
	s, err := svc.CreateSession(context.Background(), "u1", "")
	require.NoError(t, err)
	_, err = svc.Seek(context.Background(), s.SessionID, "u2", 0)
	assert.ErrorIs(t, err, ErrForbidden)
}

func TestSeek_SaveError(t *testing.T) {
	svc, d := newTestService(t)
	s, err := svc.CreateSession(context.Background(), "u1", "")
	require.NoError(t, err)
	d.store.saveErr = errBoom
	_, err = svc.Seek(context.Background(), s.SessionID, "u1", 0)
	require.Error(t, err)
}

// --- Next ---

func TestNext_Success(t *testing.T) {
	svc, _ := newTestService(t, "t1", "t2")
	s, err := svc.CreateSession(context.Background(), "u1", "")
	require.NoError(t, err)
	_, err = svc.Enqueue(context.Background(), s.SessionID, "u1", []string{"t1", "t2"}, "")
	require.NoError(t, err)

	result, err := svc.Next(context.Background(), s.SessionID, "u1")
	require.NoError(t, err)
	assert.Equal(t, "t1", result.TrackID)
	assert.Equal(t, []string{"t2"}, result.State.Queue)
	assert.Equal(t, 0, result.State.PositionMs)
}

func TestNext_EmptyQueue(t *testing.T) {
	svc, _ := newTestService(t)
	s, err := svc.CreateSession(context.Background(), "u1", "")
	require.NoError(t, err)
	_, err = svc.Next(context.Background(), s.SessionID, "u1")
	assert.ErrorIs(t, err, ErrEmptyQueue)
}

func TestNext_NotFound(t *testing.T) {
	svc, _ := newTestService(t)
	_, err := svc.Next(context.Background(), "nope", "u1")
	assert.ErrorIs(t, err, ErrSessionNotFound)
}

func TestNext_Forbidden(t *testing.T) {
	svc, _ := newTestService(t)
	s, err := svc.CreateSession(context.Background(), "u1", "")
	require.NoError(t, err)
	_, err = svc.Next(context.Background(), s.SessionID, "u2")
	assert.ErrorIs(t, err, ErrForbidden)
}

func TestNext_SaveError(t *testing.T) {
	svc, d := newTestService(t, "t1")
	s, err := svc.CreateSession(context.Background(), "u1", "")
	require.NoError(t, err)
	_, err = svc.Enqueue(context.Background(), s.SessionID, "u1", []string{"t1"}, "")
	require.NoError(t, err)

	d.store.saveErr = errBoom
	_, err = svc.Next(context.Background(), s.SessionID, "u1")
	require.Error(t, err)
}

func TestNext_ResolverError(t *testing.T) {
	svc, d := newTestService(t, "t1")
	s, err := svc.CreateSession(context.Background(), "u1", "")
	require.NoError(t, err)
	_, err = svc.Enqueue(context.Background(), s.SessionID, "u1", []string{"t1"}, "")
	require.NoError(t, err)

	d.resolver.resolveErr = errBoom
	_, err = svc.Next(context.Background(), s.SessionID, "u1")
	require.Error(t, err)
}

// --- Enqueue ---

func TestEnqueue_AppendsToEnd(t *testing.T) {
	svc, _ := newTestService(t, "t1", "t2")
	s, err := svc.CreateSession(context.Background(), "u1", "")
	require.NoError(t, err)

	_, err = svc.Enqueue(context.Background(), s.SessionID, "u1", []string{"t1"}, "")
	require.NoError(t, err)
	got, err := svc.Enqueue(context.Background(), s.SessionID, "u1", []string{"t2"}, EnqueuePositionEnd)
	require.NoError(t, err)
	assert.Equal(t, []string{"t1", "t2"}, got.Queue)
}

func TestEnqueue_InsertsNext(t *testing.T) {
	svc, _ := newTestService(t, "t1", "t2")
	s, err := svc.CreateSession(context.Background(), "u1", "")
	require.NoError(t, err)

	_, err = svc.Enqueue(context.Background(), s.SessionID, "u1", []string{"t1"}, "")
	require.NoError(t, err)
	got, err := svc.Enqueue(context.Background(), s.SessionID, "u1", []string{"t2"}, EnqueuePositionNext)
	require.NoError(t, err)
	assert.Equal(t, []string{"t2", "t1"}, got.Queue)
}

func TestEnqueue_EmptyList(t *testing.T) {
	svc, _ := newTestService(t)
	s, err := svc.CreateSession(context.Background(), "u1", "")
	require.NoError(t, err)
	_, err = svc.Enqueue(context.Background(), s.SessionID, "u1", nil, "")
	assert.ErrorIs(t, err, ErrInvalidInput)
}

func TestEnqueue_EmptyTrackID(t *testing.T) {
	svc, _ := newTestService(t)
	s, err := svc.CreateSession(context.Background(), "u1", "")
	require.NoError(t, err)
	_, err = svc.Enqueue(context.Background(), s.SessionID, "u1", []string{""}, "")
	assert.ErrorIs(t, err, ErrInvalidInput)
}

func TestEnqueue_NotFound(t *testing.T) {
	svc, _ := newTestService(t)
	_, err := svc.Enqueue(context.Background(), "nope", "u1", []string{"t1"}, "")
	assert.ErrorIs(t, err, ErrSessionNotFound)
}

func TestEnqueue_Forbidden(t *testing.T) {
	svc, _ := newTestService(t)
	s, err := svc.CreateSession(context.Background(), "u1", "")
	require.NoError(t, err)
	_, err = svc.Enqueue(context.Background(), s.SessionID, "u2", []string{"t1"}, "")
	assert.ErrorIs(t, err, ErrForbidden)
}

func TestEnqueue_TrackNotFound(t *testing.T) {
	svc, _ := newTestService(t)
	s, err := svc.CreateSession(context.Background(), "u1", "")
	require.NoError(t, err)
	_, err = svc.Enqueue(context.Background(), s.SessionID, "u1", []string{"nope"}, "")
	assert.ErrorIs(t, err, ErrTrackNotFound)
}

func TestEnqueue_TrackCheckerError(t *testing.T) {
	svc, d := newTestService(t)
	s, err := svc.CreateSession(context.Background(), "u1", "")
	require.NoError(t, err)
	d.tracks.err = errBoom
	_, err = svc.Enqueue(context.Background(), s.SessionID, "u1", []string{"t1"}, "")
	require.Error(t, err)
}

func TestEnqueue_SaveError(t *testing.T) {
	svc, d := newTestService(t, "t1")
	s, err := svc.CreateSession(context.Background(), "u1", "")
	require.NoError(t, err)
	d.store.saveErr = errBoom
	_, err = svc.Enqueue(context.Background(), s.SessionID, "u1", []string{"t1"}, "")
	require.Error(t, err)
}
