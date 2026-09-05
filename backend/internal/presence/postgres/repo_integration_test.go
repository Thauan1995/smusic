//go:build integration

// See .vibeflow/specs/backend-integration-test-coverage.md.
package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	authpg "smusic/backend/internal/auth/postgres"
	"smusic/backend/internal/platform/dbx/dbxtest"
	"smusic/backend/internal/platform/idgen"
	"smusic/backend/internal/presence"

	"smusic/backend/internal/auth"
)

func newID() string { return idgen.UUIDv7{}.NewID() }

func TestIntegration_PrivacySettingsRepo(t *testing.T) {
	pool := dbxtest.NewPool(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)

	userID := newID()
	require.NoError(t, authpg.New(pool).Create(ctx, auth.User{
		ID: userID, Email: "settings-int@example.com", DisplayName: "Settings User",
		Status: auth.UserStatusActive, Role: auth.RoleUser, CreatedAt: now, UpdatedAt: now,
	}))

	r := New(pool)

	_, err := r.Get(ctx, userID)
	assert.ErrorIs(t, err, presence.ErrSettingsNotFound)

	s := presence.DefaultPrivacySettings(userID)
	s.ProximityConsentEnabled = true
	s.ProximityConsentTS = &now
	due := now.Add(presence.ConsentValidityPeriod)
	s.ProximityConsentRenewDue = &due
	s.VisibilityRadiusM = 5000
	s.Paused = false
	s.UpdatedAt = now
	require.NoError(t, r.Upsert(ctx, s))

	got, err := r.Get(ctx, userID)
	require.NoError(t, err)
	assert.True(t, got.ProximityConsentEnabled)
	assert.Equal(t, 5000, got.VisibilityRadiusM)
	assert.False(t, got.Paused)

	// Upsert again (update path, not just insert).
	got.Paused = true
	got.UpdatedAt = now
	require.NoError(t, r.Upsert(ctx, got))
	got2, err := r.Get(ctx, userID)
	require.NoError(t, err)
	assert.True(t, got2.Paused)
}

func TestIntegration_BlockRepo(t *testing.T) {
	pool := dbxtest.NewPool(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)

	authRepo := authpg.New(pool)
	blockerID, blockedID := newID(), newID()
	require.NoError(t, authRepo.Create(ctx, auth.User{ID: blockerID, Email: "blocker@example.com", DisplayName: "B1", Status: auth.UserStatusActive, Role: auth.RoleUser, CreatedAt: now, UpdatedAt: now}))
	require.NoError(t, authRepo.Create(ctx, auth.User{ID: blockedID, Email: "blocked@example.com", DisplayName: "B2", Status: auth.UserStatusActive, Role: auth.RoleUser, CreatedAt: now, UpdatedAt: now}))

	r := New(pool)

	blocked, err := r.IsBlockedEitherWay(ctx, blockerID, blockedID)
	require.NoError(t, err)
	assert.False(t, blocked)

	require.NoError(t, r.Block(ctx, blockerID, blockedID))

	blocked, err = r.IsBlockedEitherWay(ctx, blockerID, blockedID)
	require.NoError(t, err)
	assert.True(t, blocked)
	// Symmetric: checking in the reverse order must also report blocked.
	blocked, err = r.IsBlockedEitherWay(ctx, blockedID, blockerID)
	require.NoError(t, err)
	assert.True(t, blocked)

	require.NoError(t, r.Unblock(ctx, blockerID, blockedID))
	blocked, err = r.IsBlockedEitherWay(ctx, blockerID, blockedID)
	require.NoError(t, err)
	assert.False(t, blocked)

	// Unblock is idempotent: unblocking an already-unblocked pair is not an error.
	require.NoError(t, r.Unblock(ctx, blockerID, blockedID))
}

func TestIntegration_FollowChecker(t *testing.T) {
	pool := dbxtest.NewPool(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)

	authRepo := authpg.New(pool)
	a, b := newID(), newID()
	require.NoError(t, authRepo.Create(ctx, auth.User{ID: a, Email: "follow-a@example.com", DisplayName: "A", Status: auth.UserStatusActive, Role: auth.RoleUser, CreatedAt: now, UpdatedAt: now}))
	require.NoError(t, authRepo.Create(ctx, auth.User{ID: b, Email: "follow-b@example.com", DisplayName: "B", Status: auth.UserStatusActive, Role: auth.RoleUser, CreatedAt: now, UpdatedAt: now}))

	r := New(pool)

	mutual, err := r.IsMutualFollow(ctx, a, b)
	require.NoError(t, err)
	assert.False(t, mutual)

	_, err = pool.Exec(ctx, `INSERT INTO follows (follower_id, followee_id) VALUES ($1, $2)`, a, b)
	require.NoError(t, err)

	// One-directional follow is not mutual.
	mutual, err = r.IsMutualFollow(ctx, a, b)
	require.NoError(t, err)
	assert.False(t, mutual)

	_, err = pool.Exec(ctx, `INSERT INTO follows (follower_id, followee_id) VALUES ($1, $2)`, b, a)
	require.NoError(t, err)

	mutual, err = r.IsMutualFollow(ctx, a, b)
	require.NoError(t, err)
	assert.True(t, mutual)
}

func TestIntegration_AuditLogRepo(t *testing.T) {
	pool := dbxtest.NewPool(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)

	authRepo := authpg.New(pool)
	requester, target := newID(), newID()
	require.NoError(t, authRepo.Create(ctx, auth.User{ID: requester, Email: "audit-req@example.com", DisplayName: "R", Status: auth.UserStatusActive, Role: auth.RoleUser, CreatedAt: now, UpdatedAt: now}))
	require.NoError(t, authRepo.Create(ctx, auth.User{ID: target, Email: "audit-target@example.com", DisplayName: "T", Status: auth.UserStatusActive, Role: auth.RoleUser, CreatedAt: now, UpdatedAt: now}))

	r := New(pool)
	entry := presence.AuditLogEntry{
		ID: newID(), RequesterID: requester, TargetID: target,
		OccurredAt: now, Bucket: presence.Bucket2, Endpoint: "/v1/presence/connect",
	}
	require.NoError(t, r.Append(ctx, entry))

	// Immutability (security.md §1.8): the BEFORE UPDATE/DELETE triggers
	// from migrations/0002_presence.up.sql must reject any mutation, even
	// a raw SQL statement bypassing the repo entirely — this is the one
	// integration test in this file that's really exercising the
	// migration's trigger, not just the repo's Go code.
	_, err := pool.Exec(ctx, `UPDATE presence_audit_log SET endpoint = 'tampered' WHERE id = $1`, entry.ID)
	require.Error(t, err, "presence_audit_log must be append-only")

	_, err = pool.Exec(ctx, `DELETE FROM presence_audit_log WHERE id = $1`, entry.ID)
	require.Error(t, err, "presence_audit_log must be append-only")
}
