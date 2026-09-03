// Command presence-server runs the proximity-discovery feed
// (WS /v1/presence/connect) as its OWN process, separate from
// smusic-core (cmd/server) — backend-go.md §1's explicit, day-1 extraction
// of presence-service, given its fundamentally different load/concurrency
// profile (thousands of long-lived connections, high-frequency small
// writes) from the rest of the monolith.
//
// Documented deviation from backend-go.md §1/§2: the doc's target
// architecture has presence-service talk gRPC internally to smusic-core
// for authorization/social-graph/persistence. This binary instead imports
// internal/presence, internal/auth/token and internal/platform/* directly
// (a plain Go package import) and connects to the SAME Postgres and Redis
// instances smusic-core uses, rather than introducing a
// protobuf/gRPC-Gateway toolchain with only one internal caller. This
// mirrors Fatia 1's own documented deviation ("REST/JSON via chi only, no
// gRPC/gRPC-Gateway" — see backend/README.md's "Desvios da spec" #1): a
// second process with no other service to talk to yet doesn't justify the
// tooling cost. What backend-go.md actually requires — "processo separado
// desde o dia 1... escala horizontalmente de forma independente... deploys
// e rollbacks frequentes sem arriscar o caminho crítico de reprodução" — is
// satisfied by this being a genuinely separate binary/deployment unit; HOW
// its two processes exchange authorization/social-graph facts (gRPC vs.
// shared DB access) is an internal wiring detail, not the requirement
// itself. TODO: introduce gRPC between the two processes once there's a
// second internal caller (media-edge-service) to amortize the toolchain
// cost against, or once independent Postgres/Redis credentials-per-service
// (security.md §4 "menor privilégio") force the two processes apart at the
// data layer too.
//
// This file, like cmd/server/main.go, is wiring/DI only —
// 00-overview.md §2 excludes it from the unit-coverage target.
package main

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	authtoken "smusic/backend/internal/auth/token"
	"smusic/backend/internal/platform/cache"
	"smusic/backend/internal/platform/clock"
	"smusic/backend/internal/platform/config"
	"smusic/backend/internal/platform/dbx"
	"smusic/backend/internal/platform/idgen"
	"smusic/backend/internal/platform/logging"
	"smusic/backend/internal/presence"
	presencepg "smusic/backend/internal/presence/postgres"
	"smusic/backend/internal/presence/redisstore"
	presencews "smusic/backend/internal/presence/ws"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "fatal:", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load(os.LookupEnv)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	log := logging.New()
	log.Info("starting presence-server", "http_addr", cfg.PresenceHTTPAddr)

	pool, err := dbx.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connect postgres: %w", err)
	}
	defer pool.Close()

	redisClient := cache.NewClient(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
	defer redisClient.Close()
	if err := cache.Ping(ctx, redisClient); err != nil {
		return fmt.Errorf("connect redis: %w", err)
	}

	realClock := clock.Real{}
	ids := idgen.UUIDv7{}

	// The access-token *verification* key must be the same one
	// smusic-core signs with — JWT_ED25519_SEED_HEX must be configured
	// identically in both processes' environments in any persistent
	// deployment (an ephemeral per-process key, like cmd/server's dev
	// fallback, would make every access token issued by smusic-core
	// unverifiable here, and is only tolerated for local dev with a
	// loud warning, exactly like cmd/server's own fallback).
	pub, _, err := jwtKeyPair(cfg.JWTEd25519SeedHex, log)
	if err != nil {
		return fmt.Errorf("build jwt verification key: %w", err)
	}
	verifier := authtoken.NewSigner(nil, pub, cfg.JWTIssuer, cfg.AccessTokenTTL, realClock)

	repo := presencepg.New(pool)
	geo := redisstore.New(redisClient)
	updateRateLimiter := cache.NewRedisRateLimiter(redisClient)
	pairRateLimiter := cache.NewRedisRateLimiter(redisClient)
	dailyRateLimiter := cache.NewRedisRateLimiter(redisClient)

	profiles := profileResolver{pool: pool}

	nearby := presence.NewNearbyService(
		repo, repo, repo, geo, repo, profiles,
		pairRateLimiter, dailyRateLimiter,
		presence.RandJitterer{}, realClock, ids,
	)
	hub := presence.NewHubWithLimit(nearby, cfg.PresenceTTL, cfg.PresenceWorkers, cfg.PresenceIngestBuffer, updateRateLimiter, cfg.PresenceUpdateRateLimit, cfg.PresenceUpdateRateWindow)

	go hub.Run(ctx)

	wsHandler := presencews.NewHandler(hub, nearby, verifier, cfg.PresenceTTL, log)

	mux := http.NewServeMux()
	mux.Handle("/v1/presence/connect", wsHandler)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	srv := &http.Server{
		Addr:              cfg.PresenceHTTPAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("presence-server listening", "addr", cfg.PresenceHTTPAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		log.Info("shutdown signal received")
	case err := <-errCh:
		return fmt.Errorf("http server: %w", err)
	}

	// backend-go.md §3's graceful-shutdown contract: stop accepting new
	// WS upgrades, drain existing connections with a "drain" frame, wait
	// for in-flight work, then close the listener.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := hub.Shutdown(shutdownCtx, "reconnect to presence-service"); err != nil {
		log.Warn("presence hub shutdown incomplete within deadline", "err", err)
	}
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}
	log.Info("shutdown complete")
	return nil
}

// jwtKeyPair mirrors cmd/server's helper of the same name: derive the
// Ed25519 key from JWT_ED25519_SEED_HEX, or generate an ephemeral one
// (dev-only — tokens signed by a smusic-core instance using a DIFFERENT
// ephemeral key will fail to verify here) if unset.
func jwtKeyPair(seedHex string, log *slog.Logger) (ed25519.PublicKey, ed25519.PrivateKey, error) {
	if seedHex == "" {
		log.Warn("JWT_ED25519_SEED_HEX not set; generating an ephemeral verification key (dev only — will NOT match smusic-core's key unless both processes share this env var)")
		return authtoken.NewKeyPair()
	}
	seed, err := hex.DecodeString(seedHex)
	if err != nil {
		return nil, nil, fmt.Errorf("decode JWT_ED25519_SEED_HEX: %w", err)
	}
	return authtoken.KeyPairFromSeed(seed)
}

// profileResolver implements presence.ProfileResolver by reading
// (display_name, avatar_url) directly off the `users` table.
//
// Documented boundary exception: backend-go.md §1's module-boundary rule
// ("um módulo nunca importa o pacote postgres de outro módulo nem toca
// suas tabelas") is written for modules WITHIN the smusic-core monolith,
// where the clean fix is always "depend on a small interface, wire the
// real implementation in cmd/server/main.go" (exactly how
// library.TrackChecker/playback.TrackChecker work today, per
// backend/README.md). presence-server is a SEPARATE PROCESS with no
// in-memory access to an auth.Service instance to wire against — the
// backend-go.md §1 "proper" design routes this through a gRPC call to
// smusic-core, which this slice already deviates away from (see this
// file's package doc). Given that, the least-bad option consistent with
// "share modules via Go import, connect to the same DB" is a narrow,
// explicitly read-only query against exactly the two columns proximity
// disclosure needs (never password_hash, email, or any other auth.User
// field) — written here, in wiring code, not inside internal/presence
// (which still only depends on the ProfileResolver interface, and could be
// pointed at a real gRPC-backed implementation later without any change to
// its own code).
type profileResolver struct {
	pool *pgxpool.Pool
}

func (p profileResolver) Resolve(ctx context.Context, userIDs []string) (map[string]presence.Profile, error) {
	out := make(map[string]presence.Profile, len(userIDs))
	if len(userIDs) == 0 {
		return out, nil
	}
	const q = `SELECT id, display_name, COALESCE(avatar_url, '') FROM users WHERE id = ANY($1) AND deleted_at IS NULL`
	rows, err := p.pool.Query(ctx, q, userIDs)
	if err != nil {
		return nil, fmt.Errorf("presence-server: resolve profiles: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id, name, avatar string
		if err := rows.Scan(&id, &name, &avatar); err != nil {
			return nil, fmt.Errorf("presence-server: scan profile: %w", err)
		}
		out[id] = presence.Profile{DisplayName: name, AvatarURL: avatar}
	}
	return out, rows.Err()
}
