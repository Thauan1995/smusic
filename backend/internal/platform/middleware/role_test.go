package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

type fakeRoleChecker struct {
	hasRole bool
	err     error
}

func (f fakeRoleChecker) HasRole(_ context.Context, _ string, _ string) (bool, error) {
	return f.hasRole, f.err
}

func TestRequireRole_Success(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	h := RequireRole(fakeRoleChecker{hasRole: true}, "catalog_curator")(next)

	r := httptest.NewRequest(http.MethodPost, "/", nil).WithContext(WithUserID(context.Background(), "user-1"))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, called)
}

func TestRequireRole_Forbidden(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler must not be called")
	})
	h := RequireRole(fakeRoleChecker{hasRole: false}, "catalog_curator")(next)

	r := httptest.NewRequest(http.MethodPost, "/", nil).WithContext(WithUserID(context.Background(), "user-1"))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestRequireRole_CheckerError(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler must not be called")
	})
	h := RequireRole(fakeRoleChecker{err: errors.New("boom")}, "catalog_curator")(next)

	r := httptest.NewRequest(http.MethodPost, "/", nil).WithContext(WithUserID(context.Background(), "user-1"))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// TestRequireRole_NoUserID: RequireRole must be mounted after RequireAuth;
// if it somehow isn't (a wiring bug, not a normal request), fail closed
// with 401 rather than calling the checker with an empty user ID.
func TestRequireRole_NoUserID(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler must not be called")
	})
	h := RequireRole(fakeRoleChecker{hasRole: true}, "catalog_curator")(next)

	r := httptest.NewRequest(http.MethodPost, "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
