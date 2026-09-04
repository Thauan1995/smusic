//go:build integration

// See .vibeflow/specs/backend-integration-test-coverage.md. This package
// didn't exist when that spec was written (mfa-for-proximity-consent
// landed afterward) but falls under the same gap — every
// internal/*/postgres/*.go repo, unit-covered nowhere, gets an
// integration test here too.
package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"smusic/backend/internal/auth/mfa"
	authpg "smusic/backend/internal/auth/postgres"
	"smusic/backend/internal/platform/dbx/dbxtest"
	"smusic/backend/internal/platform/idgen"

	"smusic/backend/internal/auth"
)

func TestIntegration_SecretRepo(t *testing.T) {
	pool := dbxtest.NewPool(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)

	userID := idgen.UUIDv7{}.NewID()
	require.NoError(t, authpg.New(pool).Create(ctx, auth.User{
		ID: userID, Email: "mfa-int@example.com", DisplayName: "MFA User",
		Status: auth.UserStatusActive, CreatedAt: now, UpdatedAt: now,
	}))

	r := New(pool)

	_, err := r.Get(ctx, userID)
	assert.ErrorIs(t, err, mfa.ErrSecretNotFound)

	require.NoError(t, r.Upsert(ctx, mfa.Secret{UserID: userID, Value: "SEEDSEEDSEEDSEED"}))
	got, err := r.Get(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, "SEEDSEEDSEEDSEED", got.Value)
	assert.Nil(t, got.VerifiedAt)

	require.NoError(t, r.MarkVerified(ctx, userID, now))
	got, err = r.Get(ctx, userID)
	require.NoError(t, err)
	require.NotNil(t, got.VerifiedAt)
	// .Equal, not assert.Equal: pgx scans TIMESTAMPTZ back in the
	// process's local location, not necessarily UTC — same instant,
	// different *time.Location, which assert.Equal's deep-equality would
	// (wrongly) flag as a mismatch.
	assert.True(t, now.Equal(*got.VerifiedAt), "want %v, got %v", now, *got.VerifiedAt)

	// Re-enrolling (Upsert again) resets VerifiedAt to nil — a fresh QR
	// scan must require a fresh confirmation, per TOTPChallenger's own
	// enroll-then-verify contract.
	require.NoError(t, r.Upsert(ctx, mfa.Secret{UserID: userID, Value: "NEWSECRETNEWSECR"}))
	got, err = r.Get(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, "NEWSECRETNEWSECR", got.Value)
	assert.Nil(t, got.VerifiedAt)

	// MarkVerified on a never-enrolled user reports ErrSecretNotFound.
	err = r.MarkVerified(ctx, idgen.UUIDv7{}.NewID(), now)
	assert.ErrorIs(t, err, mfa.ErrSecretNotFound)
}
