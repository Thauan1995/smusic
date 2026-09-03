// Package oauth defines the boundary for verifying third-party OIDC ID
// tokens (Google/Apple — security.md §2: "Suporte a login social ... via
// OIDC federado"). Only the interface and a stub are implemented in this
// slice, as instructed: real verification (JWKS fetch, signature/audience/
// issuer/nonce validation, key rotation handling) is deferred.
//
// TODO(security.md §2): implement real verification for Google and Apple:
//   - Google: fetch https://www.googleapis.com/oauth2/v3/certs (JWKS),
//     validate iss=="https://accounts.google.com", aud==client_id, exp/iat.
//   - Apple: fetch https://appleid.apple.com/auth/keys, validate
//     iss=="https://appleid.apple.com", aud==client_id (or Services ID).
//
// Until then, LoginWithOAuth in internal/auth/service.go will always fail
// with ErrNotImplemented — the endpoint exists and is wired end-to-end so
// swapping StubVerifier for a real implementation is the only change
// needed to turn OAuth login on.
package oauth

import (
	"context"
	"errors"
)

// Provider identifies a supported OIDC provider.
type Provider string

// Supported providers (security.md §2 requires Google/Apple for app-store
// compliance).
const (
	ProviderGoogle Provider = "google"
	ProviderApple  Provider = "apple"
)

// ErrNotImplemented is returned by StubVerifier for every call.
var ErrNotImplemented = errors.New("oauth: provider verification not implemented in this slice")

// ErrUnsupportedProvider is returned for a provider value outside the
// supported set.
var ErrUnsupportedProvider = errors.New("oauth: unsupported provider")

// Verifier verifies a provider's OIDC ID token and returns the stable,
// provider-scoped subject identifier and the verified email address (if
// the provider's token includes one).
type Verifier interface {
	Verify(ctx context.Context, provider Provider, idToken string) (subject, email string, err error)
}

// StubVerifier is the placeholder Verifier wired in for this slice. It
// validates the provider is one we claim to support and then reports
// ErrNotImplemented, distinguishing "unsupported provider" (a client bug)
// from "not implemented yet" (a known, documented gap) at the API layer.
type StubVerifier struct{}

// Verify always fails; see package doc.
func (StubVerifier) Verify(_ context.Context, provider Provider, _ string) (string, string, error) {
	switch provider {
	case ProviderGoogle, ProviderApple:
		return "", "", ErrNotImplemented
	default:
		return "", "", ErrUnsupportedProvider
	}
}
