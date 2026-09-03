package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"

	"smusic/backend/internal/playback"
)

type fakeService struct {
	createSessionFn func(ctx context.Context, userID, deviceID string) (playback.SessionState, error)
	getStateFn      func(ctx context.Context, sessionID, requesterID string) (playback.SessionState, error)
	playFn          func(ctx context.Context, sessionID, requesterID, trackID string, positionMs int) (playback.PlayResult, error)
	pauseFn         func(ctx context.Context, sessionID, requesterID string) (playback.SessionState, error)
	seekFn          func(ctx context.Context, sessionID, requesterID string, positionMs int) (playback.SessionState, error)
	nextFn          func(ctx context.Context, sessionID, requesterID string) (playback.PlayResult, error)
	enqueueFn       func(ctx context.Context, sessionID, requesterID string, trackIDs []string, position string) (playback.SessionState, error)
}

func (f *fakeService) CreateSession(ctx context.Context, userID, deviceID string) (playback.SessionState, error) {
	return f.createSessionFn(ctx, userID, deviceID)
}
func (f *fakeService) GetState(ctx context.Context, sessionID, requesterID string) (playback.SessionState, error) {
	return f.getStateFn(ctx, sessionID, requesterID)
}
func (f *fakeService) Play(ctx context.Context, sessionID, requesterID, trackID string, positionMs int) (playback.PlayResult, error) {
	return f.playFn(ctx, sessionID, requesterID, trackID, positionMs)
}
func (f *fakeService) Pause(ctx context.Context, sessionID, requesterID string) (playback.SessionState, error) {
	return f.pauseFn(ctx, sessionID, requesterID)
}
func (f *fakeService) Seek(ctx context.Context, sessionID, requesterID string, positionMs int) (playback.SessionState, error) {
	return f.seekFn(ctx, sessionID, requesterID, positionMs)
}
func (f *fakeService) Next(ctx context.Context, sessionID, requesterID string) (playback.PlayResult, error) {
	return f.nextFn(ctx, sessionID, requesterID)
}
func (f *fakeService) Enqueue(ctx context.Context, sessionID, requesterID string, trackIDs []string, position string) (playback.SessionState, error) {
	return f.enqueueFn(ctx, sessionID, requesterID, trackIDs, position)
}

type fakeAuthenticator struct{}

func (fakeAuthenticator) Authenticate(token string) (string, error) {
	if token != "valid" {
		return "", errors.New("invalid")
	}
	return "user-1", nil
}

func newTestRouter(svc *fakeService) chi.Router {
	r := chi.NewRouter()
	NewHandler(svc).Mount(r, fakeAuthenticator{})
	return r
}

func doRequest(r chi.Router, method, path, body string, auth bool) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if auth {
		req.Header.Set("Authorization", "Bearer valid")
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestCreateSession_Success(t *testing.T) {
	svc := &fakeService{createSessionFn: func(ctx context.Context, userID, deviceID string) (playback.SessionState, error) {
		assert.Equal(t, "user-1", userID)
		assert.Equal(t, "dev1", deviceID)
		return playback.SessionState{SessionID: "s1"}, nil
	}}
	w := doRequest(newTestRouter(svc), http.MethodPost, "/v1/playback/sessions", `{"device_id":"dev1"}`, true)
	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Contains(t, w.Body.String(), "s1")
}

func TestCreateSession_RequiresAuth(t *testing.T) {
	svc := &fakeService{}
	w := doRequest(newTestRouter(svc), http.MethodPost, "/v1/playback/sessions", `{}`, false)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestCreateSession_InvalidBody(t *testing.T) {
	svc := &fakeService{}
	w := doRequest(newTestRouter(svc), http.MethodPost, "/v1/playback/sessions", `{`, true)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateSession_ServiceError(t *testing.T) {
	svc := &fakeService{createSessionFn: func(ctx context.Context, userID, deviceID string) (playback.SessionState, error) {
		return playback.SessionState{}, playback.ErrInvalidInput
	}}
	w := doRequest(newTestRouter(svc), http.MethodPost, "/v1/playback/sessions", `{}`, true)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetState_Success(t *testing.T) {
	svc := &fakeService{getStateFn: func(ctx context.Context, sessionID, requesterID string) (playback.SessionState, error) {
		assert.Equal(t, "s1", sessionID)
		return playback.SessionState{TrackID: "t1", PositionMs: 100, IsPlaying: true, Queue: []string{"t2"}}, nil
	}}
	w := doRequest(newTestRouter(svc), http.MethodGet, "/v1/playback/sessions/s1/state", "", true)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"t2"`)
}

func TestGetState_EmptyQueueRendersAsEmptyArray(t *testing.T) {
	svc := &fakeService{getStateFn: func(ctx context.Context, sessionID, requesterID string) (playback.SessionState, error) {
		return playback.SessionState{}, nil
	}}
	w := doRequest(newTestRouter(svc), http.MethodGet, "/v1/playback/sessions/s1/state", "", true)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"queue":[]`)
}

func TestGetState_NotFound(t *testing.T) {
	svc := &fakeService{getStateFn: func(ctx context.Context, sessionID, requesterID string) (playback.SessionState, error) {
		return playback.SessionState{}, playback.ErrSessionNotFound
	}}
	w := doRequest(newTestRouter(svc), http.MethodGet, "/v1/playback/sessions/s1/state", "", true)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestPlay_Success(t *testing.T) {
	svc := &fakeService{playFn: func(ctx context.Context, sessionID, requesterID, trackID string, positionMs int) (playback.PlayResult, error) {
		assert.Equal(t, "t1", trackID)
		assert.Equal(t, 500, positionMs)
		return playback.PlayResult{StreamURL: "http://x/t1.mp3", ExpiresAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}, nil
	}}
	w := doRequest(newTestRouter(svc), http.MethodPost, "/v1/playback/sessions/s1/play", `{"track_id":"t1","position_ms":500}`, true)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "http://x/t1.mp3")
}

func TestPlay_InvalidBody(t *testing.T) {
	svc := &fakeService{}
	w := doRequest(newTestRouter(svc), http.MethodPost, "/v1/playback/sessions/s1/play", `{`, true)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPlay_TrackNotFound(t *testing.T) {
	svc := &fakeService{playFn: func(ctx context.Context, sessionID, requesterID, trackID string, positionMs int) (playback.PlayResult, error) {
		return playback.PlayResult{}, playback.ErrTrackNotFound
	}}
	w := doRequest(newTestRouter(svc), http.MethodPost, "/v1/playback/sessions/s1/play", `{"track_id":"t1"}`, true)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestPause_Success(t *testing.T) {
	svc := &fakeService{pauseFn: func(ctx context.Context, sessionID, requesterID string) (playback.SessionState, error) {
		return playback.SessionState{}, nil
	}}
	w := doRequest(newTestRouter(svc), http.MethodPost, "/v1/playback/sessions/s1/pause", "", true)
	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestPause_Forbidden(t *testing.T) {
	svc := &fakeService{pauseFn: func(ctx context.Context, sessionID, requesterID string) (playback.SessionState, error) {
		return playback.SessionState{}, playback.ErrForbidden
	}}
	w := doRequest(newTestRouter(svc), http.MethodPost, "/v1/playback/sessions/s1/pause", "", true)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestSeek_Success(t *testing.T) {
	svc := &fakeService{seekFn: func(ctx context.Context, sessionID, requesterID string, positionMs int) (playback.SessionState, error) {
		assert.Equal(t, 1234, positionMs)
		return playback.SessionState{}, nil
	}}
	w := doRequest(newTestRouter(svc), http.MethodPost, "/v1/playback/sessions/s1/seek", `{"position_ms":1234}`, true)
	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestSeek_InvalidBody(t *testing.T) {
	svc := &fakeService{}
	w := doRequest(newTestRouter(svc), http.MethodPost, "/v1/playback/sessions/s1/seek", `{`, true)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSeek_ServiceError(t *testing.T) {
	svc := &fakeService{seekFn: func(ctx context.Context, sessionID, requesterID string, positionMs int) (playback.SessionState, error) {
		return playback.SessionState{}, playback.ErrInvalidInput
	}}
	w := doRequest(newTestRouter(svc), http.MethodPost, "/v1/playback/sessions/s1/seek", `{"position_ms":-1}`, true)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestNext_Success(t *testing.T) {
	svc := &fakeService{nextFn: func(ctx context.Context, sessionID, requesterID string) (playback.PlayResult, error) {
		return playback.PlayResult{TrackID: "t2", StreamURL: "http://x/t2.mp3", ExpiresAt: time.Now()}, nil
	}}
	w := doRequest(newTestRouter(svc), http.MethodPost, "/v1/playback/sessions/s1/next", "", true)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "t2")
}

func TestNext_EmptyQueue(t *testing.T) {
	svc := &fakeService{nextFn: func(ctx context.Context, sessionID, requesterID string) (playback.PlayResult, error) {
		return playback.PlayResult{}, playback.ErrEmptyQueue
	}}
	w := doRequest(newTestRouter(svc), http.MethodPost, "/v1/playback/sessions/s1/next", "", true)
	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestEnqueue_Success(t *testing.T) {
	svc := &fakeService{enqueueFn: func(ctx context.Context, sessionID, requesterID string, trackIDs []string, position string) (playback.SessionState, error) {
		assert.Equal(t, []string{"t1", "t2"}, trackIDs)
		assert.Equal(t, "next", position)
		return playback.SessionState{}, nil
	}}
	w := doRequest(newTestRouter(svc), http.MethodPost, "/v1/playback/sessions/s1/queue", `{"track_ids":["t1","t2"],"position":"next"}`, true)
	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestEnqueue_InvalidBody(t *testing.T) {
	svc := &fakeService{}
	w := doRequest(newTestRouter(svc), http.MethodPost, "/v1/playback/sessions/s1/queue", `{`, true)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestEnqueue_ServiceError(t *testing.T) {
	svc := &fakeService{enqueueFn: func(ctx context.Context, sessionID, requesterID string, trackIDs []string, position string) (playback.SessionState, error) {
		return playback.SessionState{}, errors.New("boom")
	}}
	w := doRequest(newTestRouter(svc), http.MethodPost, "/v1/playback/sessions/s1/queue", `{"track_ids":["t1"]}`, true)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
