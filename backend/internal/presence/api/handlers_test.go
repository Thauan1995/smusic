package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"smusic/backend/internal/platform/middleware"
	"smusic/backend/internal/presence"
)

type fakeService struct {
	settings   presence.PrivacySettings
	getErr     error
	updateErr  error
	consentErr error
	pauseErr   error
	blockErr   error
	unblockErr error

	lastUpdate presence.UpdateSettingsInput
	lastBlock  [2]string
}

func (f *fakeService) Get(context.Context, string) (presence.PrivacySettings, error) {
	return f.settings, f.getErr
}
func (f *fakeService) Update(_ context.Context, _ string, in presence.UpdateSettingsInput) (presence.PrivacySettings, error) {
	f.lastUpdate = in
	return f.settings, f.updateErr
}
func (f *fakeService) GrantConsent(context.Context, string) (presence.PrivacySettings, error) {
	return f.settings, f.consentErr
}
func (f *fakeService) RevokeConsent(context.Context, string) (presence.PrivacySettings, error) {
	return f.settings, f.consentErr
}
func (f *fakeService) SetPaused(context.Context, string, bool) (presence.PrivacySettings, error) {
	return f.settings, f.pauseErr
}
func (f *fakeService) Block(_ context.Context, blocker, blocked string) error {
	f.lastBlock = [2]string{blocker, blocked}
	return f.blockErr
}
func (f *fakeService) Unblock(_ context.Context, blocker, blocked string) error {
	f.lastBlock = [2]string{blocker, blocked}
	return f.unblockErr
}

type fakeAuthr struct{}

func (fakeAuthr) Authenticate(token string) (string, error) { return "u1", nil }

func newTestRouter(svc *fakeService) http.Handler {
	r := chi.NewRouter()
	NewHandler(svc).Mount(r, fakeAuthr{})
	return r
}

func authedRequest(method, path string, body []byte) *http.Request {
	var r *http.Request
	if body != nil {
		r = httptest.NewRequest(method, path, bytes.NewReader(body))
	} else {
		r = httptest.NewRequest(method, path, nil)
	}
	r.Header.Set("Authorization", "Bearer anything")
	r.Header.Set("Content-Type", "application/json")
	return r
}

func TestGetSettings(t *testing.T) {
	svc := &fakeService{settings: presence.PrivacySettings{PresenceVisibility: presence.VisibilityEveryone, VisibilityRadiusM: 1000}}
	router := newTestRouter(svc)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authedRequest(http.MethodGet, "/v1/presence/settings", nil))
	require.Equal(t, http.StatusOK, w.Code)

	var resp settingsResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, presence.VisibilityEveryone, resp.PresenceVisibility)
	assert.Equal(t, 1000, resp.VisibilityRadiusM)
}

func TestGetSettings_Unauthenticated(t *testing.T) {
	svc := &fakeService{}
	router := newTestRouter(svc)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/presence/settings", nil))
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestUpdateSettings(t *testing.T) {
	svc := &fakeService{}
	router := newTestRouter(svc)

	body := []byte(`{"visibility_radius_m": 5000, "paused": false}`)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, authedRequest(http.MethodPut, "/v1/presence/settings", body))
	require.Equal(t, http.StatusOK, w.Code)
	require.NotNil(t, svc.lastUpdate.VisibilityRadiusM)
	assert.Equal(t, 5000, *svc.lastUpdate.VisibilityRadiusM)
	require.NotNil(t, svc.lastUpdate.Paused)
	assert.False(t, *svc.lastUpdate.Paused)
}

func TestUpdateSettings_InvalidJSON(t *testing.T) {
	svc := &fakeService{}
	router := newTestRouter(svc)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, authedRequest(http.MethodPut, "/v1/presence/settings", []byte(`{bad`)))
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdateSettings_ServiceValidationError(t *testing.T) {
	svc := &fakeService{updateErr: presence.ErrInvalidRadius}
	router := newTestRouter(svc)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, authedRequest(http.MethodPut, "/v1/presence/settings", []byte(`{}`)))
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGrantConsent(t *testing.T) {
	svc := &fakeService{settings: presence.PrivacySettings{ProximityConsentEnabled: true}}
	router := newTestRouter(svc)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, authedRequest(http.MethodPost, "/v1/presence/consent", nil))
	require.Equal(t, http.StatusOK, w.Code)
	var resp settingsResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp.ProximityConsentEnabled)
}

func TestRevokeConsent(t *testing.T) {
	svc := &fakeService{}
	router := newTestRouter(svc)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, authedRequest(http.MethodDelete, "/v1/presence/consent", nil))
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestPauseResume(t *testing.T) {
	svc := &fakeService{}
	router := newTestRouter(svc)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authedRequest(http.MethodPost, "/v1/presence/pause", nil))
	assert.Equal(t, http.StatusOK, w.Code)

	w = httptest.NewRecorder()
	router.ServeHTTP(w, authedRequest(http.MethodPost, "/v1/presence/resume", nil))
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestBlockUnblock(t *testing.T) {
	svc := &fakeService{}
	router := newTestRouter(svc)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authedRequest(http.MethodPost, "/v1/presence/blocks/target-1", nil))
	require.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, [2]string{"u1", "target-1"}, svc.lastBlock)

	w = httptest.NewRecorder()
	router.ServeHTTP(w, authedRequest(http.MethodDelete, "/v1/presence/blocks/target-1", nil))
	require.Equal(t, http.StatusNoContent, w.Code)
}

func TestBlock_CannotBlockSelf(t *testing.T) {
	svc := &fakeService{blockErr: presence.ErrCannotBlockSelf}
	router := newTestRouter(svc)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, authedRequest(http.MethodPost, "/v1/presence/blocks/u1", nil))
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestBlock_InternalError(t *testing.T) {
	svc := &fakeService{blockErr: assertErrAPI("boom")}
	router := newTestRouter(svc)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, authedRequest(http.MethodPost, "/v1/presence/blocks/u2", nil))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestGetSettings_FullyPopulated(t *testing.T) {
	ts := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	due := ts.Add(presence.ConsentValidityPeriod)
	svc := &fakeService{settings: presence.PrivacySettings{
		PresenceVisibility: presence.VisibilityEveryone, VisibilityRadiusM: 1000,
		ProximityConsentEnabled: true, ProximityConsentTS: &ts, ProximityConsentRenewDue: &due,
	}}
	router := newTestRouter(svc)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, authedRequest(http.MethodGet, "/v1/presence/settings", nil))
	require.Equal(t, http.StatusOK, w.Code)

	var resp settingsResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.NotNil(t, resp.ProximityConsentTS)
	require.NotNil(t, resp.ProximityConsentRenewDue)
	assert.Contains(t, *resp.ProximityConsentTS, "2026-01-01")
}

func TestGrantConsent_Error(t *testing.T) {
	svc := &fakeService{consentErr: assertErrAPI("boom")}
	router := newTestRouter(svc)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, authedRequest(http.MethodPost, "/v1/presence/consent", nil))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestRevokeConsent_Error(t *testing.T) {
	svc := &fakeService{consentErr: assertErrAPI("boom")}
	router := newTestRouter(svc)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, authedRequest(http.MethodDelete, "/v1/presence/consent", nil))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestPause_Error(t *testing.T) {
	svc := &fakeService{pauseErr: assertErrAPI("boom")}
	router := newTestRouter(svc)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, authedRequest(http.MethodPost, "/v1/presence/pause", nil))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestResume_Error(t *testing.T) {
	svc := &fakeService{pauseErr: assertErrAPI("boom")}
	router := newTestRouter(svc)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, authedRequest(http.MethodPost, "/v1/presence/resume", nil))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestUnblock_Error(t *testing.T) {
	svc := &fakeService{unblockErr: assertErrAPI("boom")}
	router := newTestRouter(svc)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, authedRequest(http.MethodDelete, "/v1/presence/blocks/u2", nil))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestGetSettings_NotFoundError(t *testing.T) {
	svc := &fakeService{getErr: presence.ErrSettingsNotFound}
	router := newTestRouter(svc)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, authedRequest(http.MethodGet, "/v1/presence/settings", nil))
	assert.Equal(t, http.StatusNotFound, w.Code)
}

type assertErrAPI string

func (e assertErrAPI) Error() string { return string(e) }

var _ middleware.Authenticator = fakeAuthr{}
