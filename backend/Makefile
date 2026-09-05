.PHONY: up down migrate-up migrate-down run build test test-race test-integration cover vet lint tidy loadtest-http loadtest-presence

up:
	docker-compose up -d

down:
	docker-compose down

migrate-up:
	go run ./cmd/migrate up

migrate-down:
	go run ./cmd/migrate down

run:
	go run ./cmd/server

build:
	go build ./...

test:
	go test ./...

test-race:
	go test -race ./...

# Integration tier (backend-go.md §7.3; .vibeflow/specs/backend-integration-test-coverage.md):
# spins up real, ephemeral Postgres containers via testcontainers-go
# (internal/platform/dbx/dbxtest) — requires a working Docker daemon.
# Separate from `test`/`test-race`/`cover` on purpose: keeps the fast unit
# loop fast (backend-go.md §7's "<30s" target), only run this before
# merge/release, not on every save.
test-integration:
	go test -tags=integration ./...

cover:
	go test -race -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

vet:
	go vet ./...

lint:
	@command -v staticcheck >/dev/null 2>&1 && staticcheck ./... || echo "staticcheck not installed, skipping (see README)"

tidy:
	go mod tidy

# Load-test harness (.vibeflow/specs/performance-benchmark-harness.md,
# docs/architecture/performance-baseline.md). Release-time checks, not CI
# gates (backend-go.md §7.5) — point DATABASE_URL/-base-url at a *local*
# instance unless you have explicit authorization to target production;
# see performance-baseline.md's rate-limiting note before raising
# LOGIN_RATE_LIMIT_PER_MINUTE anywhere but a disposable local instance.
loadtest-http:
	go run ./loadtest/http -base-url http://localhost:8080 -database-url "$(DATABASE_URL)"

loadtest-presence:
	go run ./loadtest/presence -api-base-url http://localhost:8080 -ws-base-url ws://localhost:8081 -clients 10,50,200
