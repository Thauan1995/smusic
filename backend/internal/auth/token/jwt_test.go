package token

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"smusic/backend/internal/platform/clock"
)

func newTestSigner(t *testing.T, ttl time.Duration, clk clock.Clock) *Signer {
	t.Helper()
	pub, priv, err := NewKeyPair()
	require.NoError(t, err)
	return NewSigner(priv, pub, "smusic-test", ttl, clk)
}

func TestSigner_SignAndAuthenticate(t *testing.T) {
	clk := clock.NewFrozen(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	s := newTestSigner(t, 15*time.Minute, clk)

	tok, expiresAt, err := s.Sign("user-123")
	require.NoError(t, err)
	assert.Equal(t, clk.Now().Add(15*time.Minute), expiresAt)

	userID, err := s.Authenticate(tok)
	require.NoError(t, err)
	assert.Equal(t, "user-123", userID)
}

func TestSigner_Verify_IsAliasForAuthenticate(t *testing.T) {
	clk := clock.NewFrozen(time.Now())
	s := newTestSigner(t, time.Minute, clk)
	tok, _, err := s.Sign("user-1")
	require.NoError(t, err)

	userID, err := s.Verify(tok)
	require.NoError(t, err)
	assert.Equal(t, "user-1", userID)
}

func TestSigner_ExpiredToken(t *testing.T) {
	clk := clock.NewFrozen(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	s := newTestSigner(t, time.Minute, clk)

	tok, _, err := s.Sign("user-1")
	require.NoError(t, err)

	clk.Advance(2 * time.Minute)

	_, err = s.Authenticate(tok)
	require.ErrorIs(t, err, ErrExpiredToken)
}

func TestSigner_InvalidToken(t *testing.T) {
	clk := clock.NewFrozen(time.Now())
	s := newTestSigner(t, time.Minute, clk)

	_, err := s.Authenticate("not-a-jwt")
	require.ErrorIs(t, err, ErrInvalidToken)
}

func TestSigner_WrongKeyRejected(t *testing.T) {
	clk := clock.NewFrozen(time.Now())
	signerA := newTestSigner(t, time.Minute, clk)
	signerB := newTestSigner(t, time.Minute, clk)

	tok, _, err := signerA.Sign("user-1")
	require.NoError(t, err)

	_, err = signerB.Authenticate(tok)
	require.ErrorIs(t, err, ErrInvalidToken)
}

func TestSigner_WrongIssuerRejected(t *testing.T) {
	clk := clock.NewFrozen(time.Now())
	pub, priv, err := NewKeyPair()
	require.NoError(t, err)

	issuerA := NewSigner(priv, pub, "issuer-a", time.Minute, clk)
	issuerB := NewSigner(priv, pub, "issuer-b", time.Minute, clk)

	tok, _, err := issuerA.Sign("user-1")
	require.NoError(t, err)

	_, err = issuerB.Authenticate(tok)
	require.ErrorIs(t, err, ErrInvalidToken)
}

// TestSigner_RejectsAlgorithmConfusion guards against the classic JWT "alg
// confusion" attack: a token signed with a different algorithm (here
// HS256) than the one this Signer trusts (EdDSA) must be rejected even if
// otherwise well-formed, never silently accepted via the wrong verification
// path.
func TestSigner_RejectsAlgorithmConfusion(t *testing.T) {
	clk := clock.NewFrozen(time.Now())
	s := newTestSigner(t, time.Minute, clk)

	hmacToken := jwt.NewWithClaims(jwt.SigningMethodHS256, claims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: "user-1", Issuer: "smusic-test"},
	})
	signed, err := hmacToken.SignedString([]byte("attacker-controlled-secret"))
	require.NoError(t, err)

	_, err = s.Authenticate(signed)
	require.ErrorIs(t, err, ErrInvalidToken)
}

// TestSigner_RejectsEmptySubject covers the defensive c.Subject == "" check:
// a well-formed, correctly-signed token that somehow carries no subject
// must never authenticate as an anonymous/empty user.
func TestSigner_RejectsEmptySubject(t *testing.T) {
	clk := clock.NewFrozen(time.Now())
	pub, priv, err := NewKeyPair()
	require.NoError(t, err)
	s := NewSigner(priv, pub, "smusic-test", time.Minute, clk)

	tok := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "",
			Issuer:    "smusic-test",
			IssuedAt:  jwt.NewNumericDate(clk.Now()),
			ExpiresAt: jwt.NewNumericDate(clk.Now().Add(time.Minute)),
		},
	})
	signed, err := tok.SignedString(priv)
	require.NoError(t, err)

	_, err = s.Authenticate(signed)
	require.ErrorIs(t, err, ErrInvalidToken)
}

func TestKeyPairFromSeed(t *testing.T) {
	seed := make([]byte, 32)
	for i := range seed {
		seed[i] = byte(i)
	}

	pub1, priv1, err := KeyPairFromSeed(seed)
	require.NoError(t, err)
	pub2, priv2, err := KeyPairFromSeed(seed)
	require.NoError(t, err)

	assert.Equal(t, pub1, pub2, "same seed must derive same public key")
	assert.Equal(t, priv1, priv2, "same seed must derive same private key")
}

func TestKeyPairFromSeed_WrongLength(t *testing.T) {
	_, _, err := KeyPairFromSeed([]byte{1, 2, 3})
	require.Error(t, err)
}

func TestNewKeyPair(t *testing.T) {
	pub, priv, err := NewKeyPair()
	require.NoError(t, err)
	assert.NotEmpty(t, pub)
	assert.NotEmpty(t, priv)
}
