package token

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSecureRefreshGenerator_New(t *testing.T) {
	gen := SecureRefreshGenerator{}

	a, err := gen.New()
	require.NoError(t, err)
	b, err := gen.New()
	require.NoError(t, err)

	assert.NotEmpty(t, a)
	assert.NotEqual(t, a, b, "two generated tokens must differ")
}

func TestHashRefreshToken_Deterministic(t *testing.T) {
	h1 := HashRefreshToken("plaintext-token")
	h2 := HashRefreshToken("plaintext-token")
	assert.Equal(t, h1, h2)
	assert.Len(t, h1, 64) // hex-encoded sha256
}

func TestHashRefreshToken_DifferentInputsDifferentHashes(t *testing.T) {
	a := HashRefreshToken("token-a")
	b := HashRefreshToken("token-b")
	assert.NotEqual(t, a, b)
}
