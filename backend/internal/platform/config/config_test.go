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

	// presence-service defaults (Fatia 2, backend-go.md §1/§3, security.md
	// §1.5's mandatory 90s TTL).
	assert.Equal(t, ":8081", cfg.PresenceHTTPAddr)
	assert.Equal(t, 32, cfg.PresenceWorkers)
	assert.Equal(t, 4096, cfg.PresenceIngestBuffer)
	assert.Equal(t, 90*time.Second, cfg.PresenceTTL)
	assert.Equal(t, 12, cfg.PresenceUpdateRateLimit)
	assert.Equal(t, time.Minute, cfg.PresenceUpdateRateWindow)
}

func TestLoad_Overrides(t *testing.T) {
	cfg, err := Load(fromMap(map[string]string{
		"HTTP_ADDR":                   ":9090",
		"REDIS_DB":                    "2",
		"ACCESS_TOKEN_TTL":            "5m",
		"REFRESH_TOKEN_TTL":           "72h",
		"LOGIN_RATE_LIMIT_PER_MINUTE": "3",
		"JWT_ISSUER":                  "smusic-test",
		"PRESENCE_HTTP_ADDR":          ":9091",
		"PRESENCE_WORKERS":            "64",
		"PRESENCE_INGEST_BUFFER":      "8192",
		"PRESENCE_TTL":                "60s",
		"PRESENCE_UPDATE_RATE_LIMIT":  "20",
		"PRESENCE_UPDATE_RATE_WINDOW": "2m",
	}))
	require.NoError(t, err)

	assert.Equal(t, ":9090", cfg.HTTPAddr)
	assert.Equal(t, 2, cfg.RedisDB)
	assert.Equal(t, 5*time.Minute, cfg.AccessTokenTTL)
	assert.Equal(t, 72*time.Hour, cfg.RefreshTokenTTL)
	assert.Equal(t, 3, cfg.LoginRateLimitPerMinute)
	assert.Equal(t, "smusic-test", cfg.JWTIssuer)
	assert.Equal(t, ":9091", cfg.PresenceHTTPAddr)
	assert.Equal(t, 64, cfg.PresenceWorkers)
	assert.Equal(t, 8192, cfg.PresenceIngestBuffer)
	assert.Equal(t, 60*time.Second, cfg.PresenceTTL)
	assert.Equal(t, 20, cfg.PresenceUpdateRateLimit)
	assert.Equal(t, 2*time.Minute, cfg.PresenceUpdateRateWindow)
}

func TestLoad_InvalidPresenceWorkers(t *testing.T) {
	_, err := Load(fromMap(map[string]string{"PRESENCE_WORKERS": "nope"}))
	require.Error(t, err)
}

func TestLoad_InvalidPresenceIngestBuffer(t *testing.T) {
	_, err := Load(fromMap(map[string]string{"PRESENCE_INGEST_BUFFER": "nope"}))
	require.Error(t, err)
}

func TestLoad_InvalidPresenceTTL(t *testing.T) {
	_, err := Load(fromMap(map[string]string{"PRESENCE_TTL": "nope"}))
	require.Error(t, err)
}

func TestLoad_InvalidPresenceUpdateRateLimit(t *testing.T) {
	_, err := Load(fromMap(map[string]string{"PRESENCE_UPDATE_RATE_LIMIT": "nope"}))
	require.Error(t, err)
}

func TestLoad_InvalidPresenceUpdateRateWindow(t *testing.T) {
	_, err := Load(fromMap(map[string]string{"PRESENCE_UPDATE_RATE_WINDOW": "nope"}))
	require.Error(t, err)
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
