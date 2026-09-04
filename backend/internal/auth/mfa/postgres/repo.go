// Package postgres implements mfa.SecretRepository against Postgres via
// pgx. Per backend-go.md §7's testing pyramid, this package is exercised
// by integration tests against a real database (see
// .vibeflow/specs/backend-integration-test-coverage.md — not yet built at
// the time this file was written; coverage:ignore for the unit-coverage
// number in the meantime, documented here per 00-overview.md §2's rule
// that exclusions must be explicit), not by the hermetic unit suite. The
// business logic it fronts (enrollment, verification, activation) lives
// in mfa.TOTPChallenger and is covered there with an in-memory fake
// implementing mfa.SecretRepository.
package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"smusic/backend/internal/auth/mfa"
)

// Repo implements mfa.SecretRepository against a single Postgres pool.
type Repo struct {
	pool *pgxpool.Pool
}

// New returns a Repo backed by pool.
func New(pool *pgxpool.Pool) *Repo {
	return &Repo{pool: pool}
}

func (r *Repo) Get(ctx context.Context, userID string) (mfa.Secret, error) {
	const q = `SELECT user_id, secret, verified_at FROM user_mfa_totp WHERE user_id = $1`
	var s mfa.Secret
	err := r.pool.QueryRow(ctx, q, userID).Scan(&s.UserID, &s.Value, &s.VerifiedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return mfa.Secret{}, mfa.ErrSecretNotFound
	}
	return s, err
}

func (r *Repo) Upsert(ctx context.Context, s mfa.Secret) error {
	const q = `
		INSERT INTO user_mfa_totp (user_id, secret, verified_at, updated_at)
		VALUES ($1, $2, NULL, now())
		ON CONFLICT (user_id) DO UPDATE SET secret = $2, verified_at = NULL, updated_at = now()`
	_, err := r.pool.Exec(ctx, q, s.UserID, s.Value)
	return err
}

func (r *Repo) MarkVerified(ctx context.Context, userID string, verifiedAt time.Time) error {
	const q = `UPDATE user_mfa_totp SET verified_at = $2, updated_at = now() WHERE user_id = $1`
	tag, err := r.pool.Exec(ctx, q, userID, verifiedAt)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return mfa.ErrSecretNotFound
	}
	return nil
}
