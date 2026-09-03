// Package token implements the two halves of security.md §2's session
// model: short-lived signed JWT access tokens (this file) and opaque,
// hashable, revocable refresh tokens (refresh.go).
package token

import (
	"crypto/ed25519"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"smusic/backend/internal/platform/clock"
)

// Sentinel errors for access-token verification.
var (
	ErrInvalidToken = errors.New("token: invalid access token")
	ErrExpiredToken = errors.New("token: access token expired")
)

// claims is the JWT payload. Only the registered "sub" (subject = user ID)
// and standard timing claims are used — no PII in the token itself
// (backend-go.md §5: never log/carry more than necessary).
type claims struct {
	jwt.RegisteredClaims
}

// Signer issues and verifies access tokens. It implements
// internal/platform/middleware.Authenticator.
type Signer struct {
	priv   ed25519.PrivateKey
	pub    ed25519.PublicKey
	issuer string
	ttl    time.Duration
	clock  clock.Clock
}

// NewSigner returns a Signer. TTL should be short per security.md §2
// (10-15 min).
func NewSigner(priv ed25519.PrivateKey, pub ed25519.PublicKey, issuer string, ttl time.Duration, clk clock.Clock) *Signer {
	return &Signer{priv: priv, pub: pub, issuer: issuer, ttl: ttl, clock: clk}
}

// Sign issues a new access token for userID, returning the encoded token
// and its expiry instant.
func (s *Signer) Sign(userID string) (string, time.Time, error) {
	now := s.clock.Now()
	expiresAt := now.Add(s.ttl)

	c := claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			Issuer:    s.issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}

	tok := jwt.NewWithClaims(jwt.SigningMethodEdDSA, c)
	signed, err := tok.SignedString(s.priv)
	if err != nil {
		// coverage:ignore — jwt.SignedString with EdDSA fails only if the
		// private key is malformed (wrong length), which NewSigner's
		// caller controls and which is covered instead by
		// TestNewKeyPair/TestSignerFromSeed validating key construction;
		// simulating a corrupt in-memory key here would test the jwt
		// library, not this package's logic.
		return "", time.Time{}, fmt.Errorf("token: sign: %w", err)
	}
	return signed, expiresAt, nil
}

// Authenticate implements middleware.Authenticator: it verifies tokenStr
// and returns the subject user ID.
func (s *Signer) Authenticate(tokenStr string) (string, error) {
	var c claims
	parsed, err := jwt.ParseWithClaims(tokenStr, &c, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodEd25519); !ok {
			return nil, fmt.Errorf("token: unexpected signing method %v", t.Header["alg"])
		}
		return s.pub, nil
	}, jwt.WithIssuer(s.issuer), jwt.WithTimeFunc(s.clock.Now))
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return "", ErrExpiredToken
		}
		return "", fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}
	if !parsed.Valid || c.Subject == "" {
		return "", ErrInvalidToken
	}
	return c.Subject, nil
}

// Verify is an alias for Authenticate kept for readability at call sites
// that aren't going through the middleware.Authenticator interface.
func (s *Signer) Verify(tokenStr string) (string, error) { return s.Authenticate(tokenStr) }

// NewKeyPair generates a fresh Ed25519 key pair, used for local/dev
// environments where no signing seed is configured (see
// internal/platform/config's JWTEd25519SeedHex TODO for production key
// management via Vault/KMS).
func NewKeyPair() (ed25519.PublicKey, ed25519.PrivateKey, error) {
	return ed25519.GenerateKey(nil)
}

// KeyPairFromSeed deterministically derives an Ed25519 key pair from a
// 32-byte seed, so a configured signing key survives process restarts.
func KeyPairFromSeed(seed []byte) (ed25519.PublicKey, ed25519.PrivateKey, error) {
	if len(seed) != ed25519.SeedSize {
		return nil, nil, fmt.Errorf("token: seed must be %d bytes, got %d", ed25519.SeedSize, len(seed))
	}
	priv := ed25519.NewKeyFromSeed(seed)
	pub := priv.Public().(ed25519.PublicKey)
	return pub, priv, nil
}
