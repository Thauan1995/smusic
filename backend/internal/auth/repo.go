package auth

import (
	"context"
	"time"
)

// UserRepository persists and retrieves User records.
// internal/auth/postgres implements this against Postgres; unit tests use
// an in-memory fake (see service_test.go).
type UserRepository interface {
	Create(ctx context.Context, u User) error
	GetByEmail(ctx context.Context, email string) (User, error)
	GetByID(ctx context.Context, id string) (User, error)
}

// IdentityRepository links OAuth/OIDC identities to users
// (data-architecture.md §1.1 user_auth_identities).
type IdentityRepository interface {
	GetUserIDByProvider(ctx context.Context, provider, providerUserID string) (string, error)
	Link(ctx context.Context, identity Identity) error
}

// DeviceRepository upserts device records used to bind refresh tokens to a
// specific client device (security.md §2: per-device logout).
type DeviceRepository interface {
	// Upsert creates or updates the device identified by (userID,
	// platform, pushToken-or-appVersion-derived-identity) and returns its
	// ID. The concrete matching key is left to the implementation.
	Upsert(ctx context.Context, d Device) (id string, err error)
}

// RefreshTokenRepository persists hashed refresh tokens and their
// revocation state.
type RefreshTokenRepository interface {
	Store(ctx context.Context, rt RefreshToken) error
	GetByHash(ctx context.Context, tokenHash string) (RefreshToken, error)
	// Revoke marks id as revoked at the given instant. Revoking an
	// already-revoked token is a no-op (idempotent).
	Revoke(ctx context.Context, id string, at time.Time) error
	// MarkReplaced records that oldID was rotated into newID (used to
	// detect reuse of a stale token) and revokes oldID in the same step.
	MarkReplaced(ctx context.Context, oldID, newID string, at time.Time) error
	// RevokeAllForUser revokes every non-revoked refresh token belonging to
	// userID (used on logout-all and on reuse-detection).
	RevokeAllForUser(ctx context.Context, userID string, at time.Time) error
}
