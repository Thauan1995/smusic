// Package api implements the HTTP transport for the auth module
// (backend-go.md §4's Autenticação contracts). Handlers are intentionally
// thin: decode -> call Service -> map result/error to a response
// (backend-go.md §7) — every branch of actual business logic lives in
// auth.Service and is tested there.
package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"smusic/backend/internal/auth"
	"smusic/backend/internal/auth/oauth"
	"smusic/backend/internal/platform/httpx"
	"smusic/backend/internal/platform/middleware"
)

// Service is the subset of *auth.Service the handlers need, as an
// interface so handler tests can inject a fake and stay transport-focused
// (backend-go.md §7).
type Service interface {
	SignUp(ctx context.Context, in auth.SignUpInput) (auth.AuthResult, error)
	Login(ctx context.Context, in auth.LoginInput) (auth.AuthResult, error)
	LoginWithOAuth(ctx context.Context, provider oauth.Provider, idToken, displayName string, device *auth.DeviceInput) (auth.AuthResult, error)
	Refresh(ctx context.Context, refreshToken string) (auth.AuthResult, error)
	Logout(ctx context.Context, refreshToken string) error
	LogoutAll(ctx context.Context, userID string) error
	Me(ctx context.Context, userID string) (auth.User, error)
}

// Handler holds the auth module's HTTP handlers.
type Handler struct {
	svc Service
}

// NewHandler returns a Handler backed by svc.
func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

// Mount registers auth's routes on r. authr protects the routes that
// require an authenticated caller (me, logout-all). loginRateLimit, if
// non-nil, is applied to signup/login only — security.md §4: "Auth
// (login/signup): limite agressivo por IP (mitigação de brute-force)". A
// nil loginRateLimit (e.g. in handler-only unit tests) mounts the routes
// with no extra middleware.
func (h *Handler) Mount(r chi.Router, authr middleware.Authenticator, loginRateLimit func(http.Handler) http.Handler) {
	r.Group(func(r chi.Router) {
		if loginRateLimit != nil {
			r.Use(loginRateLimit)
		}
		r.Post("/v1/auth/signup", h.signUp)
		r.Post("/v1/auth/login", h.login)
	})
	r.Post("/v1/auth/refresh", h.refresh)
	r.Post("/v1/auth/logout", h.logout)

	r.Group(func(r chi.Router) {
		r.Use(middleware.RequireAuth(authr))
		r.Get("/v1/auth/me", h.me)
		r.Post("/v1/auth/logout-all", h.logoutAll)
	})
}

type deviceRequest struct {
	Platform   string `json:"platform,omitempty"`
	PushToken  string `json:"push_token,omitempty"`
	AppVersion string `json:"app_version,omitempty"`
}

func (d *deviceRequest) toInput() *auth.DeviceInput {
	if d == nil || d.Platform == "" {
		return nil
	}
	return &auth.DeviceInput{Platform: d.Platform, PushToken: d.PushToken, AppVersion: d.AppVersion}
}

type credentialsRequest struct {
	Email         string         `json:"email,omitempty"`
	Password      string         `json:"password,omitempty"`
	DisplayName   string         `json:"display_name,omitempty"`
	OAuthProvider string         `json:"oauth_provider,omitempty"`
	OAuthToken    string         `json:"oauth_token,omitempty"`
	Device        *deviceRequest `json:"device,omitempty"`
}

type authResponse struct {
	UserID                string    `json:"user_id"`
	AccessToken           string    `json:"access_token"`
	AccessTokenExpiresAt  time.Time `json:"access_token_expires_at"`
	RefreshToken          string    `json:"refresh_token"`
	RefreshTokenExpiresAt time.Time `json:"refresh_token_expires_at"`
}

func toAuthResponse(r auth.AuthResult) authResponse {
	return authResponse{
		UserID:                r.UserID,
		AccessToken:           r.AccessToken,
		AccessTokenExpiresAt:  r.AccessTokenExpiresAt,
		RefreshToken:          r.RefreshToken,
		RefreshTokenExpiresAt: r.RefreshTokenExpiresAt,
	}
}

func (h *Handler) signUp(w http.ResponseWriter, r *http.Request) {
	var req credentialsRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	if req.OAuthToken != "" {
		h.oauthLogin(w, r, req)
		return
	}

	result, err := h.svc.SignUp(r.Context(), auth.SignUpInput{
		Email:       req.Email,
		Password:    req.Password,
		DisplayName: req.DisplayName,
		Device:      req.Device.toInput(),
	})
	if err != nil {
		writeAuthError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, toAuthResponse(result))
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var req credentialsRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	if req.OAuthToken != "" {
		h.oauthLogin(w, r, req)
		return
	}

	result, err := h.svc.Login(r.Context(), auth.LoginInput{
		Email:    req.Email,
		Password: req.Password,
		Device:   req.Device.toInput(),
	})
	if err != nil {
		writeAuthError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toAuthResponse(result))
}

func (h *Handler) oauthLogin(w http.ResponseWriter, r *http.Request, req credentialsRequest) {
	if req.OAuthProvider == "" {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "oauth_provider is required with oauth_token")
		return
	}
	result, err := h.svc.LoginWithOAuth(r.Context(), oauth.Provider(req.OAuthProvider), req.OAuthToken, req.DisplayName, req.Device.toInput())
	if err != nil {
		writeAuthError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toAuthResponse(result))
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

func (h *Handler) refresh(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	result, err := h.svc.Refresh(r.Context(), req.RefreshToken)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toAuthResponse(result))
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	if err := h.svc.Logout(r.Context(), req.RefreshToken); err != nil {
		writeAuthError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) logoutAll(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserID(r.Context())
	if err := h.svc.LogoutAll(r.Context(), userID); err != nil {
		writeAuthError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type meResponse struct {
	UserID      string `json:"user_id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	Handle      string `json:"handle,omitempty"`
	Status      string `json:"status"`
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserID(r.Context())
	user, err := h.svc.Me(r.Context(), userID)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, meResponse{
		UserID:      user.ID,
		Email:       user.Email,
		DisplayName: user.DisplayName,
		Handle:      user.Handle,
		Status:      user.Status,
	})
}

// writeAuthError maps auth's sentinel errors to HTTP status codes. This is
// the one place that translation happens, per backend-go.md §7 keeping
// domain code transport-agnostic.
func writeAuthError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, auth.ErrInvalidInput):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_input", err.Error())
	case errors.Is(err, auth.ErrEmailTaken):
		httpx.WriteError(w, http.StatusConflict, "email_taken", "email already registered")
	case errors.Is(err, auth.ErrInvalidCredentials):
		httpx.WriteError(w, http.StatusUnauthorized, "invalid_credentials", "invalid email or password")
	case errors.Is(err, auth.ErrUserNotFound):
		httpx.WriteError(w, http.StatusNotFound, "not_found", "user not found")
	case errors.Is(err, auth.ErrInvalidRefreshToken):
		httpx.WriteError(w, http.StatusUnauthorized, "invalid_refresh_token", "invalid refresh token")
	case errors.Is(err, auth.ErrRefreshTokenExpired):
		httpx.WriteError(w, http.StatusUnauthorized, "refresh_token_expired", "refresh token expired")
	case errors.Is(err, auth.ErrRefreshTokenRevoked):
		httpx.WriteError(w, http.StatusUnauthorized, "refresh_token_revoked", "refresh token revoked")
	case errors.Is(err, auth.ErrRefreshTokenReused):
		httpx.WriteError(w, http.StatusUnauthorized, "refresh_token_reused", "refresh token reuse detected; all sessions revoked")
	case errors.Is(err, oauth.ErrNotImplemented):
		httpx.WriteError(w, http.StatusNotImplemented, "not_implemented", err.Error())
	case errors.Is(err, oauth.ErrUnsupportedProvider):
		httpx.WriteError(w, http.StatusBadRequest, "unsupported_provider", err.Error())
	default:
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "internal error")
	}
}
