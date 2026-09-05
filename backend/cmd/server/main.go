// Command server runs the smusic-core monolith: auth, catalog, library and
// playback-state behind a single HTTP API, per backend-go.md §1's
// "monólito modular" decision for Fatia 1 (presence-service and
// media-edge-service are NOT extracted in this slice — see docs/architecture).
//
// This file is wiring/dependency-injection only — per 00-overview.md §2's
// test-coverage policy, main.go is explicitly excluded from the coverage
// target ("excluindo ... wiring de main.go"). All business logic lives in
// internal/<module>/service.go, which is unit-tested with fakes; this file
// only constructs the real implementations and connects them.
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

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	authapi "smusic/backend/internal/auth/api"
	"smusic/backend/internal/auth/mfa"
	mfapg "smusic/backend/internal/auth/mfa/postgres"
	"smusic/backend/internal/auth/oauth"
	"smusic/backend/internal/auth/password"
	authpg "smusic/backend/internal/auth/postgres"
	"smusic/backend/internal/auth/token"
	catalogapi "smusic/backend/internal/catalog/api"
	catalogpg "smusic/backend/internal/catalog/postgres"
	libraryapi "smusic/backend/internal/library/api"
	librarypg "smusic/backend/internal/library/postgres"
	playbackapi "smusic/backend/internal/playback/api"
	"smusic/backend/internal/playback/media"
	"smusic/backend/internal/playback/redisstore"
	presenceapi "smusic/backend/internal/presence/api"
	presencepg "smusic/backend/internal/presence/postgres"
	presenceredis "smusic/backend/internal/presence/redisstore"

	authsvc "smusic/backend/internal/auth"
	catalogsvc "smusic/backend/internal/catalog"
	librarysvc "smusic/backend/internal/library"
	playbacksvc "smusic/backend/internal/playback"
	presencesvc "smusic/backend/internal/presence"

	"smusic/backend/internal/platform/cache"
	"smusic/backend/internal/platform/clock"
	"smusic/backend/internal/platform/config"
	"smusic/backend/internal/platform/dbx"
	"smusic/backend/internal/platform/idgen"
	"smusic/backend/internal/platform/logging"
	"smusic/backend/internal/platform/middleware"
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
	log.Info("starting smusic-core", "http_addr", cfg.HTTPAddr)

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

	pub, priv, err := jwtKeyPair(cfg.JWTEd25519SeedHex, log)
	if err != nil {
		return fmt.Errorf("build jwt signing key: %w", err)
	}
	signer := token.NewSigner(priv, pub, cfg.JWTIssuer, cfg.AccessTokenTTL, realClock)

	pepper, err := hex.DecodeString(cfg.PasswordPepperHex)
	if err != nil {
		return fmt.Errorf("decode PASSWORD_PEPPER_HEX: %w", err)
	}
	if len(pepper) == 0 {
		log.Warn("PASSWORD_PEPPER_HEX not set; running without a password pepper (dev only, see security.md §2)")
	}
	hasher := password.NewHasher(pepper)

	// --- auth module ---
	authRepo := authpg.New(pool)
	mfaRepo := mfapg.New(pool)
	mfaChallenger := mfa.NewTOTPChallenger(mfaRepo, realClock, func(ctx context.Context, userID string) (string, error) {
		u, err := authRepo.GetByID(ctx, userID)
		if err != nil {
			return "", err
		}
		return u.Email, nil
	})
	authService := authsvc.NewService(
		authRepo, authRepo, authRepo, authRepo,
		hasher, signer, token.SecureRefreshGenerator{}, oauth.StubVerifier{}, mfaChallenger,
		realClock, ids, cfg.RefreshTokenTTL,
	)

	// --- catalog module ---
	artistRepo := catalogpg.NewArtistRepo(pool)
	albumRepo := catalogpg.NewAlbumRepo(pool)
	trackRepo := catalogpg.NewTrackRepo(pool)
	catalogService := catalogsvc.NewService(artistRepo, albumRepo, trackRepo, realClock, ids)

	// --- library module ---
	playlistRepo := librarypg.NewPlaylistRepo(pool)
	playlistTrackRepo := librarypg.NewPlaylistTrackRepo(pool)
	libraryTrackRepo := librarypg.NewLibraryTrackRepo(pool)
	libraryService := librarysvc.NewService(playlistRepo, playlistTrackRepo, libraryTrackRepo, catalogService, realClock, ids)

	// --- playback module ---
	signingKey := []byte(cfg.MediaSigningKey)
	resolver := media.NewLocalResolver(cfg.MediaBaseURL, signingKey, realClock)
	stateStore := redisstore.New(redisClient)
	const sessionTTL = 24 * time.Hour
	playbackService := playbacksvc.NewService(stateStore, resolver, catalogService, playbacksvc.NoopPlayEventRecorder{}, realClock, ids, sessionTTL)

	rateLimiter := cache.NewRedisRateLimiter(redisClient)
	loginRateLimit := middleware.RateLimit(rateLimiter, middleware.ClientIPKey("login"), cfg.LoginRateLimitPerMinute, time.Minute)

	// --- presence module (control plane only — Fatia 2, backend-go.md §1) ---
	// The real-time WS feed lives in the separate cmd/presence-server
	// process; smusic-core only hosts the low-frequency, Postgres-backed
	// settings/consent/block REST surface (backend-go.md §4's "REST
	// complementar"). presenceGeo is shared (same Redis instance
	// presence-server uses) purely so revoking consent/pausing here has an
	// IMMEDIATE effect on the live index (security.md §1.4/§1.1), not just
	// on the next TTL expiry.
	presenceRepo := presencepg.New(pool)
	presenceGeo := presenceredis.New(redisClient)
	// authService satisfies presence.MFAChecker structurally
	// (HasVerifiedMFA) — security.md §2's MFA-before-proximity-consent
	// requirement, see .vibeflow/specs/mfa-for-proximity-consent.md.
	presenceSettings := presencesvc.NewSettingsService(presenceRepo, presenceRepo, presenceGeo, authService, realClock)

	router := buildRouter(authService, catalogService, libraryService, playbackService, presenceSettings, signer, loginRateLimit, cfg.CORSAllowedOrigins)

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("http server listening", "addr", cfg.HTTPAddr)
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

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}
	log.Info("shutdown complete")
	return nil
}

func buildRouter(
	authService *authsvc.Service,
	catalogService *catalogsvc.Service,
	libraryService *librarysvc.Service,
	playbackService *playbacksvc.Service,
	presenceSettings *presencesvc.SettingsService,
	authr middleware.Authenticator,
	loginRateLimit func(http.Handler) http.Handler,
	corsAllowedOrigins []string,
) http.Handler {
	r := chi.NewRouter()
	r.Use(chimw.RequestID)
	// CORS: browser-only restriction (server-to-server/curl calls are
	// unaffected) so a same-origin Flutter web build in production needs
	// no entry here — this exists for cross-origin dev/staging setups
	// (`flutter run -d chrome` on a different port than the API) and any
	// future web deployment served from its own origin. Origins are an
	// explicit allowlist from CORS_ALLOWED_ORIGINS (never "*", enforced in
	// internal/platform/config): the API is bearer-token authenticated via
	// the Authorization header (see internal/auth/api/handlers.go and
	// internal/platform/middleware/auth.go — no cookies are set or read
	// anywhere in this codebase), so AllowCredentials stays false; "*"
	// would otherwise be spec-legal but is deliberately still disallowed
	// as defense-in-depth against a future cookie-based flow being added
	// without revisiting this policy. See README's "CORS" section.
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   corsAllowedOrigins,
		AllowedMethods:   []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions},
		AllowedHeaders:   []string{"Accept", "Content-Type", "Authorization"},
		AllowCredentials: false,
		MaxAge:           300,
	}))
	// Deliberately NOT using chi's RealIP middleware: it blindly trusts
	// X-Forwarded-For/X-Real-IP, which lets a client spoof its rate-limit
	// identity (GHSA-3fxj-6jh8-hvhx) unless the deployment first
	// allowlists trusted proxy IPs — not configured in this slice. Rate
	// limiting below keys on the raw r.RemoteAddr (the TCP peer) instead.
	// TODO: once deployed behind a known load balancer/proxy, adopt a
	// trusted-proxy-aware RealIP (e.g. chi's RealIP is fine IF fronted
	// only by an LB that always overwrites XFF, or use go-chi's
	// forwarded-header allowlist support).
	r.Use(chimw.Recoverer)
	r.Use(chimw.Timeout(30 * time.Second))

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	// TODO(backend-go.md §2, media-edge-service): this serves a local
	// static test asset directly from the monolith. A real deployment
	// never does this — media-edge-service generates CDN-signed URLs and
	// the monolith's Go process stays out of the audio-byte path entirely.
	r.Handle("/media/*", http.StripPrefix("/media/", http.FileServer(http.Dir("testdata/media"))))

	authapi.NewHandler(authService).Mount(r, authr, loginRateLimit)
	catalogapi.NewHandler(catalogService).Mount(r, authr, authService)
	libraryapi.NewHandler(libraryService).Mount(r, authr)
	playbackapi.NewHandler(playbackService).Mount(r, authr)
	presenceapi.NewHandler(presenceSettings).Mount(r, authr)

	return r
}

// jwtKeyPair derives the Ed25519 signing key from configuration, or
// generates an ephemeral one for local development if unset. Ephemeral
// keys mean access tokens don't survive a restart (every client must
// re-login) — acceptable for dev, never for production. See
// internal/platform/config's JWTEd25519SeedHex doc for the Vault/KMS TODO.
func jwtKeyPair(seedHex string, log *slog.Logger) (ed25519.PublicKey, ed25519.PrivateKey, error) {
	if seedHex == "" {
		log.Warn("JWT_ED25519_SEED_HEX not set; generating an ephemeral signing key (dev only, tokens won't survive a restart)")
		return token.NewKeyPair()
	}
	seed, err := hex.DecodeString(seedHex)
	if err != nil {
		return nil, nil, fmt.Errorf("decode JWT_ED25519_SEED_HEX: %w", err)
	}
	return token.KeyPairFromSeed(seed)
}
