// Package config loads process configuration from environment variables.
// It is a pure function of its input (a lookup function), so it is unit
// tested without touching the real environment.
package config

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Config holds every environment-derived setting the server needs.
type Config struct {
	HTTPAddr string

	DatabaseURL string

	RedisAddr     string
	RedisPassword string
	RedisDB       int

	// JWTEd25519SeedHex is a hex-encoded 32-byte Ed25519 seed used to sign
	// access tokens (security.md §2: "assinado (RS256/EdDSA)" — Ed25519 is
	// chosen over RSA for simplicity of key generation/rotation in Go with
	// no loss of the required guarantee). TODO: in production this must be
	// provisioned via Vault/KMS (security.md §3), never a plain env var.
	JWTEd25519SeedHex string
	JWTIssuer         string
	AccessTokenTTL    time.Duration
	RefreshTokenTTL   time.Duration

	// PasswordPepperHex is a hex-encoded application-wide secret mixed into
	// every password hash (security.md §2). TODO: provision via Vault/KMS
	// in production, same as above.
	PasswordPepperHex string

	// MediaBaseURL is the base URL the local fake MediaURLResolver serves
	// test audio from (backend-go.md §2 TODO: replace with real
	// media-edge-service + CDN signed URLs in Fatia 2).
	MediaBaseURL    string
	MediaSigningKey string

	LoginRateLimitPerMinute int

	// CORSAllowedOrigins is the explicit allowlist of browser origins
	// (scheme+host+port, e.g. "http://localhost:5173") permitted to call
	// this API cross-origin, parsed from the comma-separated
	// CORS_ALLOWED_ORIGINS env var. It defaults to empty, which disables
	// cross-origin browser access entirely (server-to-server/curl calls
	// are unaffected — CORS is a browser-enforced restriction, not a
	// server one). Deliberately never accepts "*": the API is
	// bearer-token authenticated over the Authorization header, and an
	// open wildcard origin is unnecessary risk for zero benefit — see
	// cmd/server/main.go's CORS wiring and the README's CORS section.
	CORSAllowedOrigins []string
}

// Lookup mirrors os.LookupEnv's signature, letting tests supply a fake map
// instead of touching the real process environment.
type Lookup func(key string) (string, bool)

// Load builds a Config from lookup, applying defaults for anything unset.
// It returns an error only for values that are set but malformed (e.g. a
// non-integer port) — a genuinely missing value falls back to its default
// rather than failing, since every default is safe for local development.
func Load(lookup Lookup) (Config, error) {
	cfg := Config{
		HTTPAddr:                getOr(lookup, "HTTP_ADDR", ":8080"),
		DatabaseURL:             getOr(lookup, "DATABASE_URL", "postgres://smusic:smusic@localhost:5432/smusic?sslmode=disable"),
		RedisAddr:               getOr(lookup, "REDIS_ADDR", "localhost:6379"),
		RedisPassword:           getOr(lookup, "REDIS_PASSWORD", ""),
		JWTEd25519SeedHex:       getOr(lookup, "JWT_ED25519_SEED_HEX", ""),
		JWTIssuer:               getOr(lookup, "JWT_ISSUER", "smusic"),
		PasswordPepperHex:       getOr(lookup, "PASSWORD_PEPPER_HEX", ""),
		MediaBaseURL:            getOr(lookup, "MEDIA_BASE_URL", "http://localhost:8080/media"),
		MediaSigningKey:         getOr(lookup, "MEDIA_SIGNING_KEY", "dev-only-insecure-media-signing-key"),
		LoginRateLimitPerMinute: 10,
	}

	var err error
	if cfg.RedisDB, err = getIntOr(lookup, "REDIS_DB", 0); err != nil {
		return Config{}, err
	}
	if cfg.AccessTokenTTL, err = getDurationOr(lookup, "ACCESS_TOKEN_TTL", 15*time.Minute); err != nil {
		return Config{}, err
	}
	if cfg.RefreshTokenTTL, err = getDurationOr(lookup, "REFRESH_TOKEN_TTL", 30*24*time.Hour); err != nil {
		return Config{}, err
	}
	if cfg.LoginRateLimitPerMinute, err = getIntOr(lookup, "LOGIN_RATE_LIMIT_PER_MINUTE", 10); err != nil {
		return Config{}, err
	}
	if cfg.CORSAllowedOrigins, err = getCSVOrigins(lookup, "CORS_ALLOWED_ORIGINS"); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func getOr(lookup Lookup, key, def string) string {
	if v, ok := lookup(key); ok && v != "" {
		return v
	}
	return def
}

func getIntOr(lookup Lookup, key string, def int) (int, error) {
	v, ok := lookup(key)
	if !ok || v == "" {
		return def, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("config: invalid int for %s: %w", key, err)
	}
	return n, nil
}

func getDurationOr(lookup Lookup, key string, def time.Duration) (time.Duration, error) {
	v, ok := lookup(key)
	if !ok || v == "" {
		return def, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("config: invalid duration for %s: %w", key, err)
	}
	return d, nil
}

// getCSVOrigins parses a comma-separated list of CORS origins. Unset/empty
// yields nil (CORS disabled). Entries are trimmed; empty entries between
// commas are dropped. "*" is rejected: this API is bearer-token
// authenticated and never needs a wildcard origin (see Config.CORSAllowedOrigins).
func getCSVOrigins(lookup Lookup, key string) ([]string, error) {
	v, ok := lookup(key)
	if !ok || v == "" {
		return nil, nil
	}
	parts := strings.Split(v, ",")
	origins := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if p == "*" {
			return nil, fmt.Errorf("config: %s must not contain \"*\"; list explicit origins instead", key)
		}
		origins = append(origins, p)
	}
	return origins, nil
}
