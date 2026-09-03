.PHONY: up down migrate-up migrate-down run build test test-race cover vet lint tidy

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

cover:
	go test -race -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

vet:
	go vet ./...

lint:
	@command -v staticcheck >/dev/null 2>&1 && staticcheck ./... || echo "staticcheck not installed, skipping (see README)"

tidy:
	go mod tidy
