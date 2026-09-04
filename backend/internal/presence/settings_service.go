package presence

import (
	"context"
	"errors"
	"fmt"

	"smusic/backend/internal/platform/clock"
)

// SettingsService implements the REST-facing surface: reading/updating
// privacy settings, granting/revoking proximity consent, pausing/resuming
// discovery, and managing the block list (security.md §1.1, §1.3, §1.4,
// item 7 of the task). It is deliberately separate from NearbyService (the
// WS-facing query engine) even though both live in this package: this is
// the "control plane" (infrequent, Postgres-backed, fine to run inside
// smusic-core) and NearbyService is the "data plane" (high-frequency,
// latency-sensitive, runs inside presence-service) — see cmd/presence-server
// and cmd/server's wiring.
type SettingsService struct {
	settings PrivacySettingsRepository
	blocks   BlockRepository
	geo      GeoIndex // used to enforce "removal from the index is immediate", not just eventually-via-TTL
	mfa      MFAChecker
	clock    clock.Clock
}

// NewSettingsService returns a SettingsService. geo may be nil in contexts
// that only need settings/consent/block CRUD without immediate-index-removal
// side effects (e.g. a unit test focused purely on validation) — every
// removal call below tolerates a nil geo by skipping the index side effect,
// documented at each call site. mfa must not be nil in production (see
// GrantConsent); it is never nil-checked because every call site — real
// wiring and every test — provides one.
func NewSettingsService(settings PrivacySettingsRepository, blocks BlockRepository, geo GeoIndex, mfaChecker MFAChecker, clk clock.Clock) *SettingsService {
	return &SettingsService{settings: settings, blocks: blocks, geo: geo, mfa: mfaChecker, clock: clk}
}

// Get returns userID's current settings, or the safe default
// (DefaultPrivacySettings) if none have ever been written — never an error
// for "no row yet", since that is the expected state for most users
// (security.md §1.1: off by default).
func (s *SettingsService) Get(ctx context.Context, userID string) (PrivacySettings, error) {
	if userID == "" {
		return PrivacySettings{}, fmt.Errorf("%w: user id is required", ErrInvalidInput)
	}
	st, err := s.settings.Get(ctx, userID)
	if errors.Is(err, ErrSettingsNotFound) {
		return DefaultPrivacySettings(userID), nil
	}
	if err != nil {
		return PrivacySettings{}, fmt.Errorf("presence: get settings: %w", err)
	}
	return st, nil
}

// UpdateSettingsInput carries only the fields a PUT
// /v1/presence/settings request wants to change; nil means "leave
// unchanged".
type UpdateSettingsInput struct {
	PresenceVisibility *string
	PresenceShareTrack *bool
	VisibilityRadiusM  *int
	RevealLevel        *int
	Paused             *bool
}

// Update applies a partial change to userID's settings, validating every
// touched field against security.md §1's constraints, and returns the
// resulting full record. Setting Paused=true has the immediate,
// synchronous side effect of removing userID from the live index
// (security.md §1.4).
func (s *SettingsService) Update(ctx context.Context, userID string, in UpdateSettingsInput) (PrivacySettings, error) {
	if userID == "" {
		return PrivacySettings{}, fmt.Errorf("%w: user id is required", ErrInvalidInput)
	}

	current, err := s.Get(ctx, userID)
	if err != nil {
		return PrivacySettings{}, err
	}

	if in.PresenceVisibility != nil {
		if !IsValidVisibility(*in.PresenceVisibility) {
			return PrivacySettings{}, ErrInvalidVisibility
		}
		current.PresenceVisibility = *in.PresenceVisibility
	}
	if in.PresenceShareTrack != nil {
		current.PresenceShareTrack = *in.PresenceShareTrack
	}
	if in.VisibilityRadiusM != nil {
		if !IsValidRadius(*in.VisibilityRadiusM) {
			return PrivacySettings{}, ErrInvalidRadius
		}
		current.VisibilityRadiusM = *in.VisibilityRadiusM
	}
	if in.RevealLevel != nil {
		if !IsValidRevealLevel(*in.RevealLevel) {
			return PrivacySettings{}, ErrInvalidRevealLevel
		}
		current.RevealLevel = *in.RevealLevel
	}
	if in.Paused != nil {
		current.Paused = *in.Paused
	}

	current.UpdatedAt = s.clock.Now()
	if err := s.settings.Upsert(ctx, current); err != nil {
		return PrivacySettings{}, fmt.Errorf("presence: upsert settings: %w", err)
	}

	if current.Paused || current.PresenceVisibility == VisibilityInvisible {
		s.removeFromIndex(ctx, userID)
	}

	return current, nil
}

// GrantConsent enables (or renews) proximity consent, per security.md
// §1.1: opt-in, timestamped, due for re-confirmation in
// ConsentValidityPeriod. It never implicitly changes visibility/paused —
// granting consent alone does not make a user discoverable; they must also
// unpause and choose a visibility (a deliberate two-step: consent to the
// feature processing their location is a separate decision from "be
// visible right now"). Requires a verified MFA factor (security.md §2;
// ErrMFARequired otherwise) — checked before any other work, so an
// unverified user's consent is never partially applied.
func (s *SettingsService) GrantConsent(ctx context.Context, userID string) (PrivacySettings, error) {
	if userID == "" {
		return PrivacySettings{}, fmt.Errorf("%w: user id is required", ErrInvalidInput)
	}
	hasMFA, err := s.mfa.HasVerifiedMFA(ctx, userID)
	if err != nil {
		return PrivacySettings{}, fmt.Errorf("presence: check mfa: %w", err)
	}
	if !hasMFA {
		return PrivacySettings{}, ErrMFARequired
	}
	current, err := s.Get(ctx, userID)
	if err != nil {
		return PrivacySettings{}, err
	}
	now := s.clock.Now()
	due := now.Add(ConsentValidityPeriod)
	current.ProximityConsentEnabled = true
	current.ProximityConsentTS = &now
	current.ProximityConsentRenewDue = &due
	current.UpdatedAt = now
	if err := s.settings.Upsert(ctx, current); err != nil {
		return PrivacySettings{}, fmt.Errorf("presence: upsert settings: %w", err)
	}
	return current, nil
}

// RevokeConsent implements security.md §1.1's "revogação: um único toggle
// ... interrompe o processamento imediatamente e remove o usuário do
// índice de presença ativo" and §1.7's "revogação de consentimento: 1
// toque, efeito imediato, sem necessidade de justificar, sem fricção."
// Revoking also force-pauses the account as defense in depth: consent
// disabled but paused=false would be an inconsistent state a future code
// path could accidentally read as "still discoverable."
func (s *SettingsService) RevokeConsent(ctx context.Context, userID string) (PrivacySettings, error) {
	if userID == "" {
		return PrivacySettings{}, fmt.Errorf("%w: user id is required", ErrInvalidInput)
	}
	current, err := s.Get(ctx, userID)
	if err != nil {
		return PrivacySettings{}, err
	}
	now := s.clock.Now()
	current.ProximityConsentEnabled = false
	current.Paused = true
	current.UpdatedAt = now
	if err := s.settings.Upsert(ctx, current); err != nil {
		return PrivacySettings{}, fmt.Errorf("presence: upsert settings: %w", err)
	}
	s.removeFromIndex(ctx, userID)
	return current, nil
}

// SetPaused is the fast, single-toggle path security.md §1.4 asks for
// ("toggle único e de acesso rápido... não enterrado em configurações"),
// distinct from Update so a client can wire a one-tap control without
// building a full settings PATCH payload.
func (s *SettingsService) SetPaused(ctx context.Context, userID string, paused bool) (PrivacySettings, error) {
	return s.Update(ctx, userID, UpdateSettingsInput{Paused: &paused})
}

// removeFromIndex best-effort removes userID from the live geo index. It
// never fails the caller's operation: the durable settings row (source of
// truth) is already updated by the time this runs, and NearbyService
// independently re-checks Paused/consent on every query — so a transient
// Redis error here only means the user might still show up for the
// remainder of their current TTL window (≤90s) instead of instantly,
// never a permanent privacy failure. geo may be nil (see NewSettingsService).
func (s *SettingsService) removeFromIndex(ctx context.Context, userID string) {
	if s.geo == nil {
		return
	}
	_ = s.geo.Remove(ctx, userID)
}

// Block registers that blockerID never wants blockedID to see (or be seen
// by) them in proximity discovery (security.md §1.4).
func (s *SettingsService) Block(ctx context.Context, blockerID, blockedID string) error {
	if blockerID == "" || blockedID == "" {
		return fmt.Errorf("%w: blocker and blocked user ids are required", ErrInvalidInput)
	}
	if blockerID == blockedID {
		return ErrCannotBlockSelf
	}
	if err := s.blocks.Block(ctx, blockerID, blockedID); err != nil {
		return fmt.Errorf("presence: block: %w", err)
	}
	return nil
}

// Unblock removes a block. Idempotent per BlockRepository's contract.
func (s *SettingsService) Unblock(ctx context.Context, blockerID, blockedID string) error {
	if blockerID == "" || blockedID == "" {
		return fmt.Errorf("%w: blocker and blocked user ids are required", ErrInvalidInput)
	}
	if err := s.blocks.Unblock(ctx, blockerID, blockedID); err != nil {
		return fmt.Errorf("presence: unblock: %w", err)
	}
	return nil
}
