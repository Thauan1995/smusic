// Package api implements the HTTP transport for the catalog module
// (backend-go.md §4's Catálogo contracts, plus minimal write endpoints —
// the source doc only sketched the read-facing client shape, not
// ingest/population endpoints, which the task explicitly asked for: "CRUD
// mínimo ... o suficiente para popular e listar"). Write endpoints require
// authentication (any authenticated user) as a minimal guard for this
// slice; TODO: gate behind a real admin/ingest role once role-based authz
// exists — out of scope for Fatia 1.
package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"smusic/backend/internal/catalog"
	"smusic/backend/internal/platform/httpx"
	"smusic/backend/internal/platform/middleware"
)

// Service is the subset of *catalog.Service the handlers need.
type Service interface {
	CreateArtist(ctx context.Context, in catalog.CreateArtistInput) (catalog.Artist, error)
	GetArtist(ctx context.Context, id string) (catalog.Artist, error)
	CreateAlbum(ctx context.Context, in catalog.CreateAlbumInput) (catalog.Album, error)
	GetAlbum(ctx context.Context, id string) (catalog.Album, []catalog.Track, error)
	CreateTrack(ctx context.Context, in catalog.CreateTrackInput) (catalog.Track, error)
	GetTrack(ctx context.Context, id string) (catalog.Track, []catalog.AudioAsset, error)
	Search(ctx context.Context, in catalog.SearchInput) (catalog.SearchOutput, error)
}

// Handler holds the catalog module's HTTP handlers.
type Handler struct {
	svc Service
}

// NewHandler returns a Handler backed by svc.
func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

// Mount registers catalog's routes on r. authr protects the write
// endpoints; roles additionally restricts them to auth.RoleCatalogCurator
// (.vibeflow/specs/catalog-write-authorization.md — any authenticated
// user could write shared catalog data before this).
func (h *Handler) Mount(r chi.Router, authr middleware.Authenticator, roles middleware.RoleChecker) {
	r.Get("/v1/catalog/search", h.search)
	r.Get("/v1/catalog/artists/{id}", h.getArtist)
	r.Get("/v1/catalog/albums/{id}", h.getAlbum)
	r.Get("/v1/catalog/tracks/{id}", h.getTrack)

	r.Group(func(r chi.Router) {
		r.Use(middleware.RequireAuth(authr))
		// "catalog_curator" must match auth.RoleCatalogCurator's value —
		// a literal, not an import, because catalog never imports auth
		// directly (backend-go.md §1's module-boundary rule; see
		// presence's identical convention for its own MFAChecker/
		// FollowChecker-style boundary interfaces).
		r.Use(middleware.RequireRole(roles, "catalog_curator"))
		r.Post("/v1/catalog/artists", h.createArtist)
		r.Post("/v1/catalog/albums", h.createAlbum)
		r.Post("/v1/catalog/tracks", h.createTrack)
	})
}

type artistResponse struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Slug     string `json:"slug,omitempty"`
	Bio      string `json:"bio,omitempty"`
	ImageURL string `json:"image_url,omitempty"`
	Verified bool   `json:"verified"`
}

func toArtistResponse(a catalog.Artist) artistResponse {
	return artistResponse{ID: a.ID, Name: a.Name, Slug: a.Slug, Bio: a.Bio, ImageURL: a.ImageURL, Verified: a.Verified}
}

type createArtistRequest struct {
	Name     string `json:"name"`
	Slug     string `json:"slug,omitempty"`
	Bio      string `json:"bio,omitempty"`
	ImageURL string `json:"image_url,omitempty"`
}

func (h *Handler) createArtist(w http.ResponseWriter, r *http.Request) {
	var req createArtistRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	a, err := h.svc.CreateArtist(r.Context(), catalog.CreateArtistInput{Name: req.Name, Slug: req.Slug, Bio: req.Bio, ImageURL: req.ImageURL})
	if err != nil {
		writeCatalogError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, toArtistResponse(a))
}

func (h *Handler) getArtist(w http.ResponseWriter, r *http.Request) {
	a, err := h.svc.GetArtist(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeCatalogError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toArtistResponse(a))
}

type albumResponse struct {
	ID              string          `json:"id"`
	Title           string          `json:"title"`
	PrimaryArtistID string          `json:"primary_artist_id,omitempty"`
	AlbumType       string          `json:"album_type"`
	CoverURL        string          `json:"cover_url,omitempty"`
	Tracks          []trackResponse `json:"tracks,omitempty"`
}

type createAlbumRequest struct {
	Title           string `json:"title"`
	PrimaryArtistID string `json:"primary_artist_id,omitempty"`
	AlbumType       string `json:"album_type,omitempty"`
	CoverURL        string `json:"cover_url,omitempty"`
}

func (h *Handler) createAlbum(w http.ResponseWriter, r *http.Request) {
	var req createAlbumRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	a, err := h.svc.CreateAlbum(r.Context(), catalog.CreateAlbumInput{
		Title: req.Title, PrimaryArtistID: req.PrimaryArtistID, AlbumType: req.AlbumType, CoverURL: req.CoverURL,
	})
	if err != nil {
		writeCatalogError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, albumResponse{ID: a.ID, Title: a.Title, PrimaryArtistID: a.PrimaryArtistID, AlbumType: a.AlbumType, CoverURL: a.CoverURL})
}

func (h *Handler) getAlbum(w http.ResponseWriter, r *http.Request) {
	album, tracks, err := h.svc.GetAlbum(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeCatalogError(w, err)
		return
	}
	resp := albumResponse{ID: album.ID, Title: album.Title, PrimaryArtistID: album.PrimaryArtistID, AlbumType: album.AlbumType, CoverURL: album.CoverURL}
	for _, t := range tracks {
		resp.Tracks = append(resp.Tracks, toTrackResponse(t, nil))
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

type trackArtistResponse struct {
	ArtistID   string `json:"artist_id"`
	ArtistName string `json:"artist_name"`
	Role       string `json:"role"`
}

type audioAssetResponse struct {
	QualityTier string `json:"quality_tier"`
	Codec       string `json:"codec"`
	BitrateKbps int    `json:"bitrate_kbps"`
}

type trackResponse struct {
	ID              string                `json:"id"`
	Title           string                `json:"title"`
	AlbumID         string                `json:"album_id,omitempty"`
	DurationMs      int                   `json:"duration_ms"`
	Explicit        bool                  `json:"explicit"`
	Artists         []trackArtistResponse `json:"artists,omitempty"`
	AvailableAssets []audioAssetResponse  `json:"available_bitrates,omitempty"`
}

func toTrackResponse(t catalog.Track, assets []catalog.AudioAsset) trackResponse {
	resp := trackResponse{ID: t.ID, Title: t.Title, AlbumID: t.AlbumID, DurationMs: t.DurationMs, Explicit: t.Explicit}
	for _, ta := range t.Artists {
		resp.Artists = append(resp.Artists, trackArtistResponse{ArtistID: ta.ArtistID, ArtistName: ta.ArtistName, Role: ta.Role})
	}
	for _, a := range assets {
		resp.AvailableAssets = append(resp.AvailableAssets, audioAssetResponse{QualityTier: a.QualityTier, Codec: a.Codec, BitrateKbps: a.BitrateKbps})
	}
	return resp
}

type createTrackArtistRequest struct {
	ArtistID string `json:"artist_id"`
	Role     string `json:"role"`
}

type createTrackRequest struct {
	Title       string                     `json:"title"`
	AlbumID     string                     `json:"album_id,omitempty"`
	DurationMs  int                        `json:"duration_ms"`
	TrackNumber *int                       `json:"track_number,omitempty"`
	ISRC        string                     `json:"isrc,omitempty"`
	Explicit    bool                       `json:"explicit,omitempty"`
	Artists     []createTrackArtistRequest `json:"artists"`
}

func (h *Handler) createTrack(w http.ResponseWriter, r *http.Request) {
	var req createTrackRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	artists := make([]catalog.TrackArtist, 0, len(req.Artists))
	for _, a := range req.Artists {
		artists = append(artists, catalog.TrackArtist{ArtistID: a.ArtistID, Role: a.Role})
	}

	t, err := h.svc.CreateTrack(r.Context(), catalog.CreateTrackInput{
		Title: req.Title, AlbumID: req.AlbumID, DurationMs: req.DurationMs,
		TrackNumber: req.TrackNumber, ISRC: req.ISRC, Explicit: req.Explicit, Artists: artists,
	})
	if err != nil {
		writeCatalogError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, toTrackResponse(t, nil))
}

func (h *Handler) getTrack(w http.ResponseWriter, r *http.Request) {
	t, assets, err := h.svc.GetTrack(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeCatalogError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toTrackResponse(t, assets))
}

type searchResponse struct {
	Tracks     []trackResponse  `json:"tracks,omitempty"`
	Albums     []albumResponse  `json:"albums,omitempty"`
	Artists    []artistResponse `json:"artists,omitempty"`
	NextCursor string           `json:"next_cursor,omitempty"`
}

func (h *Handler) search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit := 0
	if raw := q.Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "limit must be a non-negative integer")
			return
		}
		limit = n
	}

	out, err := h.svc.Search(r.Context(), catalog.SearchInput{
		Query:  q.Get("q"),
		Type:   catalog.SearchType(q.Get("type")),
		Cursor: q.Get("cursor"),
		Limit:  limit,
	})
	if err != nil {
		writeCatalogError(w, err)
		return
	}

	resp := searchResponse{NextCursor: out.NextCursor}
	for _, t := range out.Tracks {
		resp.Tracks = append(resp.Tracks, toTrackResponse(t, nil))
	}
	for _, a := range out.Albums {
		resp.Albums = append(resp.Albums, albumResponse{ID: a.ID, Title: a.Title, PrimaryArtistID: a.PrimaryArtistID, AlbumType: a.AlbumType, CoverURL: a.CoverURL})
	}
	for _, a := range out.Artists {
		resp.Artists = append(resp.Artists, toArtistResponse(a))
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func writeCatalogError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, catalog.ErrInvalidInput):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_input", err.Error())
	case errors.Is(err, catalog.ErrTrackNotFound):
		httpx.WriteError(w, http.StatusNotFound, "not_found", "track not found")
	case errors.Is(err, catalog.ErrAlbumNotFound):
		httpx.WriteError(w, http.StatusNotFound, "not_found", "album not found")
	case errors.Is(err, catalog.ErrArtistNotFound):
		httpx.WriteError(w, http.StatusNotFound, "not_found", "artist not found")
	default:
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "internal error")
	}
}
