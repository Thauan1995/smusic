// Package middleware holds cross-cutting HTTP middleware shared by every
// module's router: bearer-token authentication and rate limiting. Keeping
// authorization here — a "single central boundary", per backend-go.md §1 —
// rather than scattered per-handler checks is what backend-go.md calls out
// as the thing that makes the system auditable by the security team.
package middleware

import (
	"context"
	"net/http"
	"strings"

	"smusic/backend/internal/platform/httpx"
)

// Authenticator verifies a bearer access token and returns the subject's
// user ID. token.Signer (internal/auth/token) implements this.
type Authenticator interface {
	Authenticate(token string) (userID string, err error)
}

type ctxKey int

const userIDCtxKey ctxKey = iota

// UserID extracts the authenticated user ID placed in the request context
// by RequireAuth. ok is false if no authenticated user is present.
func UserID(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(userIDCtxKey).(string)
	return v, ok
}

// WithUserID returns a context carrying userID, for use in tests that
// exercise handlers directly without going through RequireAuth.
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDCtxKey, userID)
}

// RequireAuth returns middleware that rejects requests without a valid
// "Authorization: Bearer <token>" header and, for valid ones, places the
// authenticated user ID in the request context for downstream handlers.
func RequireAuth(authr Authenticator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			const prefix = "Bearer "
			if !strings.HasPrefix(header, prefix) || len(header) <= len(prefix) {
				httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing or malformed Authorization header")
				return
			}
			token := strings.TrimPrefix(header, prefix)

			userID, err := authr.Authenticate(token)
			if err != nil {
				httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "invalid or expired access token")
				return
			}

			ctx := WithUserID(r.Context(), userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
