package token

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

// RefreshGenerator mints new opaque refresh token plaintexts. Isolated
// behind an interface (backend-go.md §7) so service-layer tests can inject
// a deterministic fake instead of depending on crypto/rand output.
type RefreshGenerator interface {
	New() (plaintext string, err error)
}

// refreshTokenBytes is the amount of entropy in a generated refresh token.
// 32 bytes (256 bits) is comfortably beyond brute-force range for a
// long-lived, high-value credential.
const refreshTokenBytes = 32

// SecureRefreshGenerator is the production RefreshGenerator.
type SecureRefreshGenerator struct{}

// New returns a URL-safe, base64-encoded random token.
func (SecureRefreshGenerator) New() (string, error) {
	buf := make([]byte, refreshTokenBytes)
	if _, err := rand.Read(buf); err != nil {
		// coverage:ignore — see the identical justification in
		// password/argon2.go's Hash: crypto/rand failure means broken OS
		// entropy, not reproducible hermetically, and there is no
		// meaningful fallback other than propagating the error.
		return "", fmt.Errorf("token: generate refresh token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// HashRefreshToken returns the hex-encoded SHA-256 digest of a refresh
// token plaintext. security.md §2 requires refresh tokens be "armazenado
// hasheado" — unlike passwords, refresh tokens are already
// maximum-entropy random values (not human-chosen secrets), so a fast
// cryptographic hash (rather than Argon2id) is the appropriate, standard
// choice: it defends the database-at-rest case (a DB leak doesn't hand out
// live sessions) without the deliberate slowness that only matters against
// low-entropy, guessable secrets.
func HashRefreshToken(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}
