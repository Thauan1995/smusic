//go:build integration

// Package dbxtest provides the shared testcontainers-go Postgres fixture
// for this backend's integration test tier (backend-go.md §7.3;
// .vibeflow/specs/backend-integration-test-coverage.md). Every
// internal/<domain>/postgres integration test uses NewPool instead of
// duplicating container/migration setup four times.
//
// Build-tagged `integration` — like the test files that import it, this
// package (and its testcontainers-go/docker dependency) is never compiled
// into the unit-test binary or any production binary.
package dbxtest

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	pgx5migrate "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"smusic/backend/internal/platform/dbx"
)

// migrationsDir resolves migrations/ relative to this source file (not the
// test binary's working directory, which is the calling package's
// directory, several levels below the repo root) so every integration
// test finds the same real migration files dbx.NewPool's callers use in
// production — no separate/duplicated schema definition for tests.
func migrationsDir() string {
	_, file, _, _ := runtime.Caller(0)
	// this file: backend/internal/platform/dbx/dbxtest/dbxtest.go
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "migrations")
}

// NewPool starts an ephemeral Postgres container (isolated per call, per
// backend-go.md §7.3's "isolados por schema/container por suíte"), applies
// every migrations/*.up.sql against it, and returns a ready-to-use pool via
// the same dbx.NewPool production callers use — exercising that function
// too (it was previously entirely coverage:ignore'd). Registers t.Cleanup
// to close the pool and terminate the container; callers don't need their
// own defer/cleanup.
func NewPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()

	container, err := tcpostgres.Run(ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("smusic_test"),
		tcpostgres.WithUsername("smusic"),
		tcpostgres.WithPassword("smusic"),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatalf("dbxtest: start postgres container: %v", err)
	}
	t.Cleanup(func() {
		if err := container.Terminate(context.Background()); err != nil {
			t.Logf("dbxtest: terminate container: %v", err)
		}
	})

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("dbxtest: connection string: %v", err)
	}

	if err := applyMigrations(dsn); err != nil {
		t.Fatalf("dbxtest: apply migrations: %v", err)
	}

	pool, err := dbx.NewPool(ctx, dsn)
	if err != nil {
		t.Fatalf("dbxtest: new pool: %v", err)
	}
	t.Cleanup(pool.Close)

	return pool
}

// applyMigrations runs every migrations/*.up.sql against dsn via
// golang-migrate — the exact mechanism cmd/migrate uses in production
// (see cmd/migrate/main.go), so a migration that's broken for a real
// Postgres fails an integration test run, not just a manual `go run
// ./cmd/migrate up`.
func applyMigrations(dsn string) error {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	driver, err := pgx5migrate.WithInstance(db, &pgx5migrate.Config{})
	if err != nil {
		return fmt.Errorf("create migration driver: %w", err)
	}

	m, err := migrate.NewWithDatabaseInstance("file://"+migrationsDir(), "smusic_test", driver)
	if err != nil {
		return fmt.Errorf("create migrate instance: %w", err)
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}
