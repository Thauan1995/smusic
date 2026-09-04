//go:build integration

// See .vibeflow/specs/backend-integration-test-coverage.md: this file
// exercises NewPool directly (dbxtest.NewPool below also calls it, on
// every one of the four postgres packages' integration suites, but this
// test makes the coverage attribution to NewPool itself explicit rather
// than only incidental).
package dbx_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"smusic/backend/internal/platform/dbx"
	"smusic/backend/internal/platform/dbx/dbxtest"
)

func TestIntegration_NewPool_ConnectsAndPings(t *testing.T) {
	pool := dbxtest.NewPool(t) // starts a real container, runs migrations, calls dbx.NewPool
	require.NotNil(t, pool)
	assert.NoError(t, pool.Ping(context.Background()))
}

func TestIntegration_NewPool_InvalidURL(t *testing.T) {
	_, err := dbx.NewPool(context.Background(), "postgres://nope:nope@127.0.0.1:1/does-not-exist?connect_timeout=1")
	require.Error(t, err, "an unreachable DSN must surface as an error, not hang or panic")
}
