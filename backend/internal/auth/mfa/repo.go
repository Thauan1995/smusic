package mfa

import (
	"context"
	"time"
)

// Secret is one user's enrolled TOTP factor.
type Secret struct {
	UserID     string
	Value      string // base32-encoded TOTP seed
	VerifiedAt *time.Time // nil until the first successful Verify
}

// SecretRepository persists a user's TOTP secret (Postgres user_mfa_totp —
// see migrations/0003_mfa.up.sql). One secret per user in this slice
// (re-enrolling overwrites the previous, unverified-or-not, secret —
// matches how a user re-scanning a QR code in an authenticator app
// expects re-enrollment to behave).
type SecretRepository interface {
	// Get returns ErrSecretNotFound if userID has never enrolled.
	Get(ctx context.Context, userID string) (Secret, error)
	// Upsert creates or replaces userID's secret.
	Upsert(ctx context.Context, s Secret) error
	// MarkVerified records that userID's currently-enrolled secret has
	// been confirmed with at least one valid code (Enroll alone does not
	// activate a factor — see TOTPChallenger.Verify).
	MarkVerified(ctx context.Context, userID string, verifiedAt time.Time) error
}
