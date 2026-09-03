package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type fakeRateLimiter struct {
	allowed    bool
	retryAfter time.Duration
	err        error
	calledWith string
}

func (f *fakeRateLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, time.Duration, error) {
	f.calledWith = key
	return f.allowed, f.retryAfter, f.err
}

func TestRateLimit_Allowed(t *testing.T) {
	rl := &fakeRateLimiter{allowed: true}
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true })

	h := RateLimit(rl, ClientIPKey("login"), 5, time.Minute)(next)

	r := httptest.NewRequest(http.MethodPost, "/", nil)
	r.RemoteAddr = "1.2.3.4:5555"
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	assert.True(t, called)
	assert.Equal(t, "login:1.2.3.4", rl.calledWith)
}

func TestRateLimit_Denied(t *testing.T) {
	rl := &fakeRateLimiter{allowed: false, retryAfter: 30 * time.Second}
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Fatal("must not call next") })

	h := RateLimit(rl, ClientIPKey("login"), 5, time.Minute)(next)

	r := httptest.NewRequest(http.MethodPost, "/", nil)
	r.RemoteAddr = "1.2.3.4:5555"
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	assert.Equal(t, "30", w.Header().Get("Retry-After"))
}

func TestRateLimit_LimiterErrorFailsOpen(t *testing.T) {
	rl := &fakeRateLimiter{err: errors.New("redis down")}
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true })

	h := RateLimit(rl, ClientIPKey("login"), 5, time.Minute)(next)

	r := httptest.NewRequest(http.MethodPost, "/", nil)
	r.RemoteAddr = "1.2.3.4:5555"
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	assert.True(t, called, "middleware must fail open when the limiter errors")
}

func TestClientIPKey_NoPort(t *testing.T) {
	fn := ClientIPKey("p")
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "no-port-here"
	assert.Equal(t, "p:no-port-here", fn(r))
}
