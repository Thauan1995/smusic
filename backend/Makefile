.PHONY: up down migrate-up migrate-down run build test test-race test-integration cover vet lint tidy

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
