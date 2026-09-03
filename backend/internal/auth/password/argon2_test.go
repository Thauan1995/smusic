package password

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testParams keeps the unit suite fast (backend-go.md §7: whole suite <30s)
// while still exercising the real Argon2id code path.
var testParams = Params{Memory: 8 * 1024, Time: 1, Threads: 1, SaltLen: 16, KeyLen: 32}

func TestHashVerify_RoundTrip(t *testing.T) {
	h := NewHasherWithParams(testParams, []byte("pepper"))

	encoded, err := h.Hash("correct horse battery staple")
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(encoded, "$argon2id$v=19$"))

	ok, err := h.Verify("correct horse battery staple", encoded)
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestVerify_WrongPassword(t *testing.T) {
	h := NewHasherWithParams(testParams, []byte("pepper"))
	encoded, err := h.Hash("right-password")
	require.NoError(t, err)

	ok, err := h.Verify("wrong-password", encoded)
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestHash_SaltsDiffer(t *testing.T) {
	h := NewHasherWithParams(testParams, nil)
	a, err := h.Hash("same-password")
	require.NoError(t, err)
	b, err := h.Hash("same-password")
	require.NoError(t, err)
	assert.NotEqual(t, a, b, "two hashes of the same password must use different salts")
}

func TestHasher_NoPepperConfigured(t *testing.T) {
	h := NewHasherWithParams(testParams, nil)
	encoded, err := h.Hash("password")
	require.NoError(t, err)

	ok, err := h.Verify("password", encoded)
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestHasher_WrongPepperFails(t *testing.T) {
	h1 := NewHasherWithParams(testParams, []byte("pepper-a"))
	h2 := NewHasherWithParams(testParams, []byte("pepper-b"))

	encoded, err := h1.Hash("password")
	require.NoError(t, err)

	ok, err := h2.Verify("password", encoded)
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestVerify_InvalidHashFormats(t *testing.T) {
	h := NewHasherWithParams(testParams, nil)

	cases := map[string]string{
		"empty":            "",
		"too few segments": "$argon2id$v=19$m=8,t=1,p=1$salt",
		"wrong algorithm":  "$bcrypt$v=19$m=8,t=1,p=1$c2FsdA$aGFzaA",
		"bad version":      "$argon2id$v=abc$m=8,t=1,p=1$c2FsdA$aGFzaA",
		"bad params":       "$argon2id$v=19$bogus$c2FsdA$aGFzaA",
		"bad salt b64":     "$argon2id$v=19$m=8,t=1,p=1$not-base64!!!$aGFzaA",
		"bad hash b64":     "$argon2id$v=19$m=8,t=1,p=1$c2FsdA$not-base64!!!",
	}

	for name, encoded := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := h.Verify("password", encoded)
			require.Error(t, err)
		})
	}
}

func TestVerify_IncompatibleVersion(t *testing.T) {
	h := NewHasherWithParams(testParams, nil)
	_, err := h.Verify("password", "$argon2id$v=1$m=8,t=1,p=1$c2FsdA$aGFzaA")
	require.ErrorIs(t, err, ErrIncompatibleVersion)
}

func TestNewHasher_UsesDefaultParams(t *testing.T) {
	h := NewHasher([]byte("pepper"))
	assert.Equal(t, DefaultParams, h.params)
}
