package ws

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"smusic/backend/internal/platform/httpx"
	"smusic/backend/internal/presence"
)

// Authenticator verifies a bearer access token — satisfied by
// *token.Signer, the same type internal/platform/middleware.RequireAuth
// uses for the REST API. WS connections can't always set a custom
// Authorization header (browser WebSocket clients cannot), so this handler
// also accepts the token via the "access_token" query parameter as a
// documented fallback, same as every other production WS API that needs
// bearer-token auth on the handshake.
type Authenticator interface {
	Authenticate(token string) (userID string, err error)
}

// Handler serves GET /v1/presence/connect (backend-go.md §4). It is the
// ONLY place in this codebase that (a) rejects a connection for missing
// consent (item 7 of the task) and (b) upgrades to a WebSocket — everything
// after that is delegated to presence.Hub/NearbyService.
type Handler struct {
	hub         *presence.Hub
	nearby      *presence.NearbyService
	authr       Authenticator
	presenceTTL time.Duration
	log         *slog.Logger
	upgrader    websocket.Upgrader
}

// NewHandler returns a Handler. presenceTTL is the Redis TTL applied on
// every update/heartbeat (security.md §1.5: 90s).
func NewHandler(hub *presence.Hub, nearby *presence.NearbyService, authr Authenticator, presenceTTL time.Duration, log *slog.Logger) *Handler {
	return &Handler{
		hub: hub, nearby: nearby, authr: authr, presenceTTL: presenceTTL, log: log,
		upgrader: websocket.Upgrader{
			// CheckOrigin left at the library default (reject cross-origin
			// unless Origin is absent, e.g. non-browser clients) is
			// deliberately NOT overridden with an always-true stub here —
			// unlike the REST API's explicit CORS allowlist
			// (cmd/server/main.go), this WS handler has no configured
			// origin allowlist wired in yet. TODO: wire the same
			// CORSAllowedOrigins config into a proper CheckOrigin here
			// before any browser (web) client is exercised against this
			// endpoint in a non-same-origin deployment.
		},
	}
}

// ServeHTTP implements http.Handler: authenticate -> check consent (item 7:
// reject connections from users without active, non-expired consent) ->
// upgrade -> register -> read loop -> unregister.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	token := bearerToken(r)
	if token == "" {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing access token")
		return
	}
	userID, err := h.authr.Authenticate(token)
	if err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "invalid or expired access token")
		return
	}

	if err := h.nearby.CheckConsent(r.Context(), userID); err != nil {
		switch {
		case errors.Is(err, presence.ErrConsentRequired):
			httpx.WriteError(w, http.StatusForbidden, "consent_required", "proximity discovery consent has not been granted")
		case errors.Is(err, presence.ErrConsentExpired):
			httpx.WriteError(w, http.StatusForbidden, "consent_expired", "proximity discovery consent has expired and must be renewed")
		default:
			httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "internal error")
		}
		return
	}

	wsConn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return // Upgrade already wrote an HTTP error response on failure
	}

	c := newConn(userID, wsConn, h.log)
	h.hub.Register(c)
	go c.writePump()

	h.readLoop(r.Context(), c)

	h.hub.Unregister(context.Background(), c)
	c.close()
}

func (h *Handler) readLoop(ctx context.Context, c *conn) {
	for {
		_, data, err := c.ws.ReadMessage()
		if err != nil {
			return
		}
		var f inboundFrame
		if err := json.Unmarshal(data, &f); err != nil {
			continue // malformed frame: ignored, connection stays open (backend-go.md §4 doesn't specify a hard-fail here)
		}
		h.handleInbound(ctx, c, f)
	}
}

func (h *Handler) handleInbound(ctx context.Context, c *conn, f inboundFrame) {
	switch f.Type {
	case TypeUpdate:
		if f.Lat == nil || f.Lon == nil {
			return
		}
		allowed, err := h.hub.AllowUpdateFrame(ctx, c.UserID())
		if err != nil || !allowed {
			return // layer-3 backpressure: silently drop, client's next update/heartbeat will be tried again
		}
		trackID := ""
		if f.NowPlaying != nil {
			trackID = f.NowPlaying.TrackID
		}
		_ = h.hub.EnqueueUpdate(c, *f.Lat, *f.Lon, trackID) // ErrIngestSaturated is layer-1 backpressure: dropped, not fatal to the connection
	case TypeHeartbeat:
		_ = h.hub.EnqueueHeartbeat(c)
	case TypeVisibility:
		_ = h.nearby.SetVisibility(ctx, c.UserID(), f.Mode)
	}
}

// bearerToken extracts the access token from either the Authorization
// header (native mobile clients) or the access_token query parameter
// (browser WebSocket clients, which cannot set custom headers on the
// handshake request) — see Authenticator's doc comment.
func bearerToken(r *http.Request) string {
	const prefix = "Bearer "
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, prefix) {
		return strings.TrimPrefix(h, prefix)
	}
	return r.URL.Query().Get("access_token")
}
