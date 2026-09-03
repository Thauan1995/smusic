// Package api implements the HTTP transport for the playback module
// (backend-go.md §4's Reprodução contracts). One deviation from the
// source doc's sketch: POST /v1/playback/sessions returns just
// {session_id} rather than {session_id, playback_url_manifest} — a
// manifest (HLS variants) belongs to the real media-edge-service/CDN
// pipeline (backend-go.md §2), which is explicitly out of scope for this
// slice (see internal/playback/media's TODO); the client fetches a
// stream_url per track via .../play instead.
package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"smusic/backend/internal/platform/httpx"
	"smusic/backend/internal/platform/middleware"
	"smusic/backend/internal/playback"
)

// Service is the subset of *playback.Service the handlers need.
type Service interface {
	CreateSession(ctx context.Context, userID, deviceID string) (playback.SessionState, error)
	GetState(ctx context.Context, sessionID, requesterID string) (playback.SessionState, error)
	Play(ctx context.Context, sessionID, requesterID, trackID string, positionMs int) (playback.PlayResult, error)
	Pause(ctx context.Context, sessionID, requesterID string) (playback.SessionState, error)
	Seek(ctx context.Context, sessionID, requesterID string, positionMs int) (playback.SessionState, error)
	Next(ctx context.Context, sessionID, requesterID string) (playback.PlayResult, error)
	Enqueue(ctx context.Context, sessionID, requesterID string, trackIDs []string, position string) (playback.SessionState, error)
}

// Handler holds the playback module's HTTP handlers.
type Handler struct {
	svc Service
}

// NewHandler returns a Handler backed by svc.
func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

// Mount registers playback's routes on r, all behind authr.
func (h *Handler) Mount(r chi.Router, authr middleware.Authenticator) {
	r.Group(func(r chi.Router) {
		r.Use(middleware.RequireAuth(authr))

		r.Post("/v1/playback/sessions", h.createSession)
		r.Get("/v1/playback/sessions/{id}/state", h.getState)
		r.Post("/v1/playback/sessions/{id}/play", h.play)
		r.Post("/v1/playback/sessions/{id}/pause", h.pause)
		r.Post("/v1/playback/sessions/{id}/seek", h.seek)
		r.Post("/v1/playback/sessions/{id}/next", h.next)
		r.Post("/v1/playback/sessions/{id}/queue", h.enqueue)
	})
}

type createSessionRequest struct {
	DeviceID string `json:"device_id,omitempty"`
}

type createSessionResponse struct {
	SessionID string `json:"session_id"`
}

func (h *Handler) createSession(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserID(r.Context())
	var req createSessionRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	s, err := h.svc.CreateSession(r.Context(), userID, req.DeviceID)
	if err != nil {
		writePlaybackError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, createSessionResponse{SessionID: s.SessionID})
}

type stateResponse struct {
	TrackID    string   `json:"track_id,omitempty"`
	PositionMs int      `json:"position_ms"`
	IsPlaying  bool     `json:"is_playing"`
	Queue      []string `json:"queue"`
}

func toStateResponse(s playback.SessionState) stateResponse {
	queue := s.Queue
	if queue == nil {
		queue = []string{}
	}
	return stateResponse{TrackID: s.TrackID, PositionMs: s.PositionMs, IsPlaying: s.IsPlaying, Queue: queue}
}

func (h *Handler) getState(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserID(r.Context())
	s, err := h.svc.GetState(r.Context(), chi.URLParam(r, "id"), userID)
	if err != nil {
		writePlaybackError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toStateResponse(s))
}

type playRequest struct {
	TrackID    string `json:"track_id"`
	PositionMs int    `json:"position_ms,omitempty"`
}

type playResponse struct {
	StreamURL string `json:"stream_url"`
	ExpiresAt string `json:"expires_at"`
}

func (h *Handler) play(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserID(r.Context())
	var req playRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	result, err := h.svc.Play(r.Context(), chi.URLParam(r, "id"), userID, req.TrackID, req.PositionMs)
	if err != nil {
		writePlaybackError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, playResponse{StreamURL: result.StreamURL, ExpiresAt: result.ExpiresAt.Format(timeLayout)})
}

const timeLayout = "2006-01-02T15:04:05.999999999Z07:00"

func (h *Handler) pause(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserID(r.Context())
	if _, err := h.svc.Pause(r.Context(), chi.URLParam(r, "id"), userID); err != nil {
		writePlaybackError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type seekRequest struct {
	PositionMs int `json:"position_ms"`
}

func (h *Handler) seek(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserID(r.Context())
	var req seekRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if _, err := h.svc.Seek(r.Context(), chi.URLParam(r, "id"), userID, req.PositionMs); err != nil {
		writePlaybackError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type nextResponse struct {
	TrackID   string `json:"track_id"`
	StreamURL string `json:"stream_url"`
	ExpiresAt string `json:"expires_at"`
}

func (h *Handler) next(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserID(r.Context())
	result, err := h.svc.Next(r.Context(), chi.URLParam(r, "id"), userID)
	if err != nil {
		writePlaybackError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, nextResponse{TrackID: result.TrackID, StreamURL: result.StreamURL, ExpiresAt: result.ExpiresAt.Format(timeLayout)})
}

type enqueueRequest struct {
	TrackIDs []string `json:"track_ids"`
	Position string   `json:"position,omitempty"`
}

func (h *Handler) enqueue(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserID(r.Context())
	var req enqueueRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if _, err := h.svc.Enqueue(r.Context(), chi.URLParam(r, "id"), userID, req.TrackIDs, req.Position); err != nil {
		writePlaybackError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writePlaybackError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, playback.ErrInvalidInput):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_input", err.Error())
	case errors.Is(err, playback.ErrSessionNotFound):
		httpx.WriteError(w, http.StatusNotFound, "not_found", "session not found")
	case errors.Is(err, playback.ErrTrackNotFound):
		httpx.WriteError(w, http.StatusNotFound, "not_found", "track not found")
	case errors.Is(err, playback.ErrForbidden):
		httpx.WriteError(w, http.StatusForbidden, "forbidden", "you do not have access to this session")
	case errors.Is(err, playback.ErrEmptyQueue):
		httpx.WriteError(w, http.StatusConflict, "empty_queue", "queue is empty")
	default:
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "internal error")
	}
}
