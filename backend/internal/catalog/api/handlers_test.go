package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"smusic/backend/internal/catalog"
)

type fakeService struct {
	createArtistFn func(ctx context.Context, in catalog.CreateArtistInput) (catalog.Artist, error)
	getArtistFn    func(ctx context.Context, id string) (catalog.Artist, error)
	createAlbumFn  func(ctx context.Context, in catalog.CreateAlbumInput) (catalog.Album, error)
	getAlbumFn     func(ctx context.Context, id string) (catalog.Album, []catalog.Track, error)
	createTrackFn  func(ctx context.Context, in catalog.CreateTrackInput) (catalog.Track, error)
	getTrackFn     func(ctx context.Context, id string) (catalog.Track, []catalog.AudioAsset, error)
	searchFn       func(ctx context.Context, in catalog.SearchInput) (catalog.SearchOutput, error)
}

func (f *fakeService) CreateArtist(ctx context.Context, in catalog.CreateArtistInput) (catalog.Artist, error) {
	return f.createArtistFn(ctx, in)
}
func (f *fakeService) GetArtist(ctx context.Context, id string) (catalog.Artist, error) {
	return f.getArtistFn(ctx, id)
}
func (f *fakeService) CreateAlbum(ctx context.Context, in catalog.CreateAlbumInput) (catalog.Album, error) {
	return f.createAlbumFn(ctx, in)
}
func (f *fakeService) GetAlbum(ctx context.Context, id string) (catalog.Album, []catalog.Track, error) {
	return f.getAlbumFn(ctx, id)
}
func (f *fakeService) CreateTrack(ctx context.Context, in catalog.CreateTrackInput) (catalog.Track, error) {
	return f.createTrackFn(ctx, in)
}
func (f *fakeService) GetTrack(ctx context.Context, id string) (catalog.Track, []catalog.AudioAsset, error) {
	return f.getTrackFn(ctx, id)
}
func (f *fakeService) Search(ctx context.Context, in catalog.SearchInput) (catalog.SearchOutput, error) {
	return f.searchFn(ctx, in)
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

func TestCreateArtist_Success(t *testing.T) {
	svc := &fakeService{createArtistFn: func(ctx context.Context, in catalog.CreateArtistInput) (catalog.Artist, error) {
		assert.Equal(t, "Daft Punk", in.Name)
		return catalog.Artist{ID: "a1", Name: "Daft Punk"}, nil
	}}
	w := doRequest(newTestRouter(svc), http.MethodPost, "/v1/catalog/artists", `{"name":"Daft Punk"}`, true)
	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestCreateArtist_RequiresAuth(t *testing.T) {
	svc := &fakeService{}
	w := doRequest(newTestRouter(svc), http.MethodPost, "/v1/catalog/artists", `{"name":"X"}`, false)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestCreateArtist_InvalidBody(t *testing.T) {
	svc := &fakeService{}
	w := doRequest(newTestRouter(svc), http.MethodPost, "/v1/catalog/artists", `{`, true)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateArtist_ServiceError(t *testing.T) {
	svc := &fakeService{createArtistFn: func(ctx context.Context, in catalog.CreateArtistInput) (catalog.Artist, error) {
		return catalog.Artist{}, catalog.ErrInvalidInput
	}}
	w := doRequest(newTestRouter(svc), http.MethodPost, "/v1/catalog/artists", `{"name":"X"}`, true)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetArtist_Success(t *testing.T) {
	svc := &fakeService{getArtistFn: func(ctx context.Context, id string) (catalog.Artist, error) {
		assert.Equal(t, "a1", id)
		return catalog.Artist{ID: "a1", Name: "X"}, nil
	}}
	w := doRequest(newTestRouter(svc), http.MethodGet, "/v1/catalog/artists/a1", "", false)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetArtist_NotFound(t *testing.T) {
	svc := &fakeService{getArtistFn: func(ctx context.Context, id string) (catalog.Artist, error) {
		return catalog.Artist{}, catalog.ErrArtistNotFound
	}}
	w := doRequest(newTestRouter(svc), http.MethodGet, "/v1/catalog/artists/nope", "", false)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestCreateAlbum_Success(t *testing.T) {
	svc := &fakeService{createAlbumFn: func(ctx context.Context, in catalog.CreateAlbumInput) (catalog.Album, error) {
		return catalog.Album{ID: "al1", Title: in.Title}, nil
	}}
	w := doRequest(newTestRouter(svc), http.MethodPost, "/v1/catalog/albums", `{"title":"Discovery"}`, true)
	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestCreateAlbum_InvalidBody(t *testing.T) {
	svc := &fakeService{}
	w := doRequest(newTestRouter(svc), http.MethodPost, "/v1/catalog/albums", `{`, true)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateAlbum_ServiceError(t *testing.T) {
	svc := &fakeService{createAlbumFn: func(ctx context.Context, in catalog.CreateAlbumInput) (catalog.Album, error) {
		return catalog.Album{}, catalog.ErrInvalidInput
	}}
	w := doRequest(newTestRouter(svc), http.MethodPost, "/v1/catalog/albums", `{"title":"X"}`, true)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetAlbum_Success(t *testing.T) {
	svc := &fakeService{getAlbumFn: func(ctx context.Context, id string) (catalog.Album, []catalog.Track, error) {
		return catalog.Album{ID: "al1", Title: "T"}, []catalog.Track{{ID: "t1", Title: "S1"}}, nil
	}}
	w := doRequest(newTestRouter(svc), http.MethodGet, "/v1/catalog/albums/al1", "", false)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"S1"`)
}

func TestGetAlbum_NotFound(t *testing.T) {
	svc := &fakeService{getAlbumFn: func(ctx context.Context, id string) (catalog.Album, []catalog.Track, error) {
		return catalog.Album{}, nil, catalog.ErrAlbumNotFound
	}}
	w := doRequest(newTestRouter(svc), http.MethodGet, "/v1/catalog/albums/nope", "", false)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestCreateTrack_Success(t *testing.T) {
	svc := &fakeService{createTrackFn: func(ctx context.Context, in catalog.CreateTrackInput) (catalog.Track, error) {
		require.Len(t, in.Artists, 1)
		assert.Equal(t, "artist-1", in.Artists[0].ArtistID)
		return catalog.Track{ID: "t1", Title: in.Title}, nil
	}}
	w := doRequest(newTestRouter(svc), http.MethodPost, "/v1/catalog/tracks",
		`{"title":"Song","duration_ms":1000,"artists":[{"artist_id":"artist-1","role":"primary"}]}`, true)
	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestCreateTrack_InvalidBody(t *testing.T) {
	svc := &fakeService{}
	w := doRequest(newTestRouter(svc), http.MethodPost, "/v1/catalog/tracks", `{`, true)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateTrack_ServiceError(t *testing.T) {
	svc := &fakeService{createTrackFn: func(ctx context.Context, in catalog.CreateTrackInput) (catalog.Track, error) {
		return catalog.Track{}, catalog.ErrInvalidInput
	}}
	w := doRequest(newTestRouter(svc), http.MethodPost, "/v1/catalog/tracks", `{"title":"X","duration_ms":1}`, true)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetTrack_Success(t *testing.T) {
	svc := &fakeService{getTrackFn: func(ctx context.Context, id string) (catalog.Track, []catalog.AudioAsset, error) {
		return catalog.Track{ID: "t1", Title: "S"}, []catalog.AudioAsset{{QualityTier: "high", Codec: "aac", BitrateKbps: 256}}, nil
	}}
	w := doRequest(newTestRouter(svc), http.MethodGet, "/v1/catalog/tracks/t1", "", false)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"bitrate_kbps":256`)
}

func TestGetTrack_NotFound(t *testing.T) {
	svc := &fakeService{getTrackFn: func(ctx context.Context, id string) (catalog.Track, []catalog.AudioAsset, error) {
		return catalog.Track{}, nil, catalog.ErrTrackNotFound
	}}
	w := doRequest(newTestRouter(svc), http.MethodGet, "/v1/catalog/tracks/nope", "", false)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetTrack_InternalError(t *testing.T) {
	svc := &fakeService{getTrackFn: func(ctx context.Context, id string) (catalog.Track, []catalog.AudioAsset, error) {
		return catalog.Track{}, nil, errors.New("boom")
	}}
	w := doRequest(newTestRouter(svc), http.MethodGet, "/v1/catalog/tracks/x", "", false)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestSearch_Success(t *testing.T) {
	svc := &fakeService{searchFn: func(ctx context.Context, in catalog.SearchInput) (catalog.SearchOutput, error) {
		assert.Equal(t, "daft", in.Query)
		assert.Equal(t, catalog.SearchType("artist"), in.Type)
		assert.Equal(t, 5, in.Limit)
		assert.Equal(t, "cur1", in.Cursor)
		return catalog.SearchOutput{Artists: []catalog.Artist{{ID: "a1", Name: "Daft Punk"}}, NextCursor: "cur2"}, nil
	}}
	w := doRequest(newTestRouter(svc), http.MethodGet, "/v1/catalog/search?q=daft&type=artist&limit=5&cursor=cur1", "", false)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"cur2"`)
}

func TestSearch_TrackResultsWithCredits(t *testing.T) {
	svc := &fakeService{searchFn: func(ctx context.Context, in catalog.SearchInput) (catalog.SearchOutput, error) {
		return catalog.SearchOutput{Tracks: []catalog.Track{{
			ID: "t1", Title: "Song",
			Artists: []catalog.TrackArtist{{ArtistID: "a1", ArtistName: "Daft Punk", Role: "primary"}},
		}}}, nil
	}}
	w := doRequest(newTestRouter(svc), http.MethodGet, "/v1/catalog/search?q=song&type=track", "", false)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"Daft Punk"`)
}

func TestSearch_AlbumResults(t *testing.T) {
	svc := &fakeService{searchFn: func(ctx context.Context, in catalog.SearchInput) (catalog.SearchOutput, error) {
		return catalog.SearchOutput{Albums: []catalog.Album{{ID: "al1", Title: "Discovery"}}}, nil
	}}
	w := doRequest(newTestRouter(svc), http.MethodGet, "/v1/catalog/search?q=disco&type=album", "", false)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"Discovery"`)
}

func TestSearch_InvalidLimit(t *testing.T) {
	svc := &fakeService{}
	w := doRequest(newTestRouter(svc), http.MethodGet, "/v1/catalog/search?q=x&limit=abc", "", false)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSearch_NegativeLimit(t *testing.T) {
	svc := &fakeService{}
	w := doRequest(newTestRouter(svc), http.MethodGet, "/v1/catalog/search?q=x&limit=-1", "", false)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSearch_ServiceError(t *testing.T) {
	svc := &fakeService{searchFn: func(ctx context.Context, in catalog.SearchInput) (catalog.SearchOutput, error) {
		return catalog.SearchOutput{}, catalog.ErrInvalidInput
	}}
	w := doRequest(newTestRouter(svc), http.MethodGet, "/v1/catalog/search?q=", "", false)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSearch_NoLimitParam(t *testing.T) {
	svc := &fakeService{searchFn: func(ctx context.Context, in catalog.SearchInput) (catalog.SearchOutput, error) {
		assert.Equal(t, 0, in.Limit)
		return catalog.SearchOutput{}, nil
	}}
	w := doRequest(newTestRouter(svc), http.MethodGet, "/v1/catalog/search?q=x", "", false)
	assert.Equal(t, http.StatusOK, w.Code)
}
