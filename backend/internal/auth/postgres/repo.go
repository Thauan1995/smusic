// Package postgres implements auth's repository interfaces against
// Postgres via pgx. Per backend-go.md §7's testing pyramid, this package is
// exercised by integration tests against a real database (see
// repo_integration_test.go, build-tagged `integration`), not by the
// hermetic unit suite — coverage:ignore for the unit-coverage number,
// documented here per 00-overview.md §2's rule that exclusions must be
// explicit. The business logic it fronts (validation, session issuance,
// rotation, reuse detection) lives in auth.Service and is covered there
// with in-memory fakes implementing the same interfaces this file
// implements against Postgres.
package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"smusic/backend/internal/auth"
)

// Repo implements auth.UserRepository, auth.IdentityRepository,
// auth.DeviceRepository and auth.RefreshTokenRepository against a single
// Postgres pool.
type Repo struct {
	pool *pgxpool.Pool
}

// New returns a Repo backed by pool.
func New(pool *pgxpool.Pool) *Repo {
	return &Repo{pool: pool}
}

func (r *Repo) Create(ctx context.Context, u auth.User) error {
	const q = `
		INSERT INTO users (id, email, password_hash, display_name, status, role, created_at, updated_at)
		VALUES ($1, $2, NULLIF($3, ''), $4, $5, $6, $7, $8)`
	_, err := r.pool.Exec(ctx, q, u.ID, u.Email, u.PasswordHash, u.DisplayName, u.Status, u.Role, u.CreatedAt, u.UpdatedAt)
	if isUniqueViolation(err) {
		return auth.ErrEmailTaken
	}
	return err
}

func (r *Repo) GetByEmail(ctx context.Context, email string) (auth.User, error) {
	const q = `
		SELECT id, email, COALESCE(password_hash, ''), display_name, COALESCE(handle, ''), status, role, created_at, updated_at
		FROM users WHERE email = $1 AND deleted_at IS NULL`
	return r.scanUser(r.pool.QueryRow(ctx, q, email))
}

func (r *Repo) GetByID(ctx context.Context, id string) (auth.User, error) {
	const q = `
		SELECT id, email, COALESCE(password_hash, ''), display_name, COALESCE(handle, ''), status, role, created_at, updated_at
		FROM users WHERE id = $1 AND deleted_at IS NULL`
	return r.scanUser(r.pool.QueryRow(ctx, q, id))
}

func (r *Repo) scanUser(row pgx.Row) (auth.User, error) {
	var u auth.User
	err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.DisplayName, &u.Handle, &u.Status, &u.Role, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.User{}, auth.ErrUserNotFound
	}
	return u, err
}

func (r *Repo) GetUserIDByProvider(ctx context.Context, provider, providerUserID string) (string, error) {
	const q = `SELECT user_id FROM user_auth_identities WHERE provider = $1 AND provider_user_id = $2`
	var userID string
	err := r.pool.QueryRow(ctx, q, provider, providerUserID).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", auth.ErrIdentityNotLinked
	}
	return userID, err
}

func (r *Repo) Link(ctx context.Context, identity auth.Identity) error {
	const q = `
		INSERT INTO user_auth_identities (id, user_id, provider, provider_user_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $5)`
	_, err := r.pool.Exec(ctx, q, identity.ID, identity.UserID, identity.Provider, identity.ProviderUserID, identity.CreatedAt)
	return err
}

func (r *Repo) Upsert(ctx context.Context, d auth.Device) (string, error) {
	const q = `
		INSERT INTO user_devices (id, user_id, platform, push_token, app_version, last_seen_at, created_at, updated_at)
		VALUES (gen_random_uuid(), $1, $2, NULLIF($3, ''), NULLIF($4, ''), $5, $5, $5)
		ON CONFLICT (user_id, platform) DO UPDATE
			SET push_token = EXCLUDED.push_token,
				app_version = EXCLUDED.app_version,
				last_seen_at = EXCLUDED.last_seen_at,
				updated_at = EXCLUDED.updated_at
		RETURNING id`
	var id string
	err := r.pool.QueryRow(ctx, q, d.UserID, d.Platform, d.PushToken, d.AppVersion, d.LastSeenAt).Scan(&id)
	return id, err
}

func (r *Repo) Store(ctx context.Context, rt auth.RefreshToken) error {
	const q = `
		INSERT INTO refresh_tokens (id, user_id, device_id, token_hash, issued_at, expires_at, created_at, updated_at)
		VALUES ($1, $2, NULLIF($3, '')::uuid, $4, $5, $6, $5, $5)`
	_, err := r.pool.Exec(ctx, q, rt.ID, rt.UserID, rt.DeviceID, rt.TokenHash, rt.IssuedAt, rt.ExpiresAt)
	return err
}

func (r *Repo) GetByHash(ctx context.Context, tokenHash string) (auth.RefreshToken, error) {
	const q = `
		SELECT id, user_id, COALESCE(device_id::text, ''), token_hash, issued_at, expires_at, revoked_at, COALESCE(replaced_by::text, '')
		FROM refresh_tokens WHERE token_hash = $1`
	var rt auth.RefreshToken
	err := r.pool.QueryRow(ctx, q, tokenHash).Scan(
		&rt.ID, &rt.UserID, &rt.DeviceID, &rt.TokenHash, &rt.IssuedAt, &rt.ExpiresAt, &rt.RevokedAt, &rt.ReplacedBy,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.RefreshToken{}, auth.ErrInvalidRefreshToken
	}
	return rt, err
}

func (r *Repo) Revoke(ctx context.Context, id string, at time.Time) error {
	const q = `UPDATE refresh_tokens SET revoked_at = $2, updated_at = $2 WHERE id = $1 AND revoked_at IS NULL`
	_, err := r.pool.Exec(ctx, q, id, at)
	return err
}

func (r *Repo) MarkReplaced(ctx context.Context, oldID, newID string, at time.Time) error {
	const q = `UPDATE refresh_tokens SET replaced_by = $2, revoked_at = $3, updated_at = $3 WHERE id = $1`
	_, err := r.pool.Exec(ctx, q, oldID, newID, at)
	return err
}

func (r *Repo) RevokeAllForUser(ctx context.Context, userID string, at time.Time) error {
	const q = `UPDATE refresh_tokens SET revoked_at = $2, updated_at = $2 WHERE user_id = $1 AND revoked_at IS NULL`
	_, err := r.pool.Exec(ctx, q, userID, at)
	return err
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation
}
