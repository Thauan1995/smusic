package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestRequireAuth_Success(t *testing.T) {
	var gotUserID string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uid, ok := UserID(r.Context())
		require.True(t, ok)
		gotUserID = uid
		w.WriteHeader(http.StatusOK)
	})

	h := RequireAuth(fakeAuthenticator{userID: "user-1"})(next)

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer good-token")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "user-1", gotUserID)
}

func TestRequireAuth_MissingHeader(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler must not be called")
	})
	h := RequireAuth(fakeAuthenticator{})(next)

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestRequireAuth_MalformedHeader(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler must not be called")
	})
	h := RequireAuth(fakeAuthenticator{})(next)

	cases := []string{"Basic abcdef", "Bearer", "Bearer "}
	for _, header := range cases {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.Header.Set("Authorization", header)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		assert.Equal(t, http.StatusUnauthorized, w.Code, "header=%q", header)
	}
}

func TestRequireAuth_InvalidToken(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler must not be called")
	})
	h := RequireAuth(fakeAuthenticator{err: errors.New("bad token")})(next)

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer whatever")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestUserID_NotPresent(t *testing.T) {
	_, ok := UserID(httptest.NewRequest(http.MethodGet, "/", nil).Context())
	assert.False(t, ok)
}
