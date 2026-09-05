// Command http-loadtest measures p50/p95 latency for the HTTP-based
// backend-go.md §6 targets (search, play, seek) against a running smusic
// deployment, using vegeta as a library. See
// .vibeflow/specs/performance-benchmark-harness.md — this is the harness
// that spec builds; it does not itself validate or fail a build, it
// reports real numbers for a human to compare against the targets.
//
// This tool is a client of the already-deployed API, not exercised by
// any test suite itself — no coverage exclusion needed (it isn't part of
// the coverage target the same way cmd/server's wiring is; it's a
// separate operational tool, like cmd/migrate).
//
// Usage:
//
//	go run ./loadtest/http -base-url http://localhost:8080 -database-url "$DATABASE_URL"
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
	"net/url"
	"os"
	"time"

	vegeta "github.com/tsenart/vegeta/v12/lib"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	baseURL := flag.String("base-url", "http://localhost:8080", "smusic-core base URL")
	databaseURL := flag.String("database-url", "postgres://smusic:smusic@localhost:5432/smusic?sslmode=disable", "Postgres DSN, used only to grant the test user catalog_curator (see .vibeflow/specs/catalog-write-authorization.md)")
	duration := flag.Duration("duration", 15*time.Second, "how long to run each attack")
	rate := flag.Int("rate", 50, "requests per second per attack")
	reportPath := flag.String("report", "", "optional path to write the report to, in addition to stdout")
	flag.Parse()

	if err := run(*baseURL, *databaseURL, *duration, *rate, *reportPath); err != nil {
		log.Fatal(err)
	}
}

func run(baseURL, databaseURL string, duration time.Duration, rate int, reportPath string) error {
	ctx := context.Background()

	email := fmt.Sprintf("loadtest+%d@smusic.test", time.Now().UnixNano())
	token, userID, err := signUp(baseURL, email)
	if err != nil {
		return fmt.Errorf("sign up: %w", err)
	}
	log.Printf("signed up loadtest user %s (%s)", email, userID)

	if err := grantCatalogCurator(ctx, databaseURL, userID); err != nil {
		return fmt.Errorf("grant catalog_curator (is Postgres reachable at %s?): %w", databaseURL, err)
	}

	trackID, trackTitle, err := seedCatalog(baseURL, token)
	if err != nil {
		return fmt.Errorf("seed catalog: %w", err)
	}
	log.Printf("seeded track %s (%q)", trackID, trackTitle)

	sessionID, err := createPlaybackSession(baseURL, token)
	if err != nil {
		return fmt.Errorf("create playback session: %w", err)
	}
	log.Printf("created playback session %s", sessionID)

	var out bytes.Buffer
	fmt.Fprintf(&out, "# smusic HTTP performance baseline\n")
	fmt.Fprintf(&out, "Generated: %s\nTarget: %s\nRate: %d req/s, duration: %s\n\n", time.Now().Format(time.RFC3339), baseURL, rate, duration)

	attacks := []struct {
		name    string
		target  vegeta.Target
		p50Goal time.Duration
		p95Goal time.Duration
	}{
		{
			name: "catalog search",
			target: vegeta.Target{
				Method: http.MethodGet,
				URL:    baseURL + "/v1/catalog/search?q=" + url.QueryEscape(trackTitle) + "&type=track",
				Header: authHeader(token),
			},
			p50Goal: 100 * time.Millisecond,
			p95Goal: 300 * time.Millisecond,
		},
		{
			name: "playback play",
			target: vegeta.Target{
				Method: http.MethodPost,
				URL:    baseURL + "/v1/playback/sessions/" + sessionID + "/play",
				Body:   mustJSON(map[string]string{"track_id": trackID}),
				Header: authHeader(token),
			},
			// backend-go.md §6 gives no direct target for the play call
			// itself (it's a component of "time to first audio", most of
			// which is client-side/CDN); reusing the seek target as the
			// closest server-side proxy metric.
			p50Goal: 150 * time.Millisecond,
			p95Goal: 400 * time.Millisecond,
		},
		{
			name: "playback seek",
			target: vegeta.Target{
				Method: http.MethodPost,
				URL:    baseURL + "/v1/playback/sessions/" + sessionID + "/seek",
				Body:   mustJSON(map[string]int{"position_ms": 30000}),
				Header: authHeader(token),
			},
			p50Goal: 150 * time.Millisecond,
			p95Goal: 400 * time.Millisecond,
		},
	}

	for _, a := range attacks {
		metrics := attack(a.target, rate, duration)
		pass50 := metrics.Latencies.P50 <= a.p50Goal
		pass95 := metrics.Latencies.P95 <= a.p95Goal
		fmt.Fprintf(&out, "## %s\n", a.name)
		fmt.Fprintf(&out, "- requests: %d, success: %.1f%%\n", metrics.Requests, metrics.Success*100)
		fmt.Fprintf(&out, "- p50: %s (target ≤ %s) — %s\n", metrics.Latencies.P50, a.p50Goal, passFail(pass50))
		fmt.Fprintf(&out, "- p95: %s (target ≤ %s) — %s\n", metrics.Latencies.P95, a.p95Goal, passFail(pass95))
		if len(metrics.Errors) > 0 {
			fmt.Fprintf(&out, "- errors: %v\n", metrics.Errors)
		}
		fmt.Fprintln(&out)
	}

	fmt.Print(out.String())
	if reportPath != "" {
		if err := os.WriteFile(reportPath, out.Bytes(), 0o600); err != nil {
			return fmt.Errorf("write report: %w", err)
		}
		log.Printf("report written to %s", reportPath)
	}
	return nil
}

func passFail(ok bool) string {
	if ok {
		return "PASS"
	}
	return "FAIL"
}

func attack(target vegeta.Target, rate int, duration time.Duration) *vegeta.Metrics {
	targeter := vegeta.NewStaticTargeter(target)
	attacker := vegeta.NewAttacker()
	pacer := vegeta.Rate{Freq: rate, Per: time.Second}

	var metrics vegeta.Metrics
	for res := range attacker.Attack(targeter, pacer, duration, target.Method+" "+target.URL) {
		metrics.Add(res)
	}
	metrics.Close()
	return &metrics
}

func authHeader(token string) http.Header {
	h := http.Header{}
	h.Set("Authorization", "Bearer "+token)
	h.Set("Content-Type", "application/json")
	return h
}

func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err) // unreachable for the static, hand-built values this tool passes
	}
	return b
}

func signUp(baseURL, email string) (token string, userID string, err error) {
	body := mustJSON(map[string]string{
		"email": email, "password": "LoadTest!2026", "display_name": "Load Test",
	})
	resp, err := http.Post(baseURL+"/v1/auth/signup", "application/json", bytes.NewReader(body))
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		return "", "", fmt.Errorf("signup: status %d: %s", resp.StatusCode, b)
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

func grantCatalogCurator(ctx context.Context, databaseURL, userID string) error {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	_, err = pool.Exec(ctx, `UPDATE users SET role = 'catalog_curator' WHERE id = $1`, userID)
	return err
}

func seedCatalog(baseURL, token string) (trackID, trackTitle string, err error) {
	artistID, err := postJSON(baseURL+"/v1/catalog/artists", token, map[string]string{"name": "Loadtest Artist"}, "id")
	if err != nil {
		return "", "", fmt.Errorf("create artist: %w", err)
	}
	trackTitle = fmt.Sprintf("Loadtest Track %d", time.Now().UnixNano())
	trackID, err = postJSON(baseURL+"/v1/catalog/tracks", token, map[string]any{
		"title":       trackTitle,
		"duration_ms": 210000,
		"artists":     []map[string]string{{"artist_id": artistID, "role": "primary"}},
	}, "id")
	if err != nil {
		return "", "", fmt.Errorf("create track: %w", err)
	}
	return trackID, trackTitle, nil
}

func createPlaybackSession(baseURL, token string) (string, error) {
	return postJSON(baseURL+"/v1/playback/sessions", token, map[string]string{"device_id": "loadtest-device"}, "session_id")
}

// postJSON POSTs body to url with an auth header and extracts idField
// (e.g. "id" or "session_id") from the JSON response.
func postJSON(url, token string, body any, idField string) (string, error) {
	b := mustJSON(body)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		rb, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("status %d: %s", resp.StatusCode, rb)
	}
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	id, _ := out[idField].(string)
	if id == "" {
		return "", fmt.Errorf("response missing field %q: %v", idField, out)
	}
	return id, nil
}
