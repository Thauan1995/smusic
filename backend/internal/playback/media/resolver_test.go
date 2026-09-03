package media

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"smusic/backend/internal/platform/clock"
)

func TestLocalResolver_Resolve(t *testing.T) {
	clk := clock.NewFrozen(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	r := NewLocalResolver("http://localhost:8080/media", []byte("secret"), clk)

	url, expiresAt, err := r.Resolve(context.Background(), "track-1")
	require.NoError(t, err)
	assert.Equal(t, clk.Now().Add(DefaultTTL), expiresAt)
	assert.Contains(t, url, "track=track-1")
	assert.Contains(t, url, "http://localhost:8080/media/sample.mp3")
}

func TestLocalResolver_VerifyRoundTrip(t *testing.T) {
	clk := clock.NewFrozen(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	r := NewLocalResolver("http://x", []byte("secret"), clk)

	url, _, err := r.Resolve(context.Background(), "track-1")
	require.NoError(t, err)

	trackID, exp, sig := parseTestURL(t, url)
	assert.True(t, r.Verify(trackID, exp, sig))
}

func TestLocalResolver_VerifyRejectsTamperedSignature(t *testing.T) {
	clk := clock.NewFrozen(time.Now())
	r := NewLocalResolver("http://x", []byte("secret"), clk)
	assert.False(t, r.Verify("track-1", "9999999999", "not-a-real-signature"))
}

func TestLocalResolver_VerifyRejectsWrongKey(t *testing.T) {
	clk := clock.NewFrozen(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	r1 := NewLocalResolver("http://x", []byte("secret-a"), clk)
	r2 := NewLocalResolver("http://x", []byte("secret-b"), clk)

	url, _, err := r1.Resolve(context.Background(), "track-1")
	require.NoError(t, err)
	trackID, exp, sig := parseTestURL(t, url)

	assert.False(t, r2.Verify(trackID, exp, sig))
}

func TestLocalResolver_VerifyRejectsExpired(t *testing.T) {
	clk := clock.NewFrozen(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	r := NewLocalResolver("http://x", []byte("secret"), clk)

	url, _, err := r.Resolve(context.Background(), "track-1")
	require.NoError(t, err)
	trackID, exp, sig := parseTestURL(t, url)

	clk.Advance(DefaultTTL + time.Minute)
	assert.False(t, r.Verify(trackID, exp, sig))
}

func TestLocalResolver_VerifyRejectsMalformedExpiry(t *testing.T) {
	clk := clock.NewFrozen(time.Now())
	r := NewLocalResolver("http://x", []byte("secret"), clk)
	sig := r.sign("track-1", "not-a-number")
	assert.False(t, r.Verify("track-1", "not-a-number", sig))
}

// parseTestURL extracts track/exp/sig from a Resolve()-produced URL
// without importing net/url just for the test (the query string shape is
// fixed and owned by this package).
func parseTestURL(t *testing.T, url string) (trackID, exp, sig string) {
	t.Helper()
	var query string
	for i := 0; i < len(url); i++ {
		if url[i] == '?' {
			query = url[i+1:]
			break
		}
	}
	require.NotEmpty(t, query)

	params := map[string]string{}
	start := 0
	for i := 0; i <= len(query); i++ {
		if i == len(query) || query[i] == '&' {
			pair := query[start:i]
			for j := 0; j < len(pair); j++ {
				if pair[j] == '=' {
					params[pair[:j]] = pair[j+1:]
					break
				}
			}
			start = i + 1
		}
	}
	return params["track"], params["exp"], params["sig"]
}
