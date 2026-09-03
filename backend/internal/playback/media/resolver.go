// Package media implements playback.MediaURLResolver.
//
// TODO(backend-go.md §2, "media-edge-service"): this slice does NOT talk to
// a CDN. LocalResolver instead signs a URL pointing at a local static test
// asset (served by cmd/server from testdata/media/, see README) using the
// same shape a real implementation would (short-lived, HMAC-signed,
// track-scoped) so the playback module's contract and tests don't change
// when a real media-edge-service is wired in for Fatia 2 — only this
// file's internals do.
package media

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	"smusic/backend/internal/platform/clock"
)

// DefaultTTL is the validity window of a resolved URL (backend-go.md §2:
// "URLs assinadas e de curta duração (ex.: 5-10 min)").
const DefaultTTL = 10 * time.Minute

// LocalResolver signs URLs against a fixed local base URL and secret,
// simulating the CDN-signed-URL contract without a real CDN.
type LocalResolver struct {
	baseURL    string
	signingKey []byte
	ttl        time.Duration
	clock      clock.Clock
}

// NewLocalResolver returns a LocalResolver. baseURL should point at
// cmd/server's local media file handler (e.g.
// "http://localhost:8080/media"); signingKey is a local dev-only secret
// (see internal/platform/config's MediaSigningKey TODO).
func NewLocalResolver(baseURL string, signingKey []byte, clk clock.Clock) *LocalResolver {
	return &LocalResolver{baseURL: baseURL, signingKey: signingKey, ttl: DefaultTTL, clock: clk}
}

// Resolve implements playback.MediaURLResolver.
func (r *LocalResolver) Resolve(_ context.Context, trackID string) (string, time.Time, error) {
	expiresAt := r.clock.Now().Add(r.ttl)
	exp := strconv.FormatInt(expiresAt.Unix(), 10)
	sig := r.sign(trackID, exp)
	url := fmt.Sprintf("%s/sample.mp3?track=%s&exp=%s&sig=%s", r.baseURL, trackID, exp, sig)
	return url, expiresAt, nil
}

// Verify checks a signature produced by Resolve, for use by the local
// media file handler (cmd/server) to reject expired/tampered URLs. It is
// exported so the HTTP handler serving the static test asset can enforce
// the same contract a real CDN's signed-URL verification would.
func (r *LocalResolver) Verify(trackID, exp, sig string) bool {
	want := r.sign(trackID, exp)
	if !hmac.Equal([]byte(want), []byte(sig)) {
		return false
	}
	expUnix, err := strconv.ParseInt(exp, 10, 64)
	if err != nil {
		return false
	}
	return r.clock.Now().Unix() <= expUnix
}

func (r *LocalResolver) sign(trackID, exp string) string {
	mac := hmac.New(sha256.New, r.signingKey)
	mac.Write([]byte(trackID))
	mac.Write([]byte("."))
	mac.Write([]byte(exp))
	return hex.EncodeToString(mac.Sum(nil))
}
