// Package mfa defines the interface for second-factor authentication.
// Per the task's explicit instruction, this slice ships the interface only
// — not an implementation — because Fatia 1 has no feature that requires
// MFA: security.md §2 mandates TOTP for the proximity feature (Fatia 2,
// explicitly out of scope) and for sensitive-account actions (password/
// email change, data export, active-session management) that also aren't
// part of this slice's endpoint set (backend-go.md §4, as scoped down to
// auth/catalog/library/playback).
//
// TODO(security.md §2): implement TOTP (RFC 6238), e.g. via
// pquerna/otp: Enroll generates a secret + otpauth:// URI for a QR code;
// Verify checks a submitted code against the stored secret with a small
// time-skew window. Wire Challenger into auth.Service for step-up flows
// once a feature that needs it (proximity, or sensitive actions) ships.
package mfa

import (
	"context"
	"errors"
)

// ErrNotImplemented is returned by NoopChallenger for every call.
var ErrNotImplemented = errors.New("mfa: not implemented in this slice")

// Challenger prepares and verifies a second authentication factor for a
// user.
type Challenger interface {
	// Enroll provisions a new second factor for userID and returns
	// provider-specific enrollment data (e.g. a TOTP secret).
	Enroll(ctx context.Context, userID string) (secret string, err error)
	// Verify checks a submitted code against userID's enrolled factor.
	Verify(ctx context.Context, userID string, code string) (bool, error)
}

// NoopChallenger is the Challenger wired in for this slice: it always
// reports ErrNotImplemented, keeping the interface's call sites (once they
// exist) ready to be pointed at a real implementation without further
// wiring changes.
type NoopChallenger struct{}

// Enroll always fails; see package doc.
func (NoopChallenger) Enroll(context.Context, string) (string, error) {
	return "", ErrNotImplemented
}

// Verify always fails; see package doc.
func (NoopChallenger) Verify(context.Context, string, string) (bool, error) {
	return false, ErrNotImplemented
}
