// Command migrate applies (or rolls back) database/sql migrations against
// the Postgres instance configured via DATABASE_URL, using golang-migrate
// (chosen over atlas for this slice: golang-migrate's plain numbered
// .up.sql/.down.sql file pairs are simpler to review in a PR diff than
// atlas's HCL-based declarative schema, and the project doesn't yet need
// atlas's schema-diffing; migrations/ is intentionally plain SQL, not
// generated).
//
// This file is wiring/CLI-plumbing only — like cmd/server/main.go, it is
// excluded from the unit-coverage target per 00-overview.md §2.
//
// Usage:
//
//	go run ./cmd/migrate up
//	go run ./cmd/migrate down
//	go run ./cmd/migrate -dir=./migrations up
package main

import (
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/golang-migrate/migrate/v4"
	pgx5 "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"

	"smusic/backend/internal/platform/config"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "fatal:", err)
		os.Exit(1)
	}
}

func run() error {
	dir := flag.String("dir", "migrations", "path to the migrations directory")
	flag.Parse()

	args := flag.Args()
	if len(args) != 1 || (args[0] != "up" && args[0] != "down") {
		return errors.New("usage: migrate [-dir=migrations] up|down")
	}

	cfg, err := config.Load(os.LookupEnv)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	db, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	driver, err := pgx5.WithInstance(db, &pgx5.Config{})
	if err != nil {
		return fmt.Errorf("create migration driver: %w", err)
	}

	m, err := migrate.NewWithDatabaseInstance("file://"+*dir, "smusic", driver)
	if err != nil {
		return fmt.Errorf("create migrate instance: %w", err)
	}

	switch args[0] {
	case "up":
		err = m.Up()
	case "down":
		err = m.Down()
	}
	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("run migration %s: %w", args[0], err)
	}

	fmt.Printf("migrate %s: done\n", args[0])
	return nil
}
