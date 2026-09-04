# Multi-stage build for smusic's 3 Go binaries. GOTOOLCHAIN=auto (the Go
# default, unmodified here) makes `go build` auto-download the exact
# toolchain go.mod's `go` directive pins (see README's "Go-toolchain
# vulnerabilities" section) regardless of what this base image ships with.
FROM golang:1-alpine AS builder
WORKDIR /src
RUN apk add --no-cache git
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/smusic-server ./cmd/server
RUN CGO_ENABLED=0 go build -o /out/smusic-presence-server ./cmd/presence-server
RUN CGO_ENABLED=0 go build -o /out/smusic-migrate ./cmd/migrate

# --- cmd/server: REST/WS control plane (auth, catalog, library, playback,
# presence settings). Needs testdata/media/ alongside it — cmd/server's
# router serves /media/* from that relative path at runtime (see main.go).
FROM alpine:3 AS server
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /out/smusic-server ./smusic-server
COPY --from=builder /src/testdata ./testdata
EXPOSE 8080
ENTRYPOINT ["./smusic-server"]

# --- cmd/presence-server: WS /v1/presence/connect data plane only.
FROM alpine:3 AS presence-server
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /out/smusic-presence-server ./smusic-presence-server
EXPOSE 8081
ENTRYPOINT ["./smusic-presence-server"]

# --- cmd/migrate: one-off schema migration runner. Needs migrations/
# alongside it — defaults to the relative "migrations" dir (see main.go's
# -dir flag default).
FROM alpine:3 AS migrate
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /out/smusic-migrate ./smusic-migrate
COPY --from=builder /src/migrations ./migrations
ENTRYPOINT ["./smusic-migrate"]
CMD ["up"]
