// Package api implements the HTTP transport for the library module
// (backend-go.md §4's Biblioteca contracts, extended with a
// list-playlist-tracks and an unsave-track endpoint the source doc's
// sketch omitted but the task explicitly asked for: "adicionar/remover
// faixa, listar"). Every route requires authentication; the requester's ID
// comes from the auth middleware, never from the request body — a user can
// only ever act as themselves.
package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"smusic/backend/internal/library"
	"smusic/backend/internal/platform/httpx"
	"smusic/backend/internal/platform/middleware"
)

// Service is the subset of *library.Service the handlers need.
type Service interface {
	CreatePlaylist(ctx context.Context, in library.CreatePlaylistInput) (library.Playlist, error)
	ListPlaylists(ctx context.Context, ownerID string) ([]library.Playlist, error)
	ListPlaylistTracks(ctx context.Context, playlistID, requesterID string) (library.Playlist, []library.PlaylistTrack, error)
	AddTrack(ctx context.Context, playlistID, requesterID, trackID, position string) (library.PlaylistTrack, error)
	RemoveTrack(ctx context.Context, playlistID, requesterID, trackID string) error
	SaveTrack(ctx context.Context, userID, trackID string) error
	UnsaveTrack(ctx context.Context, userID, trackID string) error
	ListSavedTracks(ctx context.Context, userID, cursor string, limit int) ([]library.LibraryTrack, string, error)
}

// Handler holds the library module's HTTP handlers.
type Handler struct {
	svc Service
}

// NewHandler returns a Handler backed by svc.
func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

// Mount registers library's routes on r, all behind authr.
func (h *Handler) Mount(r chi.Router, authr middleware.Authenticator) {
	r.Group(func(r chi.Router) {
		r.Use(middleware.RequireAuth(authr))

		r.Get("/v1/library/me/playlists", h.listPlaylists)
		r.Post("/v1/library/me/playlists", h.createPlaylist)
		r.Get("/v1/library/me/playlists/{id}/tracks", h.listPlaylistTracks)
		r.Post("/v1/library/me/playlists/{id}/tracks", h.addTrack)
		r.Delete("/v1/library/me/playlists/{id}/tracks/{track_id}", h.removeTrack)

		r.Get("/v1/library/me/saved-tracks", h.listSavedTracks)
		r.Post("/v1/library/me/saved-tracks", h.saveTrack)
		r.Delete("/v1/library/me/saved-tracks/{track_id}", h.unsaveTrack)
	})
}

type playlistResponse struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Visibility  string `json:"visibility"`
	CoverURL    string `json:"cover_url,omitempty"`
}

func toPlaylistResponse(p library.Playlist) playlistResponse {
	return playlistResponse{ID: p.ID, Title: p.Title, Description: p.Description, Visibility: p.Visibility, CoverURL: p.CoverURL}
}

type createPlaylistRequest struct {
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Visibility  string `json:"visibility,omitempty"`
}

func (h *Handler) createPlaylist(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserID(r.Context())
	var req createPlaylistRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	p, err := h.svc.CreatePlaylist(r.Context(), library.CreatePlaylistInput{
		OwnerID: userID, Title: req.Title, Description: req.Description, Visibility: req.Visibility,
	})
	if err != nil {
		writeLibraryError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, toPlaylistResponse(p))
}

type playlistsResponse struct {
	Playlists []playlistResponse `json:"playlists"`
}

func (h *Handler) listPlaylists(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserID(r.Context())
	playlists, err := h.svc.ListPlaylists(r.Context(), userID)
	if err != nil {
		writeLibraryError(w, err)
		return
	}
	resp := playlistsResponse{Playlists: make([]playlistResponse, 0, len(playlists))}
	for _, p := range playlists {
		resp.Playlists = append(resp.Playlists, toPlaylistResponse(p))
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

type playlistTrackResponse struct {
	TrackID string    `json:"track_id"`
	AddedBy string    `json:"added_by,omitempty"`
	AddedAt time.Time `json:"added_at"`
}

type playlistTracksResponse struct {
	Playlist playlistResponse        `json:"playlist"`
	Tracks   []playlistTrackResponse `json:"tracks"`
}

func (h *Handler) listPlaylistTracks(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserID(r.Context())
	playlist, tracks, err := h.svc.ListPlaylistTracks(r.Context(), chi.URLParam(r, "id"), userID)
	if err != nil {
		writeLibraryError(w, err)
		return
	}
	resp := playlistTracksResponse{Playlist: toPlaylistResponse(playlist), Tracks: make([]playlistTrackResponse, 0, len(tracks))}
	for _, t := range tracks {
		resp.Tracks = append(resp.Tracks, playlistTrackResponse{TrackID: t.TrackID, AddedBy: t.AddedBy, AddedAt: t.AddedAt})
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

type addTrackRequest struct {
	TrackID  string `json:"track_id"`
	Position string `json:"position,omitempty"`
}

func (h *Handler) addTrack(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserID(r.Context())
	var req addTrackRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if _, err := h.svc.AddTrack(r.Context(), chi.URLParam(r, "id"), userID, req.TrackID, req.Position); err != nil {
		writeLibraryError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) removeTrack(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserID(r.Context())
	if err := h.svc.RemoveTrack(r.Context(), chi.URLParam(r, "id"), userID, chi.URLParam(r, "track_id")); err != nil {
		writeLibraryError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type saveTrackRequest struct {
	TrackID string `json:"track_id"`
}

func (h *Handler) saveTrack(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserID(r.Context())
	var req saveTrackRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if err := h.svc.SaveTrack(r.Context(), userID, req.TrackID); err != nil {
		writeLibraryError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) unsaveTrack(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserID(r.Context())
	if err := h.svc.UnsaveTrack(r.Context(), userID, chi.URLParam(r, "track_id")); err != nil {
		writeLibraryError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type savedTrackResponse struct {
	TrackID string    `json:"track_id"`
	AddedAt time.Time `json:"added_at"`
}

type savedTracksResponse struct {
	Tracks     []savedTrackResponse `json:"tracks"`
	NextCursor string               `json:"next_cursor,omitempty"`
}

func (h *Handler) listSavedTracks(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserID(r.Context())
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

	items, next, err := h.svc.ListSavedTracks(r.Context(), userID, q.Get("cursor"), limit)
	if err != nil {
		writeLibraryError(w, err)
		return
	}
	resp := savedTracksResponse{Tracks: make([]savedTrackResponse, 0, len(items)), NextCursor: next}
	for _, it := range items {
		resp.Tracks = append(resp.Tracks, savedTrackResponse{TrackID: it.TrackID, AddedAt: it.AddedAt})
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func writeLibraryError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, library.ErrInvalidInput):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_input", err.Error())
	case errors.Is(err, library.ErrPlaylistNotFound):
		httpx.WriteError(w, http.StatusNotFound, "not_found", "playlist not found")
	case errors.Is(err, library.ErrTrackNotFound):
		httpx.WriteError(w, http.StatusNotFound, "not_found", "track not found")
	case errors.Is(err, library.ErrTrackNotInPlaylist):
		httpx.WriteError(w, http.StatusNotFound, "not_found", "track not in playlist")
	case errors.Is(err, library.ErrNotOwner):
		httpx.WriteError(w, http.StatusForbidden, "forbidden", "you do not have access to this playlist")
	default:
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "internal error")
	}
}
