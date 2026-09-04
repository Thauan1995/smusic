// Package postgres implements presence's durable repository interfaces
// (PrivacySettingsRepository, BlockRepository, FollowChecker,
// AuditLogRepository) against Postgres via pgx. Per backend-go.md §7's
// testing pyramid, this package is exercised by integration tests against
// a real database (repo_integration_test.go, `//go:build integration`,
// run via `make test-integration`), not by the hermetic unit suite —
// coverage:ignore for the unit-coverage number, same carve-out as every
// other module's postgres package (00-overview.md §2). The business logic
// it fronts (consent validation,
// radius/reveal-level enforcement, the entire privacy filter pipeline)
// lives in internal/presence's service files and is unit-tested there with
// in-memory fakes implementing these same interfaces.
package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"smusic/backend/internal/presence"
)

// Repo implements presence.PrivacySettingsRepository, presence.BlockRepository,
// presence.FollowChecker and presence.AuditLogRepository against a single
// Postgres pool.
type Repo struct {
	pool *pgxpool.Pool
}

// New returns a Repo backed by pool.
func New(pool *pgxpool.Pool) *Repo {
	return &Repo{pool: pool}
}

// --- PrivacySettingsRepository ---------------------------------------------

func (r *Repo) Get(ctx context.Context, userID string) (presence.PrivacySettings, error) {
	const q = `
		SELECT user_id, presence_visibility, presence_share_track,
		       proximity_consent_enabled, proximity_consent_ts, proximity_consent_renew_due,
		       visibility_radius_m, reveal_level, paused_bool, created_at, updated_at
		FROM user_privacy_settings WHERE user_id = $1`
	var s presence.PrivacySettings
	err := r.pool.QueryRow(ctx, q, userID).Scan(
		&s.UserID, &s.PresenceVisibility, &s.PresenceShareTrack,
		&s.ProximityConsentEnabled, &s.ProximityConsentTS, &s.ProximityConsentRenewDue,
		&s.VisibilityRadiusM, &s.RevealLevel, &s.Paused, &s.CreatedAt, &s.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return presence.PrivacySettings{}, presence.ErrSettingsNotFound
	}
	if err != nil {
		return presence.PrivacySettings{}, err
	}
	return s, nil
}

func (r *Repo) Upsert(ctx context.Context, s presence.PrivacySettings) error {
	const q = `
		INSERT INTO user_privacy_settings (
			user_id, presence_visibility, presence_share_track,
			proximity_consent_enabled, proximity_consent_ts, proximity_consent_renew_due,
			visibility_radius_m, reveal_level, paused_bool, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, now(), now())
		ON CONFLICT (user_id) DO UPDATE SET
			presence_visibility = EXCLUDED.presence_visibility,
			presence_share_track = EXCLUDED.presence_share_track,
			proximity_consent_enabled = EXCLUDED.proximity_consent_enabled,
			proximity_consent_ts = EXCLUDED.proximity_consent_ts,
			proximity_consent_renew_due = EXCLUDED.proximity_consent_renew_due,
			visibility_radius_m = EXCLUDED.visibility_radius_m,
			reveal_level = EXCLUDED.reveal_level,
			paused_bool = EXCLUDED.paused_bool,
			updated_at = now()`
	_, err := r.pool.Exec(ctx, q,
		s.UserID, s.PresenceVisibility, s.PresenceShareTrack,
		s.ProximityConsentEnabled, s.ProximityConsentTS, s.ProximityConsentRenewDue,
		s.VisibilityRadiusM, s.RevealLevel, s.Paused,
	)
	return err
}

// --- BlockRepository ---------------------------------------------------------

func (r *Repo) Block(ctx context.Context, blockerID, blockedID string) error {
	const q = `
		INSERT INTO user_blocks (blocker_id, blocked_id, created_at)
		VALUES ($1, $2, now())
		ON CONFLICT (blocker_id, blocked_id) DO NOTHING`
	_, err := r.pool.Exec(ctx, q, blockerID, blockedID)
	if isCheckViolation(err) {
		return presence.ErrCannotBlockSelf
	}
	return err
}

func (r *Repo) Unblock(ctx context.Context, blockerID, blockedID string) error {
	const q = `DELETE FROM user_blocks WHERE blocker_id = $1 AND blocked_id = $2`
	_, err := r.pool.Exec(ctx, q, blockerID, blockedID)
	return err
}

func (r *Repo) IsBlockedEitherWay(ctx context.Context, a, b string) (bool, error) {
	const q = `
		SELECT EXISTS(
			SELECT 1 FROM user_blocks
			WHERE (blocker_id = $1 AND blocked_id = $2)
			   OR (blocker_id = $2 AND blocked_id = $1)
		)`
	var exists bool
	err := r.pool.QueryRow(ctx, q, a, b).Scan(&exists)
	return exists, err
}

// --- FollowChecker (data-architecture.md §1.6's `follows` table) ------------

func (r *Repo) IsMutualFollow(ctx context.Context, a, b string) (bool, error) {
	const q = `
		SELECT
			EXISTS(SELECT 1 FROM follows WHERE follower_id = $1 AND followee_id = $2)
			AND
			EXISTS(SELECT 1 FROM follows WHERE follower_id = $2 AND followee_id = $1)`
	var mutual bool
	err := r.pool.QueryRow(ctx, q, a, b).Scan(&mutual)
	return mutual, err
}

// --- AuditLogRepository (append-only, security.md §1.8) ---------------------

func (r *Repo) Append(ctx context.Context, e presence.AuditLogEntry) error {
	const q = `
		INSERT INTO presence_audit_log (id, requester_id, target_id, occurred_at, distance_bucket, endpoint)
		VALUES ($1, $2, $3, $4, $5, $6)`
	_, err := r.pool.Exec(ctx, q, e.ID, e.RequesterID, e.TargetID, e.OccurredAt, int(e.Bucket), e.Endpoint)
	if err != nil {
		return fmt.Errorf("presence/postgres: append audit log: %w", err)
	}
	return nil
}

func isCheckViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgerrcode.CheckViolation
}
