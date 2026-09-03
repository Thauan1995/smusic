package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"smusic/backend/internal/auth/oauth"
	"smusic/backend/internal/auth/token"
	"smusic/backend/internal/platform/clock"
	"smusic/backend/internal/platform/idgen"
)

// Signer is the subset of token.Signer the service depends on, expressed
// as an interface so tests can inject a fake instead of generating real
// Ed25519 keys for every test case.
type Signer interface {
	Sign(userID string) (accessToken string, expiresAt time.Time, err error)
}

// Hasher is the subset of password.Hasher the service depends on.
type Hasher interface {
	Hash(passwordPlain string) (string, error)
	Verify(passwordPlain, encoded string) (bool, error)
}

// DeviceInput is the optional device context a client may send at
// signup/login/refresh time, per security.md §2's per-device session model.
type DeviceInput struct {
	Platform   string
	PushToken  string
	AppVersion string
}

// SignUpInput is the input to Service.SignUp.
type SignUpInput struct {
	Email       string
	Password    string
	DisplayName string
	Device      *DeviceInput
}

// LoginInput is the input to Service.Login.
type LoginInput struct {
	Email    string
	Password string
	Device   *DeviceInput
}

// AuthResult is returned by every flow that issues (or re-issues) a
// session: it carries a fresh access token and, except where noted, a
// fresh opaque refresh token plaintext (shown to the client exactly once).
type AuthResult struct {
	UserID                string
	AccessToken           string
	AccessTokenExpiresAt  time.Time
	RefreshToken          string
	RefreshTokenExpiresAt time.Time
}

// Service implements signup/login/refresh/logout. All dependencies are
// interfaces (backend-go.md §7), so the full flow is unit-testable with
// in-memory fakes and no real Postgres/Redis.
type Service struct {
	users         UserRepository
	identities    IdentityRepository
	devices       DeviceRepository
	refreshTokens RefreshTokenRepository
	hasher        Hasher
	signer        Signer
	refreshGen    token.RefreshGenerator
	oauthVerifier oauth.Verifier
	clock         clock.Clock
	ids           idgen.Generator
	refreshTTL    time.Duration
}

// NewService constructs a Service from its dependencies.
func NewService(
	users UserRepository,
	identities IdentityRepository,
	devices DeviceRepository,
	refreshTokens RefreshTokenRepository,
	hasher Hasher,
	signer Signer,
	refreshGen token.RefreshGenerator,
	oauthVerifier oauth.Verifier,
	clk clock.Clock,
	ids idgen.Generator,
	refreshTTL time.Duration,
) *Service {
	return &Service{
		users:         users,
		identities:    identities,
		devices:       devices,
		refreshTokens: refreshTokens,
		hasher:        hasher,
		signer:        signer,
		refreshGen:    refreshGen,
		oauthVerifier: oauthVerifier,
		clock:         clk,
		ids:           ids,
		refreshTTL:    refreshTTL,
	}
}

// SignUp creates a new password-authenticated user and issues a session.
func (s *Service) SignUp(ctx context.Context, in SignUpInput) (AuthResult, error) {
	email := normalizeEmail(in.Email)
	if email == "" {
		return AuthResult{}, fmt.Errorf("%w: email is required", ErrInvalidInput)
	}
	if len(in.Password) < 8 {
		return AuthResult{}, fmt.Errorf("%w: password must be at least 8 characters", ErrInvalidInput)
	}
	displayName := strings.TrimSpace(in.DisplayName)
	if displayName == "" {
		return AuthResult{}, fmt.Errorf("%w: display_name is required", ErrInvalidInput)
	}

	hash, err := s.hasher.Hash(in.Password)
	if err != nil {
		return AuthResult{}, fmt.Errorf("auth: hash password: %w", err)
	}

	now := s.clock.Now()
	user := User{
		ID:           s.ids.NewID(),
		Email:        email,
		PasswordHash: hash,
		DisplayName:  displayName,
		Status:       UserStatusActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := s.users.Create(ctx, user); err != nil {
		if errors.Is(err, ErrEmailTaken) {
			return AuthResult{}, ErrEmailTaken
		}
		return AuthResult{}, fmt.Errorf("auth: create user: %w", err)
	}

	return s.issueSession(ctx, user.ID, in.Device)
}

// Login authenticates a user by email+password and issues a session.
func (s *Service) Login(ctx context.Context, in LoginInput) (AuthResult, error) {
	email := normalizeEmail(in.Email)
	if email == "" || in.Password == "" {
		return AuthResult{}, fmt.Errorf("%w: email and password are required", ErrInvalidInput)
	}

	user, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			// Deliberately identical error to "wrong password" below: the
			// API must never reveal whether an email is registered
			// (security.md §2/§5, account-takeover threat model).
			return AuthResult{}, ErrInvalidCredentials
		}
		return AuthResult{}, fmt.Errorf("auth: get user: %w", err)
	}

	if user.Status != UserStatusActive || user.PasswordHash == "" {
		return AuthResult{}, ErrInvalidCredentials
	}

	ok, err := s.hasher.Verify(in.Password, user.PasswordHash)
	if err != nil {
		return AuthResult{}, fmt.Errorf("auth: verify password: %w", err)
	}
	if !ok {
		return AuthResult{}, ErrInvalidCredentials
	}

	return s.issueSession(ctx, user.ID, in.Device)
}

// LoginWithOAuth verifies a third-party ID token and either logs in the
// linked user or provisions a new one (find-or-create), then issues a
// session. With the StubVerifier wired in (see internal/auth/oauth), this
// always returns oauth.ErrNotImplemented — the endpoint and flow are fully
// wired so swapping in a real Verifier is the only change needed.
func (s *Service) LoginWithOAuth(ctx context.Context, provider oauth.Provider, idToken string, displayName string, device *DeviceInput) (AuthResult, error) {
	if idToken == "" {
		return AuthResult{}, fmt.Errorf("%w: oauth_token is required", ErrInvalidInput)
	}

	subject, email, err := s.oauthVerifier.Verify(ctx, provider, idToken)
	if err != nil {
		return AuthResult{}, err
	}

	userID, err := s.identities.GetUserIDByProvider(ctx, string(provider), subject)
	if err == nil {
		return s.issueSession(ctx, userID, device)
	}
	if !errors.Is(err, ErrIdentityNotLinked) {
		return AuthResult{}, fmt.Errorf("auth: lookup identity: %w", err)
	}

	// Find-or-create: no identity linked yet, provision a new user.
	now := s.clock.Now()
	name := strings.TrimSpace(displayName)
	if name == "" {
		name = normalizeEmail(email)
	}
	user := User{
		ID:          s.ids.NewID(),
		Email:       normalizeEmail(email),
		DisplayName: name,
		Status:      UserStatusActive,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.users.Create(ctx, user); err != nil {
		return AuthResult{}, fmt.Errorf("auth: create oauth user: %w", err)
	}
	if err := s.identities.Link(ctx, Identity{
		ID:             s.ids.NewID(),
		UserID:         user.ID,
		Provider:       string(provider),
		ProviderUserID: subject,
		CreatedAt:      now,
	}); err != nil {
		return AuthResult{}, fmt.Errorf("auth: link identity: %w", err)
	}

	return s.issueSession(ctx, user.ID, device)
}

// Refresh rotates a refresh token: it validates the presented plaintext,
// issues a new access+refresh token pair, and revokes the old refresh
// token. Presenting a token that was already rotated away (reuse) revokes
// every refresh token for that user, on the assumption the token was
// stolen (addresses backend-go.md's open question on reuse detection).
func (s *Service) Refresh(ctx context.Context, refreshTokenPlaintext string) (AuthResult, error) {
	if refreshTokenPlaintext == "" {
		return AuthResult{}, fmt.Errorf("%w: refresh_token is required", ErrInvalidInput)
	}

	hash := token.HashRefreshToken(refreshTokenPlaintext)
	rt, err := s.refreshTokens.GetByHash(ctx, hash)
	if err != nil {
		if errors.Is(err, ErrInvalidRefreshToken) {
			return AuthResult{}, ErrInvalidRefreshToken
		}
		return AuthResult{}, fmt.Errorf("auth: get refresh token: %w", err)
	}

	now := s.clock.Now()

	if rt.ReplacedBy != "" {
		if err := s.refreshTokens.RevokeAllForUser(ctx, rt.UserID, now); err != nil {
			return AuthResult{}, fmt.Errorf("auth: revoke all after reuse: %w", err)
		}
		return AuthResult{}, ErrRefreshTokenReused
	}
	if rt.RevokedAt != nil {
		return AuthResult{}, ErrRefreshTokenRevoked
	}
	if !rt.ExpiresAt.After(now) {
		return AuthResult{}, ErrRefreshTokenExpired
	}

	result, newID, err := s.issueTokens(ctx, rt.UserID, rt.DeviceID)
	if err != nil {
		return AuthResult{}, err
	}

	if err := s.refreshTokens.MarkReplaced(ctx, rt.ID, newID, now); err != nil {
		return AuthResult{}, fmt.Errorf("auth: mark replaced: %w", err)
	}

	return result, nil
}

// Logout revokes a single refresh token. It is idempotent: presenting an
// unknown or already-revoked token is treated as "already logged out"
// rather than an error, so the endpoint never leaks whether a given token
// ever existed.
func (s *Service) Logout(ctx context.Context, refreshTokenPlaintext string) error {
	if refreshTokenPlaintext == "" {
		return fmt.Errorf("%w: refresh_token is required", ErrInvalidInput)
	}

	hash := token.HashRefreshToken(refreshTokenPlaintext)
	rt, err := s.refreshTokens.GetByHash(ctx, hash)
	if err != nil {
		if errors.Is(err, ErrInvalidRefreshToken) {
			return nil
		}
		return fmt.Errorf("auth: get refresh token: %w", err)
	}

	if err := s.refreshTokens.Revoke(ctx, rt.ID, s.clock.Now()); err != nil {
		return fmt.Errorf("auth: revoke refresh token: %w", err)
	}
	return nil
}

// LogoutAll revokes every refresh token belonging to userID (security.md
// §2: "logout de todos os dispositivos", used e.g. after an abuse report).
func (s *Service) LogoutAll(ctx context.Context, userID string) error {
	if userID == "" {
		return fmt.Errorf("%w: user id is required", ErrInvalidInput)
	}
	if err := s.refreshTokens.RevokeAllForUser(ctx, userID, s.clock.Now()); err != nil {
		return fmt.Errorf("auth: revoke all: %w", err)
	}
	return nil
}

// Me returns the profile of an already-authenticated user.
func (s *Service) Me(ctx context.Context, userID string) (User, error) {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return User{}, ErrUserNotFound
		}
		return User{}, fmt.Errorf("auth: get user: %w", err)
	}
	return user, nil
}

// issueSession signs a fresh access token, generates+stores a fresh
// refresh token, and optionally upserts the device context.
func (s *Service) issueSession(ctx context.Context, userID string, device *DeviceInput) (AuthResult, error) {
	deviceID, err := s.upsertDevice(ctx, userID, device)
	if err != nil {
		return AuthResult{}, err
	}
	result, _, err := s.issueTokens(ctx, userID, deviceID)
	return result, err
}

// issueTokens signs a fresh access token, generates a fresh opaque refresh
// token, and stores exactly one refresh-token record for it — callers that
// need the new record's ID (Refresh, to link old->new via MarkReplaced)
// get it back instead of storing a second, duplicate record themselves
// (storing twice would also violate the DB's UNIQUE(token_hash)).
func (s *Service) issueTokens(ctx context.Context, userID, deviceID string) (AuthResult, string, error) {
	accessToken, accessExpiresAt, err := s.signer.Sign(userID)
	if err != nil {
		return AuthResult{}, "", fmt.Errorf("auth: sign access token: %w", err)
	}

	refreshPlain, err := s.refreshGen.New()
	if err != nil {
		return AuthResult{}, "", fmt.Errorf("auth: generate refresh token: %w", err)
	}
	refreshExpiresAt := s.clock.Now().Add(s.refreshTTL)

	newID, err := s.storeRefreshToken(ctx, userID, deviceID, token.HashRefreshToken(refreshPlain), refreshExpiresAt)
	if err != nil {
		return AuthResult{}, "", err
	}

	return AuthResult{
		UserID:                userID,
		AccessToken:           accessToken,
		AccessTokenExpiresAt:  accessExpiresAt,
		RefreshToken:          refreshPlain,
		RefreshTokenExpiresAt: refreshExpiresAt,
	}, newID, nil
}

func (s *Service) storeRefreshToken(ctx context.Context, userID, deviceID, tokenHash string, expiresAt time.Time) (string, error) {
	id := s.ids.NewID()
	rt := RefreshToken{
		ID:        id,
		UserID:    userID,
		DeviceID:  deviceID,
		TokenHash: tokenHash,
		IssuedAt:  s.clock.Now(),
		ExpiresAt: expiresAt,
	}
	if err := s.refreshTokens.Store(ctx, rt); err != nil {
		return "", fmt.Errorf("auth: store refresh token: %w", err)
	}
	return id, nil
}

func (s *Service) upsertDevice(ctx context.Context, userID string, device *DeviceInput) (string, error) {
	if device == nil || device.Platform == "" {
		return "", nil
	}
	id, err := s.devices.Upsert(ctx, Device{
		UserID:     userID,
		Platform:   device.Platform,
		PushToken:  device.PushToken,
		AppVersion: device.AppVersion,
		LastSeenAt: s.clock.Now(),
	})
	if err != nil {
		return "", fmt.Errorf("auth: upsert device: %w", err)
	}
	return id, nil
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
