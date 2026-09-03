package ws

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
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
// every update/heartbeat (security.md §1.5: 90s). allowedOrigins is the same
// explicit CORS allowlist smusic-core's REST API uses
// (Config.CORSAllowedOrigins, from CORS_ALLOWED_ORIGINS) — required for a
// browser (Flutter Web) client to ever complete the WS handshake against
// this handler, since presence-server is a separate process/origin from the
// Web app by design (backend-go.md §1), so the library's own same-origin
// default (Upgrader.CheckOrigin unset) rejects every realistic Web
// deployment topology outright. See newOriginChecker's doc comment for the
// exact matching rules.
func NewHandler(hub *presence.Hub, nearby *presence.NearbyService, authr Authenticator, presenceTTL time.Duration, allowedOrigins []string, log *slog.Logger) *Handler {
	return &Handler{
		hub: hub, nearby: nearby, authr: authr, presenceTTL: presenceTTL, log: log,
		upgrader: websocket.Upgrader{
			CheckOrigin: newOriginChecker(allowedOrigins),
		},
	}
}

// newOriginChecker builds an Upgrader.CheckOrigin function that enforces the
// same explicit allowlist policy as the REST API's CORS middleware
// (cmd/server/main.go's buildRouter, via github.com/go-chi/cors), rather
// than gorilla/websocket's built-in default (reject unless Origin equals
// r.Host — same-origin only). Rules, in order:
//
//  1. No Origin header at all (native/mobile clients, and any other
//     non-browser WS client — browsers are the only user agent that sets
//     Origin on a WebSocket handshake) is always allowed: CORS is a
//     browser-enforced restriction, never a server-side one, exactly like
//     the REST CORS policy's own doc comment explains.
//  2. allowedOrigins configured (non-empty): the Origin header must appear
//     in it verbatim (case-insensitive on the whole origin string, since
//     scheme/host are case-insensitive per RFC 6454 and a stray-case port
//     is not a meaningful distinction) — no wildcard, no substring/suffix
//     matching, mirroring config.getCSVOrigins's explicit-allowlist,
//     never-"*" policy.
//  3. allowedOrigins NOT configured (unset/empty, the "CORS disabled"
//     default per config.CORSAllowedOrigins's doc comment): falls back to
//     gorilla/websocket's own same-origin default instead of rejecting
//     every browser client outright, so local/dev deployments that never
//     set CORS_ALLOWED_ORIGINS keep working exactly as before this fix for
//     the one topology that doesn't need it (WS server and Web app served
//     from literally the same origin).
func newOriginChecker(allowedOrigins []string) func(r *http.Request) bool {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, o := range allowedOrigins {
		allowed[strings.ToLower(o)] = struct{}{}
	}
	return func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true
		}
		if len(allowed) == 0 {
			u, err := url.Parse(origin)
			if err != nil {
				return false
			}
			return strings.EqualFold(u.Host, r.Host)
		}
		_, ok := allowed[strings.ToLower(origin)]
		return ok
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
