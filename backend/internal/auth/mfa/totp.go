package mfa

import (
	"context"
	"errors"
	"fmt"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"

	"smusic/backend/internal/platform/clock"
)

// Issuer is the fixed issuer name embedded in every enrolled secret's
// otpauth:// URI — shown by authenticator apps (Google Authenticator,
// Authy, ...) to identify which service the code belongs to.
const Issuer = "smusic"

// TOTPChallenger is the production Challenger (security.md §2, RFC 6238).
// Every dependency is an interface, per backend-go.md §7: unit tests use
// an in-memory fake SecretRepository and never touch a real clock or
// Postgres.
type TOTPChallenger struct {
	secrets SecretRepository
	clock   clock.Clock
	// accountName resolves the human-readable label (usually an email)
	// shown alongside Issuer in the authenticator app — injected rather
	// than looked up here, so this package never depends on auth.Service
	// (which would be a package import cycle: auth depends on mfa.Challenger).
	accountName func(ctx context.Context, userID string) (string, error)
}

// NewTOTPChallenger returns a TOTPChallenger. accountName resolves userID
// to the label shown in the authenticator app (see field doc above) — pass
// a function backed by auth's own UserRepository.GetByID in production.
func NewTOTPChallenger(secrets SecretRepository, clk clock.Clock, accountName func(ctx context.Context, userID string) (string, error)) *TOTPChallenger {
	return &TOTPChallenger{secrets: secrets, clock: clk, accountName: accountName}
}

// Enroll generates a new TOTP secret for userID and persists it
// (unverified — see Verify), returning the base32 secret. Implements
// Challenger.
func (c *TOTPChallenger) Enroll(ctx context.Context, userID string) (string, error) {
	key, err := c.enroll(ctx, userID)
	if err != nil {
		return "", err
	}
	return key.Secret(), nil
}

// EnrollURI does the same enrollment as Enroll, additionally returning a
// ready-to-render otpauth:// URI (for a QR code) so the caller's HTTP
// handler doesn't need its own otp/totp dependency. Split from Enroll so
// Challenger's interface (secret string only) stays unchanged.
func (c *TOTPChallenger) EnrollURI(ctx context.Context, userID string) (secret string, otpauthURL string, err error) {
	key, err := c.enroll(ctx, userID)
	if err != nil {
		return "", "", err
	}
	return key.Secret(), key.String(), nil
}

// enroll generates and persists a new secret for userID. Enrolling does
// not activate the factor (VerifiedAt stays nil) — see Verify's doc
// comment.
func (c *TOTPChallenger) enroll(ctx context.Context, userID string) (*otp.Key, error) {
	account, err := c.accountName(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("mfa: resolve account name: %w", err)
	}

	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      Issuer,
		AccountName: account,
	})
	if err != nil {
		// coverage:ignore — totp.Generate only fails on a malformed
		// Issuer/AccountName (e.g. containing ":") or a crypto/rand read
		// failure; Issuer is a fixed constant and account is validated
		// upstream (a registered user's email), so this is not
		// reproducible in a hermetic unit test — same documented-
		// impossible-in-practice shape as password.Hasher.Hash's salt
		// generation (see internal/auth/password/argon2.go).
		return nil, fmt.Errorf("mfa: generate secret: %w", err)
	}

	if err := c.secrets.Upsert(ctx, Secret{UserID: userID, Value: key.Secret()}); err != nil {
		return nil, fmt.Errorf("mfa: store secret: %w", err)
	}
	return key, nil
}

// Verify checks code against userID's enrolled secret. The first
// successful Verify also activates the factor (MarkVerified) — Enroll
// alone leaves a factor pending, exactly like every mainstream TOTP setup
// flow (scan QR code, then confirm one code before it's "on"), so a typo'd
// or never-completed enrollment can't silently count as MFA being active.
func (c *TOTPChallenger) Verify(ctx context.Context, userID string, code string) (bool, error) {
	s, err := c.secrets.Get(ctx, userID)
	if err != nil {
		if errors.Is(err, ErrSecretNotFound) {
			return false, ErrSecretNotFound
		}
		return false, fmt.Errorf("mfa: get secret: %w", err)
	}

	ok, err := totp.ValidateCustom(code, s.Value, c.clock.Now(), totp.ValidateOpts{
		Period:    30,
		Skew:      1, // ±30s clock-skew tolerance, RFC 6238's own recommended default
		Digits:    otp.DigitsSix,
		Algorithm: otp.AlgorithmSHA1,
	})
	if err != nil {
		return false, fmt.Errorf("mfa: validate code: %w", err)
	}
	if !ok {
		return false, nil
	}

	if s.VerifiedAt == nil {
		now := c.clock.Now()
		if err := c.secrets.MarkVerified(ctx, userID, now); err != nil {
			return false, fmt.Errorf("mfa: mark verified: %w", err)
		}
	}
	return true, nil
}

// HasVerified reports whether userID has at least one activated (Verify'd
// at least once) TOTP factor — the check
// presence.SettingsService.GrantConsent uses (via auth.Service, which this
// type is wired into) to enforce security.md §2's "MFA obrigatório para
// habilitar a feature de proximidade".
func (c *TOTPChallenger) HasVerified(ctx context.Context, userID string) (bool, error) {
	s, err := c.secrets.Get(ctx, userID)
	if err != nil {
		if errors.Is(err, ErrSecretNotFound) {
			return false, nil
		}
		return false, fmt.Errorf("mfa: get secret: %w", err)
	}
	return s.VerifiedAt != nil, nil
}
