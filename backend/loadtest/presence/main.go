// Command presence-loadtest measures presence fanout latency
// (backend-go.md §6: "update de um usuário → visível para vizinhos
// conectados ≤ 2s p95") against a running smusic deployment. See
// .vibeflow/specs/performance-benchmark-harness.md.
//
// Each simulated client: signs up, enrolls+verifies TOTP MFA (required to
// grant proximity consent — .vibeflow/specs/mfa-for-proximity-consent.md),
// grants consent, sets visibility=everyone/radius=1km/unpaused, then
// connects to the presence WebSocket and heartbeats periodically. One
// "trigger" client sends a fresh update; every other already-connected
// client is watched for the first nearby_update frame that reports the
// trigger's user_id, and that delay is the fanout latency for this N.
//
// Usage:
//
//	go run ./loadtest/presence -api-base-url http://localhost:8080 -ws-base-url ws://localhost:8081 -clients 50
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pquerna/otp/totp"
)

func main() {
	apiBaseURL := flag.String("api-base-url", "http://localhost:8080", "smusic-core base URL (signup/MFA/consent/settings)")
	wsBaseURL := flag.String("ws-base-url", "ws://localhost:8081", "presence-server base URL (WS /v1/presence/connect)")
	clientCounts := flag.String("clients", "10,50,200", "comma-separated N values to test, ascending")
	flag.Parse()

	var ns []int
	for _, s := range splitCSV(*clientCounts) {
		var n int
		if _, err := fmt.Sscanf(s, "%d", &n); err != nil {
			log.Fatalf("invalid -clients value %q: %v", s, err)
		}
		ns = append(ns, n)
	}

	for _, n := range ns {
		latency, err := runOnce(*apiBaseURL, *wsBaseURL, n)
		if err != nil {
			log.Printf("N=%d: FAILED: %v", n, err)
			continue
		}
		fmt.Printf("N=%d clients: fanout latency = %s (target ≤ 2s p95, single-sample here — see .vibeflow/specs/performance-benchmark-harness.md's caveat on home-lab hardware)\n", n, latency)
	}
}

func splitCSV(s string) []string {
	var out []string
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == ',' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	return out
}

type client struct {
	userID string
	token  string
	conn   *websocket.Conn
}

// runOnce sets up n clients, has one send a fresh update, and returns how
// long it took for another already-connected client's heartbeat to first
// report the trigger client's presence.
func runOnce(apiBaseURL, wsBaseURL string, n int) (time.Duration, error) {
	if n < 2 {
		return 0, fmt.Errorf("need at least 2 clients to measure fanout, got %d", n)
	}

	clients := make([]*client, 0, n)
	defer func() {
		for _, c := range clients {
			if c.conn != nil {
				_ = c.conn.Close()
			}
		}
	}()

	baseLat, baseLon := -23.5505, -46.6333 // clustered within a few meters of each other
	for i := 0; i < n; i++ {
		c, err := setupClient(apiBaseURL, wsBaseURL, i)
		if err != nil {
			return 0, fmt.Errorf("setup client %d: %w", i, err)
		}
		clients = append(clients, c)
		// Establish initial presence for everyone before measuring.
		if err := sendUpdate(c.conn, baseLat, baseLon); err != nil {
			return 0, fmt.Errorf("initial update for client %d: %w", i, err)
		}
	}

	// Let the index settle.
	time.Sleep(300 * time.Millisecond)

	trigger := clients[0]
	watchers := clients[1:]

	var wg sync.WaitGroup
	firstSeen := make(chan time.Duration, len(watchers))
	stop := make(chan struct{})
	start := time.Now()

	for _, w := range watchers {
		wg.Add(1)
		go func(w *client) {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				_ = w.conn.SetReadDeadline(time.Now().Add(3 * time.Second))
				_, data, err := w.conn.ReadMessage()
				if err != nil {
					return
				}
				var frame struct {
					Type  string `json:"type"`
					Users []struct {
						UserID string `json:"user_id"`
					} `json:"users"`
				}
				if err := json.Unmarshal(data, &frame); err != nil {
					continue
				}
				for _, u := range frame.Users {
					if u.UserID == trigger.userID {
						select {
						case firstSeen <- time.Since(start):
						default:
						}
						return
					}
				}
			}
		}(w)
	}

	// Watchers heartbeat periodically so they actually re-query their
	// nearby list (this backend computes it synchronously per request,
	// not via async push — see internal/presence/hub.go).
	heartbeatStop := make(chan struct{})
	for _, w := range watchers {
		go func(w *client) {
			t := time.NewTicker(200 * time.Millisecond)
			defer t.Stop()
			for {
				select {
				case <-heartbeatStop:
					return
				case <-t.C:
					_ = w.conn.WriteJSON(map[string]string{"type": "heartbeat"})
				}
			}
		}(w)
	}

	// The trigger: a fresh update, at t=0 for this measurement.
	if err := sendUpdate(trigger.conn, baseLat+0.00001, baseLon+0.00001); err != nil {
		close(stop)
		close(heartbeatStop)
		return 0, fmt.Errorf("trigger update: %w", err)
	}

	var result time.Duration
	select {
	case result = <-firstSeen:
	case <-time.After(10 * time.Second):
		result = -1 // not observed within the timeout
	}
	close(stop)
	close(heartbeatStop)
	wg.Wait()

	if result < 0 {
		return 0, fmt.Errorf("no watcher observed the trigger client within 10s (N=%d)", n)
	}
	return result, nil
}

func sendUpdate(conn *websocket.Conn, lat, lon float64) error {
	return conn.WriteJSON(map[string]any{"type": "update", "lat": lat, "lon": lon})
}

func setupClient(apiBaseURL, wsBaseURL string, i int) (*client, error) {
	email := fmt.Sprintf("presence-loadtest-%d-%d@smusic.test", time.Now().UnixNano(), i)
	token, userID, err := signUp(apiBaseURL, email)
	if err != nil {
		return nil, fmt.Errorf("signup: %w", err)
	}

	secret, err := enrollMFA(apiBaseURL, token)
	if err != nil {
		return nil, fmt.Errorf("enroll mfa: %w", err)
	}
	code, err := totp.GenerateCode(secret, time.Now())
	if err != nil {
		return nil, fmt.Errorf("generate totp code: %w", err)
	}
	if err := verifyMFA(apiBaseURL, token, code); err != nil {
		return nil, fmt.Errorf("verify mfa: %w", err)
	}

	if err := grantConsent(apiBaseURL, token); err != nil {
		return nil, fmt.Errorf("grant consent: %w", err)
	}
	if err := updateSettings(apiBaseURL, token); err != nil {
		return nil, fmt.Errorf("update settings: %w", err)
	}

	conn, err := connectWS(wsBaseURL, token)
	if err != nil {
		return nil, fmt.Errorf("connect ws: %w", err)
	}

	return &client{userID: userID, token: token, conn: conn}, nil
}

func signUp(baseURL, email string) (token, userID string, err error) {
	body, _ := json.Marshal(map[string]string{"email": email, "password": "LoadTest!2026", "display_name": "Presence Load Test"})
	resp, err := http.Post(baseURL+"/v1/auth/signup", "application/json", bytes.NewReader(body))
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		return "", "", fmt.Errorf("status %d: %s", resp.StatusCode, b)
	}
	var out struct {
		UserID      string `json:"user_id"`
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", "", err
	}
	return out.AccessToken, out.UserID, nil
}

func enrollMFA(baseURL, token string) (secret string, err error) {
	var out struct {
		Secret string `json:"secret"`
	}
	if err := doJSON(baseURL+"/v1/auth/mfa/enroll", token, nil, &out); err != nil {
		return "", err
	}
	return out.Secret, nil
}

func verifyMFA(baseURL, token, code string) error {
	return doJSON(baseURL+"/v1/auth/mfa/verify", token, map[string]string{"code": code}, nil)
}

func grantConsent(baseURL, token string) error {
	return doJSON(baseURL+"/v1/presence/consent", token, nil, nil)
}

func updateSettings(baseURL, token string) error {
	req, err := http.NewRequest(http.MethodPut, baseURL+"/v1/presence/settings", bytes.NewReader(mustJSON(map[string]any{
		"presence_visibility": "everyone",
		"visibility_radius_m": 1000,
		"paused":              false,
	})))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("status %d: %s", resp.StatusCode, b)
	}
	return nil
}

func doJSON(url, token string, body any, out any) error {
	var reader *bytes.Reader
	if body != nil {
		reader = bytes.NewReader(mustJSON(body))
	} else {
		reader = bytes.NewReader(nil)
	}
	req, err := http.NewRequest(http.MethodPost, url, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("status %d: %s", resp.StatusCode, b)
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}

func connectWS(baseURL, token string) (*websocket.Conn, error) {
	u := baseURL + "/v1/presence/connect?access_token=" + token
	conn, _, err := websocket.DefaultDialer.DialContext(context.Background(), u, nil)
	return conn, err
}

func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}
