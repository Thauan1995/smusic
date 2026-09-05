//go:build integration

// See .vibeflow/specs/backend-integration-test-coverage.md: this package's
// unit-coverage exclusion (repo.go's package doc) is closed by this file,
// run separately via `make test-integration` against a real, ephemeral
// Postgres (dbxtest.NewPool) — never in the fast unit-test loop.
package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"smusic/backend/internal/auth"
	"smusic/backend/internal/platform/dbx/dbxtest"
	"smusic/backend/internal/platform/idgen"
)

func newID() string { return idgen.UUIDv7{}.NewID() }

func TestIntegration_Repo_Users(t *testing.T) {
	pool := dbxtest.NewPool(t)
	r := New(pool)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)

	u := auth.User{
		ID: newID(), Email: "int-test@example.com", PasswordHash: "hashed:x",
		DisplayName: "Integration Test", Status: auth.UserStatusActive, Role: auth.RoleUser,
		CreatedAt: now, UpdatedAt: now,
	}
	require.NoError(t, r.Create(ctx, u))

	// Create: duplicate email is a unique violation mapped to ErrEmailTaken.
	dup := u
	dup.ID = newID()
	err := r.Create(ctx, dup)
	assert.ErrorIs(t, err, auth.ErrEmailTaken)

	// GetByEmail: found, case-insensitive (CITEXT), and not-found.
	got, err := r.GetByEmail(ctx, "INT-test@example.com")
	require.NoError(t, err)
	assert.Equal(t, u.ID, got.ID)
	assert.Equal(t, u.DisplayName, got.DisplayName)

	_, err = r.GetByEmail(ctx, "nobody@example.com")
	assert.ErrorIs(t, err, auth.ErrUserNotFound)

	// GetByID: found and not-found.
	got, err = r.GetByID(ctx, u.ID)
	require.NoError(t, err)
	assert.Equal(t, u.Email, got.Email)

	_, err = r.GetByID(ctx, newID())
	assert.ErrorIs(t, err, auth.ErrUserNotFound)
}

func TestIntegration_Repo_Identities(t *testing.T) {
	pool := dbxtest.NewPool(t)
	r := New(pool)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)

	userID := newID()
	require.NoError(t, r.Create(ctx, auth.User{
		ID: userID, Email: "oauth-int@example.com", DisplayName: "OAuth User",
		Status: auth.UserStatusActive, Role: auth.RoleUser, CreatedAt: now, UpdatedAt: now,
	}))

	// GetUserIDByProvider: not linked yet.
	_, err := r.GetUserIDByProvider(ctx, "google", "subject-1")
	assert.ErrorIs(t, err, auth.ErrIdentityNotLinked)

	require.NoError(t, r.Link(ctx, auth.Identity{
		ID: newID(), UserID: userID, Provider: "google", ProviderUserID: "subject-1", CreatedAt: now,
	}))

	found, err := r.GetUserIDByProvider(ctx, "google", "subject-1")
	require.NoError(t, err)
	assert.Equal(t, userID, found)
}

func TestIntegration_Repo_Devices(t *testing.T) {
	pool := dbxtest.NewPool(t)
	r := New(pool)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)

	userID := newID()
	require.NoError(t, r.Create(ctx, auth.User{
		ID: userID, Email: "device-int@example.com", DisplayName: "Device User",
		Status: auth.UserStatusActive, Role: auth.RoleUser, CreatedAt: now, UpdatedAt: now,
	}))

	id1, err := r.Upsert(ctx, auth.Device{UserID: userID, Platform: "ios", AppVersion: "1.0", LastSeenAt: now})
	require.NoError(t, err)
	assert.NotEmpty(t, id1)

	// Upsert on the same (user, platform) pair updates in place, per the
	// table's UNIQUE(user_id, platform) — same device ID back.
	id2, err := r.Upsert(ctx, auth.Device{UserID: userID, Platform: "ios", AppVersion: "1.1", LastSeenAt: now})
	require.NoError(t, err)
	assert.Equal(t, id1, id2)
}

func TestIntegration_Repo_RefreshTokens(t *testing.T) {
	pool := dbxtest.NewPool(t)
	r := New(pool)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)

	userID := newID()
	require.NoError(t, r.Create(ctx, auth.User{
		ID: userID, Email: "rt-int@example.com", DisplayName: "RT User",
		Status: auth.UserStatusActive, Role: auth.RoleUser, CreatedAt: now, UpdatedAt: now,
	}))

	rt := auth.RefreshToken{
		ID: newID(), UserID: userID, TokenHash: "hash-1",
		IssuedAt: now, ExpiresAt: now.Add(24 * time.Hour),
	}
	require.NoError(t, r.Store(ctx, rt))

	got, err := r.GetByHash(ctx, "hash-1")
	require.NoError(t, err)
	assert.Equal(t, rt.ID, got.ID)
	assert.Nil(t, got.RevokedAt)
	assert.Empty(t, got.ReplacedBy)

	_, err = r.GetByHash(ctx, "no-such-hash")
	assert.ErrorIs(t, err, auth.ErrInvalidRefreshToken)

	// MarkReplaced: rotates rt into a new token, revoking rt in the process
	// (security.md §2's rotation-on-every-use + reuse-detection model).
	newRT := auth.RefreshToken{
		ID: newID(), UserID: userID, TokenHash: "hash-2",
		IssuedAt: now, ExpiresAt: now.Add(24 * time.Hour),
	}
	require.NoError(t, r.Store(ctx, newRT))
	require.NoError(t, r.MarkReplaced(ctx, rt.ID, newRT.ID, now))

	got, err = r.GetByHash(ctx, "hash-1")
	require.NoError(t, err)
	assert.Equal(t, newRT.ID, got.ReplacedBy)

	// Revoke: idempotent single-token revocation.
	require.NoError(t, r.Revoke(ctx, newRT.ID, now))
	got, err = r.GetByHash(ctx, "hash-2")
	require.NoError(t, err)
	require.NotNil(t, got.RevokedAt)

	// RevokeAllForUser: a third token, revoked in bulk.
	thirdRT := auth.RefreshToken{
		ID: newID(), UserID: userID, TokenHash: "hash-3",
		IssuedAt: now, ExpiresAt: now.Add(24 * time.Hour),
	}
	require.NoError(t, r.Store(ctx, thirdRT))
	require.NoError(t, r.RevokeAllForUser(ctx, userID, now))
	got, err = r.GetByHash(ctx, "hash-3")
	require.NoError(t, err)
	assert.NotNil(t, got.RevokedAt)
}
