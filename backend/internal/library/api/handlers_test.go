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

	"smusic/backend/internal/library"
)

type fakeService struct {
	createPlaylistFn     func(ctx context.Context, in library.CreatePlaylistInput) (library.Playlist, error)
	listPlaylistsFn      func(ctx context.Context, ownerID string) ([]library.Playlist, error)
	listPlaylistTracksFn func(ctx context.Context, playlistID, requesterID string) (library.Playlist, []library.PlaylistTrack, error)
	addTrackFn           func(ctx context.Context, playlistID, requesterID, trackID, position string) (library.PlaylistTrack, error)
	removeTrackFn        func(ctx context.Context, playlistID, requesterID, trackID string) error
	saveTrackFn          func(ctx context.Context, userID, trackID string) error
	unsaveTrackFn        func(ctx context.Context, userID, trackID string) error
	listSavedTracksFn    func(ctx context.Context, userID, cursor string, limit int) ([]library.LibraryTrack, string, error)
}

func (f *fakeService) CreatePlaylist(ctx context.Context, in library.CreatePlaylistInput) (library.Playlist, error) {
	return f.createPlaylistFn(ctx, in)
}
func (f *fakeService) ListPlaylists(ctx context.Context, ownerID string) ([]library.Playlist, error) {
	return f.listPlaylistsFn(ctx, ownerID)
}
func (f *fakeService) ListPlaylistTracks(ctx context.Context, playlistID, requesterID string) (library.Playlist, []library.PlaylistTrack, error) {
	return f.listPlaylistTracksFn(ctx, playlistID, requesterID)
}
func (f *fakeService) AddTrack(ctx context.Context, playlistID, requesterID, trackID, position string) (library.PlaylistTrack, error) {
	return f.addTrackFn(ctx, playlistID, requesterID, trackID, position)
}
func (f *fakeService) RemoveTrack(ctx context.Context, playlistID, requesterID, trackID string) error {
	return f.removeTrackFn(ctx, playlistID, requesterID, trackID)
}
func (f *fakeService) SaveTrack(ctx context.Context, userID, trackID string) error {
	return f.saveTrackFn(ctx, userID, trackID)
}
func (f *fakeService) UnsaveTrack(ctx context.Context, userID, trackID string) error {
	return f.unsaveTrackFn(ctx, userID, trackID)
}
func (f *fakeService) ListSavedTracks(ctx context.Context, userID, cursor string, limit int) ([]library.LibraryTrack, string, error) {
	return f.listSavedTracksFn(ctx, userID, cursor, limit)
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

func TestCreatePlaylist_Success(t *testing.T) {
	svc := &fakeService{createPlaylistFn: func(ctx context.Context, in library.CreatePlaylistInput) (library.Playlist, error) {
		assert.Equal(t, "user-1", in.OwnerID)
		return library.Playlist{ID: "p1", Title: in.Title}, nil
	}}
	w := doRequest(newTestRouter(svc), http.MethodPost, "/v1/library/me/playlists", `{"title":"Mix"}`, true)
	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestCreatePlaylist_RequiresAuth(t *testing.T) {
	svc := &fakeService{}
	w := doRequest(newTestRouter(svc), http.MethodPost, "/v1/library/me/playlists", `{"title":"Mix"}`, false)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestCreatePlaylist_InvalidBody(t *testing.T) {
	svc := &fakeService{}
	w := doRequest(newTestRouter(svc), http.MethodPost, "/v1/library/me/playlists", `{`, true)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreatePlaylist_ServiceError(t *testing.T) {
	svc := &fakeService{createPlaylistFn: func(ctx context.Context, in library.CreatePlaylistInput) (library.Playlist, error) {
		return library.Playlist{}, library.ErrInvalidInput
	}}
	w := doRequest(newTestRouter(svc), http.MethodPost, "/v1/library/me/playlists", `{"title":"X"}`, true)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestListPlaylists_Success(t *testing.T) {
	svc := &fakeService{listPlaylistsFn: func(ctx context.Context, ownerID string) ([]library.Playlist, error) {
		return []library.Playlist{{ID: "p1", Title: "Mix"}}, nil
	}}
	w := doRequest(newTestRouter(svc), http.MethodGet, "/v1/library/me/playlists", "", true)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Mix")
}

func TestListPlaylists_Error(t *testing.T) {
	svc := &fakeService{listPlaylistsFn: func(ctx context.Context, ownerID string) ([]library.Playlist, error) {
		return nil, errors.New("boom")
	}}
	w := doRequest(newTestRouter(svc), http.MethodGet, "/v1/library/me/playlists", "", true)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestListPlaylistTracks_Success(t *testing.T) {
	svc := &fakeService{listPlaylistTracksFn: func(ctx context.Context, playlistID, requesterID string) (library.Playlist, []library.PlaylistTrack, error) {
		assert.Equal(t, "p1", playlistID)
		return library.Playlist{ID: "p1"}, []library.PlaylistTrack{{TrackID: "t1", AddedAt: time.Now()}}, nil
	}}
	w := doRequest(newTestRouter(svc), http.MethodGet, "/v1/library/me/playlists/p1/tracks", "", true)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestListPlaylistTracks_Forbidden(t *testing.T) {
	svc := &fakeService{listPlaylistTracksFn: func(ctx context.Context, playlistID, requesterID string) (library.Playlist, []library.PlaylistTrack, error) {
		return library.Playlist{}, nil, library.ErrNotOwner
	}}
	w := doRequest(newTestRouter(svc), http.MethodGet, "/v1/library/me/playlists/p1/tracks", "", true)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestAddTrack_Success(t *testing.T) {
	svc := &fakeService{addTrackFn: func(ctx context.Context, playlistID, requesterID, trackID, position string) (library.PlaylistTrack, error) {
		assert.Equal(t, "p1", playlistID)
		assert.Equal(t, "t1", trackID)
		assert.Equal(t, "start", position)
		return library.PlaylistTrack{}, nil
	}}
	w := doRequest(newTestRouter(svc), http.MethodPost, "/v1/library/me/playlists/p1/tracks", `{"track_id":"t1","position":"start"}`, true)
	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestAddTrack_InvalidBody(t *testing.T) {
	svc := &fakeService{}
	w := doRequest(newTestRouter(svc), http.MethodPost, "/v1/library/me/playlists/p1/tracks", `{`, true)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAddTrack_NotFound(t *testing.T) {
	svc := &fakeService{addTrackFn: func(ctx context.Context, playlistID, requesterID, trackID, position string) (library.PlaylistTrack, error) {
		return library.PlaylistTrack{}, library.ErrTrackNotFound
	}}
	w := doRequest(newTestRouter(svc), http.MethodPost, "/v1/library/me/playlists/p1/tracks", `{"track_id":"t1"}`, true)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestRemoveTrack_Success(t *testing.T) {
	svc := &fakeService{removeTrackFn: func(ctx context.Context, playlistID, requesterID, trackID string) error {
		assert.Equal(t, "p1", playlistID)
		assert.Equal(t, "t1", trackID)
		return nil
	}}
	w := doRequest(newTestRouter(svc), http.MethodDelete, "/v1/library/me/playlists/p1/tracks/t1", "", true)
	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestRemoveTrack_NotInPlaylist(t *testing.T) {
	svc := &fakeService{removeTrackFn: func(ctx context.Context, playlistID, requesterID, trackID string) error {
		return library.ErrTrackNotInPlaylist
	}}
	w := doRequest(newTestRouter(svc), http.MethodDelete, "/v1/library/me/playlists/p1/tracks/t1", "", true)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestSaveTrack_Success(t *testing.T) {
	svc := &fakeService{saveTrackFn: func(ctx context.Context, userID, trackID string) error {
		assert.Equal(t, "user-1", userID)
		assert.Equal(t, "t1", trackID)
		return nil
	}}
	w := doRequest(newTestRouter(svc), http.MethodPost, "/v1/library/me/saved-tracks", `{"track_id":"t1"}`, true)
	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestSaveTrack_InvalidBody(t *testing.T) {
	svc := &fakeService{}
	w := doRequest(newTestRouter(svc), http.MethodPost, "/v1/library/me/saved-tracks", `{`, true)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSaveTrack_NotFound(t *testing.T) {
	svc := &fakeService{saveTrackFn: func(ctx context.Context, userID, trackID string) error {
		return library.ErrTrackNotFound
	}}
	w := doRequest(newTestRouter(svc), http.MethodPost, "/v1/library/me/saved-tracks", `{"track_id":"t1"}`, true)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestUnsaveTrack_Success(t *testing.T) {
	svc := &fakeService{unsaveTrackFn: func(ctx context.Context, userID, trackID string) error {
		assert.Equal(t, "t1", trackID)
		return nil
	}}
	w := doRequest(newTestRouter(svc), http.MethodDelete, "/v1/library/me/saved-tracks/t1", "", true)
	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestUnsaveTrack_Error(t *testing.T) {
	svc := &fakeService{unsaveTrackFn: func(ctx context.Context, userID, trackID string) error {
		return errors.New("boom")
	}}
	w := doRequest(newTestRouter(svc), http.MethodDelete, "/v1/library/me/saved-tracks/t1", "", true)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestListSavedTracks_Success(t *testing.T) {
	svc := &fakeService{listSavedTracksFn: func(ctx context.Context, userID, cursor string, limit int) ([]library.LibraryTrack, string, error) {
		assert.Equal(t, 10, limit)
		assert.Equal(t, "cur1", cursor)
		return []library.LibraryTrack{{TrackID: "t1", AddedAt: time.Now()}}, "cur2", nil
	}}
	w := doRequest(newTestRouter(svc), http.MethodGet, "/v1/library/me/saved-tracks?limit=10&cursor=cur1", "", true)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "cur2")
}

func TestListSavedTracks_InvalidLimit(t *testing.T) {
	svc := &fakeService{}
	w := doRequest(newTestRouter(svc), http.MethodGet, "/v1/library/me/saved-tracks?limit=abc", "", true)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestListSavedTracks_NegativeLimit(t *testing.T) {
	svc := &fakeService{}
	w := doRequest(newTestRouter(svc), http.MethodGet, "/v1/library/me/saved-tracks?limit=-1", "", true)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestListSavedTracks_Error(t *testing.T) {
	svc := &fakeService{listSavedTracksFn: func(ctx context.Context, userID, cursor string, limit int) ([]library.LibraryTrack, string, error) {
		return nil, "", errors.New("boom")
	}}
	w := doRequest(newTestRouter(svc), http.MethodGet, "/v1/library/me/saved-tracks", "", true)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestWriteLibraryError_AllBranches(t *testing.T) {
	cases := []struct {
		err    error
		status int
	}{
		{library.ErrInvalidInput, http.StatusBadRequest},
		{library.ErrPlaylistNotFound, http.StatusNotFound},
		{library.ErrTrackNotFound, http.StatusNotFound},
		{library.ErrTrackNotInPlaylist, http.StatusNotFound},
		{library.ErrNotOwner, http.StatusForbidden},
		{errors.New("unmapped"), http.StatusInternalServerError},
	}
	for _, tc := range cases {
		svc := &fakeService{listPlaylistsFn: func(ctx context.Context, ownerID string) ([]library.Playlist, error) {
			return nil, tc.err
		}}
		w := doRequest(newTestRouter(svc), http.MethodGet, "/v1/library/me/playlists", "", true)
		assert.Equal(t, tc.status, w.Code, "err=%v", tc.err)
	}
}
