// Package dbx wires the shared pgx connection pool. It is intentionally
// thin: connecting to a real Postgres is an integration-tested concern
// (backend-go.md §7's testing pyramid, item 3 — testcontainers-go against a
// real database), not a unit-tested one. coverage:ignore applies to this
// whole file for that reason, documented per 00-overview.md §2's rule that
// every exclusion must be explicit and reviewable rather than silent.
package dbx

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPool opens a pgx connection pool against databaseURL and verifies
// connectivity with a ping, so the process fails fast at startup instead of
// on the first request.
func NewPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("dbx: create pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("dbx: ping: %w", err)
	}
	return pool, nil
}
