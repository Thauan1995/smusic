package ws

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"smusic/backend/internal/platform/clock"
	"smusic/backend/internal/platform/idgen"
	"smusic/backend/internal/presence"
)

type fakeAuthenticator struct {
	userID string
	err    error
}

func (f fakeAuthenticator) Authenticate(token string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.userID, nil
}

// newHandlerTestServer builds a real NearbyService (in-memory fakes) and
// Hub, exactly like internal/presence's own tests, so this package's tests
// exercise the full stack down to (fake) privacy enforcement -- only the
// network layer (this package) is what's actually under test here.
func newHandlerTestServer(t *testing.T, authr Authenticator, consentGranted bool) (*httptest.Server, func()) {
	t.Helper()

	settings := newFakeSettingsRepo()
	blocks := newFakeBlockRepoWS()
	follows := newFakeFollowCheckerWS()
	geo := newFakeGeoIndexWS()
	audit := newFakeAuditRepoWS()
	profiles := newFakeProfileResolverWS()
	rl := newFakeRateLimiterWS()
	clk := clock.NewFrozen(time.Now())

	if consentGranted {
		now := clk.Now()
		due := now.Add(presence.ConsentValidityPeriod)
		settings.rows["u1"] = presence.PrivacySettings{
			UserID: "u1", PresenceVisibility: presence.VisibilityEveryone, VisibilityRadiusM: presence.DefaultRadiusM,
			ProximityConsentEnabled: true, ProximityConsentTS: &now, ProximityConsentRenewDue: &due,
		}
	}

	nearby := presence.NewNearbyService(settings, blocks, follows, geo, audit, profiles, rl, rl, presence.FixedJitterer{}, clk, idgen.NewSequential("id"))
	hub := presence.NewHub(nearby, time.Minute, 2, 16, nil)
	ctx, cancel := context.WithCancel(context.Background())
	go hub.Run(ctx)

	h := NewHandler(hub, nearby, authr, time.Minute, nil, testLogger())
	srv := httptest.NewServer(h)
	cleanup := func() {
		cancel()
		srv.Close()
	}
	return srv, cleanup
}

// newHandlerTestServerWithOrigins is newHandlerTestServer plus an explicit
// allowedOrigins allowlist wired into NewHandler, for exercising
// Upgrader.CheckOrigin end to end (Bug 1: CheckOrigin previously left at the
// gorilla/websocket same-origin default, which rejects every realistic Web
// deployment topology since presence-server is a separate origin from the
// Web app by design).
func newHandlerTestServerWithOrigins(t *testing.T, allowedOrigins []string) (*httptest.Server, func()) {
	t.Helper()

	settings := newFakeSettingsRepo()
	blocks := newFakeBlockRepoWS()
	follows := newFakeFollowCheckerWS()
	geo := newFakeGeoIndexWS()
	audit := newFakeAuditRepoWS()
	profiles := newFakeProfileResolverWS()
	rl := newFakeRateLimiterWS()
	clk := clock.NewFrozen(time.Now())

	now := clk.Now()
	due := now.Add(presence.ConsentValidityPeriod)
	settings.rows["u1"] = presence.PrivacySettings{
		UserID: "u1", PresenceVisibility: presence.VisibilityEveryone, VisibilityRadiusM: presence.DefaultRadiusM,
		ProximityConsentEnabled: true, ProximityConsentTS: &now, ProximityConsentRenewDue: &due,
	}

	nearby := presence.NewNearbyService(settings, blocks, follows, geo, audit, profiles, rl, rl, presence.FixedJitterer{}, clk, idgen.NewSequential("id"))
	hub := presence.NewHub(nearby, time.Minute, 2, 16, nil)
	ctx, cancel := context.WithCancel(context.Background())
	go hub.Run(ctx)

	h := NewHandler(hub, nearby, fakeAuthenticator{userID: "u1"}, time.Minute, allowedOrigins, testLogger())
	srv := httptest.NewServer(h)
	cleanup := func() {
		cancel()
		srv.Close()
	}
	return srv, cleanup
}

// TestHandler_CheckOrigin_AllowedOrigin_Upgrades is the Bug 1 fix's core
// assertion: a handshake whose Origin header is on the configured allowlist
// (simulating a Web client on its own origin, connecting to presence-server
// as a separate process/origin per backend-go.md §1) must succeed with 101,
// not be rejected by the library's same-origin default.
func TestHandler_CheckOrigin_AllowedOrigin_Upgrades(t *testing.T) {
	srv, cleanup := newHandlerTestServerWithOrigins(t, []string{"http://allowed.example"})
	defer cleanup()

	wsURL := "ws" + srv.URL[len("http"):] + "?access_token=whatever"
	header := http.Header{}
	header.Set("Origin", "http://allowed.example")
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, header)
	require.NoError(t, err)
	defer conn.Close()
	require.NotNil(t, resp)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusSwitchingProtocols, resp.StatusCode)
}

// TestHandler_CheckOrigin_DisallowedOrigin_Rejected asserts the allowlist
// stays a real allowlist post-fix: an Origin NOT on the list must still get
// 403, never silently become wide open.
func TestHandler_CheckOrigin_DisallowedOrigin_Rejected(t *testing.T) {
	srv, cleanup := newHandlerTestServerWithOrigins(t, []string{"http://allowed.example"})
	defer cleanup()

	wsURL := "ws" + srv.URL[len("http"):] + "?access_token=whatever"
	header := http.Header{}
	header.Set("Origin", "http://evil.example")
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, header)
	require.Error(t, err)
	if conn != nil {
		defer conn.Close()
	}
	require.NotNil(t, resp)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

// TestHandler_CheckOrigin_NoOriginHeader_StillUpgrades asserts native
// clients (no Origin header at all — only browsers set it on a WS
// handshake) keep working even with a non-empty allowedOrigins configured.
func TestHandler_CheckOrigin_NoOriginHeader_StillUpgrades(t *testing.T) {
	srv, cleanup := newHandlerTestServerWithOrigins(t, []string{"http://allowed.example"})
	defer cleanup()

	wsURL := "ws" + srv.URL[len("http"):] + "?access_token=whatever"
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer conn.Close()
	require.NotNil(t, resp)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusSwitchingProtocols, resp.StatusCode)
}

// TestNewOriginChecker unit-tests newOriginChecker directly (all of its
// branches, including the "allowedOrigins unset" same-origin fallback,
// which the end-to-end tests above don't exercise since they always
// configure a non-empty allowlist).
func TestNewOriginChecker(t *testing.T) {
	tests := []struct {
		name    string
		allowed []string
		origin  string
		host    string
		want    bool
	}{
		{"no Origin header is always allowed", []string{"https://allowed.example"}, "", "presence.example", true},
		{"origin on the allowlist matches", []string{"https://allowed.example"}, "https://allowed.example", "presence.example", true},
		{"match is case-insensitive", []string{"HTTPS://ALLOWED.EXAMPLE"}, "https://allowed.example", "presence.example", true},
		{"origin off the allowlist is rejected", []string{"https://allowed.example"}, "https://evil.example", "presence.example", false},
		{"origin is a substring but not an exact match: rejected", []string{"https://allowed.example"}, "https://not-allowed.example", "presence.example", false},
		{"empty allowlist falls back to same-origin: matching host allowed", nil, "http://presence.example", "presence.example", true},
		{"empty allowlist falls back to same-origin: different host rejected", nil, "http://evil.example", "presence.example", false},
		{"empty allowlist, malformed Origin header rejected", nil, "http://%zz", "presence.example", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			check := newOriginChecker(tt.allowed)
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.Host = tt.host
			if tt.origin != "" {
				r.Header.Set("Origin", tt.origin)
			}
			assert.Equal(t, tt.want, check(r))
		})
	}
}

func TestHandler_MissingToken_Unauthorized(t *testing.T) {
	srv, cleanup := newHandlerTestServer(t, fakeAuthenticator{userID: "u1"}, true)
	defer cleanup()

	resp, err := http.Get(srv.URL)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestHandler_InvalidToken_Unauthorized(t *testing.T) {
	srv, cleanup := newHandlerTestServer(t, fakeAuthenticator{err: assertErrWS("bad token")}, true)
	defer cleanup()

	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	req.Header.Set("Authorization", "Bearer whatever")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

// TestHandler_NoConsent_Forbidden is the item-7 invariant: the backend
// must reject WS connections from users without active, valid consent.
func TestHandler_NoConsent_Forbidden(t *testing.T) {
	srv, cleanup := newHandlerTestServer(t, fakeAuthenticator{userID: "u1"}, false)
	defer cleanup()

	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	req.Header.Set("Authorization", "Bearer whatever")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

// TestHandler_ConsentExpired_Forbidden exercises ServeHTTP's
// ErrConsentExpired branch specifically (distinct from
// TestHandler_NoConsent_Forbidden's ErrConsentRequired, "never granted"
// case): consent was granted once but its renewal is overdue.
func TestHandler_ConsentExpired_Forbidden(t *testing.T) {
	settings := newFakeSettingsRepo()
	clk := clock.NewFrozen(time.Now())
	past := clk.Now().Add(-time.Hour)
	settings.rows["u1"] = presence.PrivacySettings{
		UserID: "u1", PresenceVisibility: presence.VisibilityEveryone, VisibilityRadiusM: presence.DefaultRadiusM,
		ProximityConsentEnabled: true, ProximityConsentRenewDue: &past,
	}
	blocks := newFakeBlockRepoWS()
	follows := newFakeFollowCheckerWS()
	geo := newFakeGeoIndexWS()
	audit := newFakeAuditRepoWS()
	profiles := newFakeProfileResolverWS()
	rl := newFakeRateLimiterWS()

	nearby := presence.NewNearbyService(settings, blocks, follows, geo, audit, profiles, rl, rl, presence.FixedJitterer{}, clk, idgen.NewSequential("id"))
	hub := presence.NewHub(nearby, time.Minute, 1, 8, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	h := NewHandler(hub, nearby, fakeAuthenticator{userID: "u1"}, time.Minute, nil, testLogger())
	srv := httptest.NewServer(h)
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	req.Header.Set("Authorization", "Bearer whatever")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

// TestHandler_CheckConsent_InternalError exercises ServeHTTP's default
// (internal_error) branch: a settings-repository failure that isn't a
// consent sentinel must map to 500, not be misreported as a consent issue.
func TestHandler_CheckConsent_InternalError(t *testing.T) {
	settings := newFakeSettingsRepo()
	settings.getErr = assertErrWS("boom")
	blocks := newFakeBlockRepoWS()
	follows := newFakeFollowCheckerWS()
	geo := newFakeGeoIndexWS()
	audit := newFakeAuditRepoWS()
	profiles := newFakeProfileResolverWS()
	rl := newFakeRateLimiterWS()

	nearby := presence.NewNearbyService(settings, blocks, follows, geo, audit, profiles, rl, rl, presence.FixedJitterer{}, clock.NewFrozen(time.Now()), idgen.NewSequential("id"))
	hub := presence.NewHub(nearby, time.Minute, 1, 8, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	h := NewHandler(hub, nearby, fakeAuthenticator{userID: "u1"}, time.Minute, nil, testLogger())
	srv := httptest.NewServer(h)
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	req.Header.Set("Authorization", "Bearer whatever")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

// TestHandler_UpgradeFailure_NotAWebSocketRequest exercises ServeHTTP's
// Upgrade-failure branch: a plain HTTP request (no WS handshake headers)
// that otherwise passes auth+consent must fail at Upgrade, not panic or
// hang.
func TestHandler_UpgradeFailure_NotAWebSocketRequest(t *testing.T) {
	srv, cleanup := newHandlerTestServer(t, fakeAuthenticator{userID: "u1"}, true)
	defer cleanup()

	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	req.Header.Set("Authorization", "Bearer whatever")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.NotEqual(t, http.StatusSwitchingProtocols, resp.StatusCode)
}

func TestHandler_ConsentGranted_UpgradesAndExchangesFrames(t *testing.T) {
	srv, cleanup := newHandlerTestServer(t, fakeAuthenticator{userID: "u1"}, true)
	defer cleanup()

	wsURL := "ws" + srv.URL[len("http"):] + "?access_token=whatever"
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer conn.Close()
	if resp != nil {
		defer resp.Body.Close()
	}

	require.NoError(t, conn.WriteJSON(map[string]any{"type": "update", "lat": -23.5505, "lon": -46.6333}))

	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, data, err := conn.ReadMessage()
	require.NoError(t, err)

	var frame outboundFrame
	require.NoError(t, json.Unmarshal(data, &frame))
	assert.Equal(t, presence.FrameNearbyUpdate, frame.Type)
}

// TestHandler_UpdateFrame_WithNowPlaying_PropagatesTrackID exercises
// handleInbound's "now_playing" branch: the client's now_playing.track_id
// must reach the requester's own presence entry in the geo index (which is
// what a nearby user with presence_share_track visibility would later see).
func TestHandler_UpdateFrame_WithNowPlaying_PropagatesTrackID(t *testing.T) {
	settings := newFakeSettingsRepo()
	now := time.Now()
	due := now.Add(presence.ConsentValidityPeriod)
	settings.rows["u1"] = presence.PrivacySettings{
		UserID: "u1", PresenceVisibility: presence.VisibilityEveryone, VisibilityRadiusM: presence.DefaultRadiusM,
		PresenceShareTrack: true, ProximityConsentEnabled: true, ProximityConsentTS: &now, ProximityConsentRenewDue: &due,
	}
	blocks := newFakeBlockRepoWS()
	follows := newFakeFollowCheckerWS()
	geo := newFakeGeoIndexWS()
	audit := newFakeAuditRepoWS()
	profiles := newFakeProfileResolverWS()
	rl := newFakeRateLimiterWS()

	nearby := presence.NewNearbyService(settings, blocks, follows, geo, audit, profiles, rl, rl, presence.FixedJitterer{}, clock.NewFrozen(now), idgen.NewSequential("id"))
	hub := presence.NewHub(nearby, time.Minute, 2, 16, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	h := NewHandler(hub, nearby, fakeAuthenticator{userID: "u1"}, time.Minute, nil, testLogger())
	srv := httptest.NewServer(h)
	defer srv.Close()

	wsURL := "ws" + srv.URL[len("http"):] + "?access_token=whatever"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer conn.Close()

	require.NoError(t, conn.WriteJSON(map[string]any{
		"type": "update", "lat": 1.0, "lon": 1.0,
		"now_playing": map[string]any{"track_id": "track-xyz"},
	}))

	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, _, err = conn.ReadMessage() // wait for the resulting nearby_update frame (empty, but confirms processing finished)
	require.NoError(t, err)

	geo.mu.Lock()
	entry := geo.entries["u1"].entry
	geo.mu.Unlock()
	assert.Equal(t, "track-xyz", entry.TrackID)
}

func TestHandler_HeartbeatFrame(t *testing.T) {
	srv, cleanup := newHandlerTestServer(t, fakeAuthenticator{userID: "u1"}, true)
	defer cleanup()

	wsURL := "ws" + srv.URL[len("http"):] + "?access_token=whatever"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer conn.Close()

	require.NoError(t, conn.WriteJSON(map[string]any{"type": "heartbeat"}))
	// No entry exists yet (never sent "update"), so ApplyHeartbeat returns
	// nil results without erroring -- assert the connection stays open by
	// following up with an update and getting a reply.
	require.NoError(t, conn.WriteJSON(map[string]any{"type": "update", "lat": 1.0, "lon": 1.0}))
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, _, err = conn.ReadMessage()
	assert.NoError(t, err)
}

func TestHandler_MalformedFrame_ConnectionStaysOpen(t *testing.T) {
	srv, cleanup := newHandlerTestServer(t, fakeAuthenticator{userID: "u1"}, true)
	defer cleanup()

	wsURL := "ws" + srv.URL[len("http"):] + "?access_token=whatever"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer conn.Close()

	require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte("not json")))
	require.NoError(t, conn.WriteJSON(map[string]any{"type": "update", "lat": 1.0, "lon": 1.0}))

	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, _, err = conn.ReadMessage()
	assert.NoError(t, err, "a malformed frame must not kill the connection")
}

func TestHandler_UpdateFrame_MissingLatLon_Ignored(t *testing.T) {
	srv, cleanup := newHandlerTestServer(t, fakeAuthenticator{userID: "u1"}, true)
	defer cleanup()

	wsURL := "ws" + srv.URL[len("http"):] + "?access_token=whatever"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer conn.Close()

	require.NoError(t, conn.WriteJSON(map[string]any{"type": "update"})) // no lat/lon
	require.NoError(t, conn.WriteJSON(map[string]any{"type": "update", "lat": 1.0, "lon": 1.0}))

	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, _, err = conn.ReadMessage()
	assert.NoError(t, err)
}

func TestHandler_UnknownFrameType_Ignored(t *testing.T) {
	srv, cleanup := newHandlerTestServer(t, fakeAuthenticator{userID: "u1"}, true)
	defer cleanup()

	wsURL := "ws" + srv.URL[len("http"):] + "?access_token=whatever"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer conn.Close()

	require.NoError(t, conn.WriteJSON(map[string]any{"type": "something_unknown"}))
	require.NoError(t, conn.WriteJSON(map[string]any{"type": "update", "lat": 1.0, "lon": 1.0}))

	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, _, err = conn.ReadMessage()
	assert.NoError(t, err)
}

// TestHandler_UpdateFrame_RateLimited exercises handleInbound's layer-3
// backpressure branch: when the Hub's update-frame rate limiter denies,
// the frame is silently dropped instead of being enqueued.
func TestHandler_UpdateFrame_RateLimited(t *testing.T) {
	settings := newFakeSettingsRepo()
	now := time.Now()
	due := now.Add(presence.ConsentValidityPeriod)
	settings.rows["u1"] = presence.PrivacySettings{
		UserID: "u1", PresenceVisibility: presence.VisibilityEveryone, VisibilityRadiusM: presence.DefaultRadiusM,
		ProximityConsentEnabled: true, ProximityConsentTS: &now, ProximityConsentRenewDue: &due,
	}
	blocks := newFakeBlockRepoWS()
	follows := newFakeFollowCheckerWS()
	geo := newFakeGeoIndexWS()
	audit := newFakeAuditRepoWS()
	profiles := newFakeProfileResolverWS()
	rl := newFakeRateLimiterWS()

	nearby := presence.NewNearbyService(settings, blocks, follows, geo, audit, profiles, rl, rl, presence.FixedJitterer{}, clock.NewFrozen(now), idgen.NewSequential("id"))
	hub := presence.NewHub(nearby, time.Minute, 2, 16, denyAllLimiter{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	h := NewHandler(hub, nearby, fakeAuthenticator{userID: "u1"}, time.Minute, nil, testLogger())
	srv := httptest.NewServer(h)
	defer srv.Close()

	wsURL := "ws" + srv.URL[len("http"):] + "?access_token=whatever"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer conn.Close()

	require.NoError(t, conn.WriteJSON(map[string]any{"type": "update", "lat": 1.0, "lon": 1.0}))
	_ = conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	_, _, err = conn.ReadMessage()
	assert.Error(t, err, "the rate-limited update must never reach the pipeline, so no frame should arrive")
}

type denyAllLimiter struct{}

func (denyAllLimiter) Allow(context.Context, string, int, time.Duration) (bool, time.Duration, error) {
	return false, time.Second, nil
}

func TestHandler_VisibilityFrame(t *testing.T) {
	srv, cleanup := newHandlerTestServer(t, fakeAuthenticator{userID: "u1"}, true)
	defer cleanup()

	wsURL := "ws" + srv.URL[len("http"):] + "?access_token=whatever"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer conn.Close()

	require.NoError(t, conn.WriteJSON(map[string]any{"type": "visibility", "mode": "invisible"}))
	time.Sleep(50 * time.Millisecond) // let the server process it; no reply frame is expected for this type
}

// TestHandler_ConsentRevoked_MidSession_ClientReceivesDrainBeforeClose is a
// regression test for Bug 2 (Auditor-reported, reproduced 4/4 times):
// hub.go's Hub.process (~line 279) enqueues a "drain" frame carrying a
// reconnect_hint and then immediately calls Conn.Close() when a connected
// user's proximity consent is found revoked/expired mid-session (e.g. a
// REST DELETE /v1/presence/consent from another device, or the 6-month
// renewal window lapsing while this WS connection sat idle-but-open). The
// bug lived in conn.go's close(): it tore the physical socket down
// synchronously, racing writePump's own goroutine for that just-enqueued
// frame and routinely winning the race — the client only ever observed a
// bare 1006 close, never the drain frame explaining why.
//
// This deliberately exercises the REAL *conn (via a real Handler, a real
// httptest WS server, and a real client dial) rather than hub_test.go's
// channel-based fake Conn: the bug is entirely in the timing between
// conn.close() and its own writePump goroutine, which a fake Conn has no
// such race to get wrong in the first place. It must fail against the
// pre-fix conn.go (client's second ReadMessage gets a close error instead
// of the drain frame).
func TestHandler_ConsentRevoked_MidSession_ClientReceivesDrainBeforeClose(t *testing.T) {
	settings := newFakeSettingsRepo()
	now := time.Now()
	due := now.Add(presence.ConsentValidityPeriod)
	settings.rows["u1"] = presence.PrivacySettings{
		UserID: "u1", PresenceVisibility: presence.VisibilityEveryone, VisibilityRadiusM: presence.DefaultRadiusM,
		ProximityConsentEnabled: true, ProximityConsentTS: &now, ProximityConsentRenewDue: &due,
	}
	blocks := newFakeBlockRepoWS()
	follows := newFakeFollowCheckerWS()
	geo := newFakeGeoIndexWS()
	audit := newFakeAuditRepoWS()
	profiles := newFakeProfileResolverWS()
	rl := newFakeRateLimiterWS()
	clk := clock.NewFrozen(now)

	nearby := presence.NewNearbyService(settings, blocks, follows, geo, audit, profiles, rl, rl, presence.FixedJitterer{}, clk, idgen.NewSequential("id"))
	hub := presence.NewHub(nearby, time.Minute, 2, 16, nil)
	ctx, cancel := context.WithCancel(context.Background())
	go hub.Run(ctx)
	defer cancel()

	h := NewHandler(hub, nearby, fakeAuthenticator{userID: "u1"}, time.Minute, nil, testLogger())
	srv := httptest.NewServer(h)
	defer srv.Close()

	wsURL := "ws" + srv.URL[len("http"):] + "?access_token=whatever"
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer conn.Close()
	if resp != nil {
		defer resp.Body.Close()
	}

	// Establish the session is alive: one normal "update" round-trip.
	require.NoError(t, conn.WriteJSON(map[string]any{"type": "update", "lat": -23.5505, "lon": -46.6333}))
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, data, err := conn.ReadMessage()
	require.NoError(t, err)
	var initial outboundFrame
	require.NoError(t, json.Unmarshal(data, &initial))
	require.Equal(t, presence.FrameNearbyUpdate, initial.Type)

	// Revoke consent mid-session without touching the still-open WS
	// connection (simulating a REST DELETE /v1/presence/consent from a
	// different device/session than this one).
	settings.mu.Lock()
	s := settings.rows["u1"]
	s.ProximityConsentEnabled = false
	settings.rows["u1"] = s
	settings.mu.Unlock()

	// Trigger Hub.process's mid-session re-validation: checkConsent runs on
	// every update/heartbeat, not just at handshake.
	require.NoError(t, conn.WriteJSON(map[string]any{"type": "update", "lat": -23.5505, "lon": -46.6333}))

	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, data, err = conn.ReadMessage()
	require.NoError(t, err, "client must receive the drain frame BEFORE the connection closes, not just a bare close")

	var drain outboundFrame
	require.NoError(t, json.Unmarshal(data, &drain))
	assert.Equal(t, presence.FrameDrain, drain.Type)
	assert.Equal(t, "consent_required", drain.ReconnectHint)

	// The connection must then actually close (drain, not a leaked/frozen
	// connection).
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, _, err = conn.ReadMessage()
	assert.Error(t, err, "connection should close after the drain frame")
}

func TestBearerToken_HeaderPreferredOverQuery(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/?access_token=fromquery", nil)
	r.Header.Set("Authorization", "Bearer fromheader")
	assert.Equal(t, "fromheader", bearerToken(r))
}

func TestBearerToken_FallsBackToQuery(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/?access_token=fromquery", nil)
	assert.Equal(t, "fromquery", bearerToken(r))
}

func TestBearerToken_Empty(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	assert.Equal(t, "", bearerToken(r))
}

type assertErrWS string

func (e assertErrWS) Error() string { return string(e) }
