// Package mfa implements TOTP (RFC 6238) second-factor authentication.
// security.md §2 mandates it for the proximity feature (Fatia 2 — see
// TOTPChallenger, wired into auth.Service and required by
// presence.SettingsService.GrantConsent, per
// .vibeflow/specs/mfa-for-proximity-consent.md) and for sensitive-account
// actions (password/email change, data export, active-session
// management); only the proximity-consent call site is wired in this
// slice — the others remain a documented follow-up, same as before.
package mfa

import (
	"context"
	"errors"
)

// ErrNotImplemented is returned by NoopChallenger for every call.
var ErrNotImplemented = errors.New("mfa: not implemented in this slice")

// Sentinel errors for TOTPChallenger.
var (
	ErrSecretNotFound = errors.New("mfa: no factor enrolled for this user")
	ErrInvalidCode    = errors.New("mfa: invalid or expired code")
)

// Challenger prepares and verifies a second authentication factor for a
// user.
type Challenger interface {
	// Enroll provisions a new second factor for userID and returns
	// provider-specific enrollment data (e.g. a TOTP secret).
	Enroll(ctx context.Context, userID string) (secret string, err error)
	// Verify checks a submitted code against userID's enrolled factor.
	Verify(ctx context.Context, userID string, code string) (bool, error)
}

// NoopChallenger always reports ErrNotImplemented — useful as a test
// double, or wired in place of TOTPChallenger for a call site that
// deliberately has no MFA requirement (nothing in this codebase does
// today; TOTPChallenger is wired for every Challenger use — see
// cmd/server/main.go).
type NoopChallenger struct{}

// Enroll always fails; see package doc.
func (NoopChallenger) Enroll(context.Context, string) (string, error) {
	return "", ErrNotImplemented
}

// Verify always fails; see package doc.
func (NoopChallenger) Verify(context.Context, string, string) (bool, error) {
	return false, ErrNotImplemented
}
