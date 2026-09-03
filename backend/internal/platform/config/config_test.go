package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fromMap(m map[string]string) Lookup {
	return func(key string) (string, bool) {
		v, ok := m[key]
		return v, ok
	}
}

func TestLoad_Defaults(t *testing.T) {
	cfg, err := Load(fromMap(nil))
	require.NoError(t, err)

	assert.Equal(t, ":8080", cfg.HTTPAddr)
	assert.Equal(t, "localhost:6379", cfg.RedisAddr)
	assert.Equal(t, 0, cfg.RedisDB)
	assert.Equal(t, 15*time.Minute, cfg.AccessTokenTTL)
	assert.Equal(t, 30*24*time.Hour, cfg.RefreshTokenTTL)
	assert.Equal(t, 10, cfg.LoginRateLimitPerMinute)
	assert.Equal(t, "smusic", cfg.JWTIssuer)
}

func TestLoad_Overrides(t *testing.T) {
	cfg, err := Load(fromMap(map[string]string{
		"HTTP_ADDR":                   ":9090",
		"REDIS_DB":                    "2",
		"ACCESS_TOKEN_TTL":            "5m",
		"REFRESH_TOKEN_TTL":           "72h",
		"LOGIN_RATE_LIMIT_PER_MINUTE": "3",
		"JWT_ISSUER":                  "smusic-test",
	}))
	require.NoError(t, err)

	assert.Equal(t, ":9090", cfg.HTTPAddr)
	assert.Equal(t, 2, cfg.RedisDB)
	assert.Equal(t, 5*time.Minute, cfg.AccessTokenTTL)
	assert.Equal(t, 72*time.Hour, cfg.RefreshTokenTTL)
	assert.Equal(t, 3, cfg.LoginRateLimitPerMinute)
	assert.Equal(t, "smusic-test", cfg.JWTIssuer)
}

func TestLoad_InvalidInt(t *testing.T) {
	_, err := Load(fromMap(map[string]string{"REDIS_DB": "not-an-int"}))
	require.Error(t, err)
}

func TestLoad_InvalidDuration(t *testing.T) {
	_, err := Load(fromMap(map[string]string{"ACCESS_TOKEN_TTL": "not-a-duration"}))
	require.Error(t, err)
}

func TestLoad_InvalidRateLimit(t *testing.T) {
	_, err := Load(fromMap(map[string]string{"LOGIN_RATE_LIMIT_PER_MINUTE": "nope"}))
	require.Error(t, err)
}

func TestLoad_InvalidRefreshTTL(t *testing.T) {
	_, err := Load(fromMap(map[string]string{"REFRESH_TOKEN_TTL": "nope"}))
	require.Error(t, err)
}

func TestLoad_EmptyStringFallsBackToDefault(t *testing.T) {
	cfg, err := Load(fromMap(map[string]string{"HTTP_ADDR": ""}))
	require.NoError(t, err)
	assert.Equal(t, ":8080", cfg.HTTPAddr)
}

func TestLoad_CORSAllowedOrigins_Unset(t *testing.T) {
	cfg, err := Load(fromMap(nil))
	require.NoError(t, err)
	assert.Nil(t, cfg.CORSAllowedOrigins)
}

func TestLoad_CORSAllowedOrigins_ParsesCSVAndTrims(t *testing.T) {
	cfg, err := Load(fromMap(map[string]string{
		"CORS_ALLOWED_ORIGINS": " http://localhost:5173 ,https://app.smusic.example ,,http://localhost:3000",
	}))
	require.NoError(t, err)
	assert.Equal(t, []string{
		"http://localhost:5173",
		"https://app.smusic.example",
		"http://localhost:3000",
	}, cfg.CORSAllowedOrigins)
}

func TestLoad_CORSAllowedOrigins_RejectsWildcard(t *testing.T) {
	_, err := Load(fromMap(map[string]string{"CORS_ALLOWED_ORIGINS": "*"}))
	require.Error(t, err)

	_, err = Load(fromMap(map[string]string{"CORS_ALLOWED_ORIGINS": "http://localhost:5173,*"}))
	require.Error(t, err)
}
