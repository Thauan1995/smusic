package middleware

import (
	"context"
	"net/http"

	"smusic/backend/internal/platform/httpx"
)

// RoleChecker answers "does userID's account have exactly this role" —
// *auth.Service.HasRole implements this, wired only where a specific
// route needs it (.vibeflow/specs/catalog-write-authorization.md), never
// imported by a module directly (backend-go.md §1's module-boundary rule
// stays intact: this interface lives in the shared middleware package,
// not inside auth or catalog).
type RoleChecker interface {
	HasRole(ctx context.Context, userID string, role string) (bool, error)
}

// RequireRole returns middleware that rejects (403) an otherwise-
// authenticated request whose user lacks role. Must be mounted after
// RequireAuth — it reads the user ID RequireAuth already placed in the
// request context, and returns 401 (not 403) if that's missing, which
// only happens if RequireRole is wired without RequireAuth ahead of it (a
// wiring bug, not a normal request outcome).
func RequireRole(checker RoleChecker, role string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, ok := UserID(r.Context())
			if !ok {
				httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing authenticated user")
				return
			}
			hasRole, err := checker.HasRole(r.Context(), userID, role)
			if err != nil {
				httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "internal error")
				return
			}
			if !hasRole {
				httpx.WriteError(w, http.StatusForbidden, "forbidden", "this action requires the "+role+" role")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
