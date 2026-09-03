// Package password implements Argon2id password hashing per security.md §2:
// "Argon2id, parâmetros mínimos alinhados à recomendação OWASP: memória 19
// MiB, iterações = 2, paralelismo = 1 ... onde a capacidade do servidor
// permitir, subir para memória 64 MiB / iterações 3", plus an application
// pepper (a secret held outside the database, per the same section).
package password

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Sentinel errors for the encoded-hash format.
var (
	ErrInvalidHash         = errors.New("password: invalid encoded hash format")
	ErrIncompatibleVersion = errors.New("password: incompatible argon2 version")
)

// Params configures Argon2id. Defaults (see NewHasher) use the "server
// capacity permits" tier from security.md §2 (64 MiB / t=3 / p=1).
type Params struct {
	Memory  uint32 // KiB
	Time    uint32 // iterations
	Threads uint8
	SaltLen uint32
	KeyLen  uint32
}

// DefaultParams is the production tier from security.md §2.
var DefaultParams = Params{
	Memory:  64 * 1024,
	Time:    3,
	Threads: 1,
	SaltLen: 16,
	KeyLen:  32,
}

// Hasher hashes and verifies passwords.
type Hasher struct {
	params Params
	pepper []byte
}

// NewHasher returns a Hasher using DefaultParams. pepper is an
// application-wide secret (security.md §2); it may be nil in local/dev
// environments where no pepper is configured, but production deployments
// must supply one via Vault/KMS (see internal/platform/config's TODO).
func NewHasher(pepper []byte) *Hasher {
	return &Hasher{params: DefaultParams, pepper: pepper}
}

// NewHasherWithParams returns a Hasher with explicit params, primarily for
// tests that want a cheap configuration so the unit suite stays fast
// (backend-go.md §7: "toda a suíte unitária em <30s").
func NewHasherWithParams(params Params, pepper []byte) *Hasher {
	return &Hasher{params: params, pepper: pepper}
}

// pepper applies the HMAC-SHA256 pepper step: HMAC(pepper, password) is fed
// into Argon2id instead of the raw password. Two reasons to HMAC rather
// than simply concatenate: (1) it produces a fixed-size, uniformly
// distributed input regardless of password length, avoiding any
// implementation quirks around very long inputs; (2) it's the standard,
// reviewable construction for "secret-keyed password pre-hash".
func (h *Hasher) peppered(passwordPlain string) []byte {
	if len(h.pepper) == 0 {
		return []byte(passwordPlain)
	}
	mac := hmac.New(sha256.New, h.pepper)
	mac.Write([]byte(passwordPlain))
	return mac.Sum(nil)
}

// Hash returns a self-describing PHC-like encoded hash:
// $argon2id$v=19$m=<mem>,t=<time>,p=<threads>$<salt-b64>$<hash-b64>
func (h *Hasher) Hash(passwordPlain string) (string, error) {
	salt := make([]byte, h.params.SaltLen)
	if _, err := rand.Read(salt); err != nil {
		// coverage:ignore — crypto/rand.Read failing means the OS entropy
		// source is broken; not reproducible in a hermetic unit test, and
		// there is no meaningful recovery other than surfacing the error
		// (never silently hashing with a weak/zero salt).
		return "", fmt.Errorf("password: read salt: %w", err)
	}

	key := argon2.IDKey(h.peppered(passwordPlain), salt, h.params.Time, h.params.Memory, h.params.Threads, h.params.KeyLen)

	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		h.params.Memory, h.params.Time, h.params.Threads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

// Verify reports whether passwordPlain matches encoded, using a
// constant-time comparison of the derived keys.
func (h *Hasher) Verify(passwordPlain, encoded string) (bool, error) {
	params, salt, want, err := decode(encoded)
	if err != nil {
		return false, err
	}

	got := argon2.IDKey(h.peppered(passwordPlain), salt, params.Time, params.Memory, params.Threads, uint32(len(want)))

	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

func decode(encoded string) (Params, []byte, []byte, error) {
	parts := strings.Split(encoded, "$")
	// parts[0] is "" (leading $), then "argon2id", "v=19", "m=..,t=..,p=..", salt, hash
	if len(parts) != 6 || parts[1] != "argon2id" {
		return Params{}, nil, nil, ErrInvalidHash
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return Params{}, nil, nil, ErrInvalidHash
	}
	if version != argon2.Version {
		return Params{}, nil, nil, ErrIncompatibleVersion
	}

	var p Params
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.Memory, &p.Time, &p.Threads); err != nil {
		return Params{}, nil, nil, ErrInvalidHash
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return Params{}, nil, nil, ErrInvalidHash
	}
	hash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return Params{}, nil, nil, ErrInvalidHash
	}

	return p, salt, hash, nil
}
