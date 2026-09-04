// Package api implements the REST transport for presence's "control
// plane" — settings, consent, pause and block management (item 7 of the
// task; backend-go.md §4's "REST complementar para consultas não
// realtime"). The real-time WebSocket feed (/v1/presence/connect) is
// intentionally NOT here: it's hosted by the separate presence-service
// process (backend-go.md §1) — see cmd/presence-server and
// internal/presence/ws. This package is mounted on smusic-core
// (cmd/server) instead, since settings/consent/blocks are low-frequency,
// Postgres-backed account configuration, not part of the
// latency/concurrency-sensitive presence data plane.
package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"smusic/backend/internal/platform/httpx"
	"smusic/backend/internal/platform/middleware"
	"smusic/backend/internal/presence"
)

// Service is the subset of *presence.SettingsService the handlers need.
type Service interface {
	Get(ctx context.Context, userID string) (presence.PrivacySettings, error)
	Update(ctx context.Context, userID string, in presence.UpdateSettingsInput) (presence.PrivacySettings, error)
	GrantConsent(ctx context.Context, userID string) (presence.PrivacySettings, error)
	RevokeConsent(ctx context.Context, userID string) (presence.PrivacySettings, error)
	SetPaused(ctx context.Context, userID string, paused bool) (presence.PrivacySettings, error)
	Block(ctx context.Context, blockerID, blockedID string) error
	Unblock(ctx context.Context, blockerID, blockedID string) error
}

// Handler holds presence's REST handlers.
type Handler struct {
	svc Service
}

// NewHandler returns a Handler backed by svc.
func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

// Mount registers presence's REST routes on r. Every route requires
// authentication — there is no anonymous access to presence settings.
func (h *Handler) Mount(r chi.Router, authr middleware.Authenticator) {
	r.Group(func(r chi.Router) {
		r.Use(middleware.RequireAuth(authr))
		r.Get("/v1/presence/settings", h.getSettings)
		r.Put("/v1/presence/settings", h.updateSettings)
		r.Post("/v1/presence/consent", h.grantConsent)
		r.Delete("/v1/presence/consent", h.revokeConsent)
		r.Post("/v1/presence/pause", h.pause)
		r.Post("/v1/presence/resume", h.resume)
		r.Post("/v1/presence/blocks/{user_id}", h.block)
		r.Delete("/v1/presence/blocks/{user_id}", h.unblock)
	})
}

type settingsResponse struct {
	PresenceVisibility       string  `json:"presence_visibility"`
	PresenceShareTrack       bool    `json:"presence_share_track"`
	ProximityConsentEnabled  bool    `json:"proximity_consent_enabled"`
	ProximityConsentTS       *string `json:"proximity_consent_ts,omitempty"`
	ProximityConsentRenewDue *string `json:"proximity_consent_renew_due,omitempty"`
	VisibilityRadiusM        int     `json:"visibility_radius_m"`
	RevealLevel              int     `json:"reveal_level"`
	Paused                   bool    `json:"paused"`
}

func toSettingsResponse(s presence.PrivacySettings) settingsResponse {
	resp := settingsResponse{
		PresenceVisibility:      s.PresenceVisibility,
		PresenceShareTrack:      s.PresenceShareTrack,
		ProximityConsentEnabled: s.ProximityConsentEnabled,
		VisibilityRadiusM:       s.VisibilityRadiusM,
		RevealLevel:             s.RevealLevel,
		Paused:                  s.Paused,
	}
	if s.ProximityConsentTS != nil {
		v := s.ProximityConsentTS.Format("2006-01-02T15:04:05Z07:00")
		resp.ProximityConsentTS = &v
	}
	if s.ProximityConsentRenewDue != nil {
		v := s.ProximityConsentRenewDue.Format("2006-01-02T15:04:05Z07:00")
		resp.ProximityConsentRenewDue = &v
	}
	return resp
}

func (h *Handler) getSettings(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserID(r.Context())
	s, err := h.svc.Get(r.Context(), userID)
	if err != nil {
		writeError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toSettingsResponse(s))
}

type updateSettingsRequest struct {
	PresenceVisibility *string `json:"presence_visibility,omitempty"`
	PresenceShareTrack *bool   `json:"presence_share_track,omitempty"`
	VisibilityRadiusM  *int    `json:"visibility_radius_m,omitempty"`
	RevealLevel        *int    `json:"reveal_level,omitempty"`
	Paused             *bool   `json:"paused,omitempty"`
}

func (h *Handler) updateSettings(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserID(r.Context())
	var req updateSettingsRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	s, err := h.svc.Update(r.Context(), userID, presence.UpdateSettingsInput{
		PresenceVisibility: req.PresenceVisibility,
		PresenceShareTrack: req.PresenceShareTrack,
		VisibilityRadiusM:  req.VisibilityRadiusM,
		RevealLevel:        req.RevealLevel,
		Paused:             req.Paused,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toSettingsResponse(s))
}

func (h *Handler) grantConsent(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserID(r.Context())
	s, err := h.svc.GrantConsent(r.Context(), userID)
	if err != nil {
		writeError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toSettingsResponse(s))
}

func (h *Handler) revokeConsent(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserID(r.Context())
	s, err := h.svc.RevokeConsent(r.Context(), userID)
	if err != nil {
		writeError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toSettingsResponse(s))
}

func (h *Handler) pause(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserID(r.Context())
	s, err := h.svc.SetPaused(r.Context(), userID, true)
	if err != nil {
		writeError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toSettingsResponse(s))
}

func (h *Handler) resume(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserID(r.Context())
	s, err := h.svc.SetPaused(r.Context(), userID, false)
	if err != nil {
		writeError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toSettingsResponse(s))
}

func (h *Handler) block(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserID(r.Context())
	target := chi.URLParam(r, "user_id")
	if err := h.svc.Block(r.Context(), userID, target); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) unblock(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserID(r.Context())
	target := chi.URLParam(r, "user_id")
	if err := h.svc.Unblock(r.Context(), userID, target); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, presence.ErrInvalidInput),
		errors.Is(err, presence.ErrInvalidRadius),
		errors.Is(err, presence.ErrInvalidRevealLevel),
		errors.Is(err, presence.ErrInvalidVisibility):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_input", err.Error())
	case errors.Is(err, presence.ErrCannotBlockSelf):
		httpx.WriteError(w, http.StatusBadRequest, "cannot_block_self", err.Error())
	case errors.Is(err, presence.ErrMFARequired):
		httpx.WriteError(w, http.StatusForbidden, "mfa_required", err.Error())
	case errors.Is(err, presence.ErrSettingsNotFound):
		httpx.WriteError(w, http.StatusNotFound, "not_found", err.Error())
	default:
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "internal error")
	}
}
