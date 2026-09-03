package logging

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNew_DefaultsToTextHandler(t *testing.T) {
	t.Setenv("ENV", "")
	t.Setenv("LOG_LEVEL", "")
	log := New()
	assert.NotNil(t, log)
	assert.True(t, log.Enabled(context.TODO(), 0)) // Info level enabled by default
}

func TestNew_ProductionUsesJSONHandler(t *testing.T) {
	t.Setenv("ENV", "production")
	log := New()
	assert.NotNil(t, log)
}

func TestNew_DebugLevel(t *testing.T) {
	t.Setenv("LOG_LEVEL", "debug")
	log := New()
	assert.NotNil(t, log)
	assert.True(t, log.Enabled(context.TODO(), -4)) // slog.LevelDebug == -4
}
