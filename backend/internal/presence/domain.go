// Package presence implements the proximity-discovery feature's privacy
// model end to end (security.md §1) and the concurrency pipeline that
// processes presence updates (backend-go.md §3). It is imported both by
// smusic-core (cmd/server — for the REST settings/consent/block surface,
// which isn't latency/concurrency sensitive) and by the separately
// deployable cmd/presence-server binary (for the WebSocket feed) per
// backend-go.md §1's mandate that presence-service be a separate process
// from day one; sharing this package via a plain Go import (rather than a
// network call between the two) is a documented, deliberate deviation —
// see cmd/presence-server's package doc for the full rationale.
//
// Every privacy control from security.md §1 is enforced in this package's
// pure/testable business logic (NearbyService, SettingsService), not in the
// WebSocket transport layer, per backend-go.md §7's "handlers finos"
// principle: the transport (internal/presence/ws, cmd/presence-server) only
// decodes frames and calls into here.
package presence

import (
	"errors"
	"time"
)

// presence_visibility values (data-architecture.md §4.5, security.md §1.1/§1.4).
const (
	VisibilityInvisible   = "invisible"
	VisibilityFriendsOnly = "friends_only"
	VisibilityEveryone    = "everyone"
)

// reveal_level values (security.md §1.6).
const (
	RevealLevelAnonymous     = 0 // default: no name/avatar shown to anyone
	RevealLevelConnections   = 1 // mutual follows see name/avatar
	RevealLevelOpenDiscovery = 2 // opt-in: level 1 also shown to non-connections
)

// AllowedRadiiM is the closed set of visibility-radius steps security.md
// §1.3 mandates: a slider, not a free-form number, with a 150m floor and a
// hard 15km ceiling ("não existe raio 'ilimitado'").
var AllowedRadiiM = []int{150, 1000, 5000, 15000}

// DefaultRadiusM is security.md §1.3's stated default when a user first
// enables the feature.
const DefaultRadiusM = 1000

// ConsentValidityPeriod is security.md §1.1's mandatory re-confirmation
// window ("consentimento expira a cada 6 meses"). Modeled as a fixed
// duration (~6 calendar months) rather than calendar-month arithmetic —
// simpler and dependency-free; the ~10 day slop across 6 months is
// immaterial to a consent-freshness control measured in months, and is
// documented here rather than silently assumed.
const ConsentValidityPeriod = 6 * 30 * 24 * time.Hour

// DistanceBucket is security.md §1.2's client-facing distance category —
// the ONLY location information that ever leaves the presence service
// toward a client about a third party. Never a coordinate, never a
// geohash.
type DistanceBucket int

const (
	// BucketNone means "not visible" — either outside both users' radii or
	// otherwise filtered out. Never sent to a client; used internally to
	// signal "exclude this candidate".
	BucketNone DistanceBucket = 0
	// Bucket1 = "Bem pertinho" (< 150m).
	Bucket1 DistanceBucket = 1
	// Bucket2 = "No seu bairro" (150m – 1km).
	Bucket2 DistanceBucket = 2
	// Bucket3 = "Na sua região" (1km – 5km).
	Bucket3 DistanceBucket = 3
	// Bucket4 = "Na sua cidade" (5km – 15km).
	Bucket4 DistanceBucket = 4
)

// Label returns the security.md §1.2 user-facing string for b, or "" for
// BucketNone (which is never displayed).
func (b DistanceBucket) Label() string {
	switch b {
	case Bucket1:
		return "Bem pertinho"
	case Bucket2:
		return "No seu bairro"
	case Bucket3:
		return "Na sua região"
	case Bucket4:
		return "Na sua cidade"
	default:
		return ""
	}
}

// PrivacySettings is the durable (Postgres) configuration record for one
// user's proximity-discovery preferences — the schema data-architecture.md
// §4.5 sketched and security.md §7 confirmed field-by-field.
type PrivacySettings struct {
	UserID                   string
	PresenceVisibility       string
	PresenceShareTrack       bool
	ProximityConsentEnabled  bool
	ProximityConsentTS       *time.Time
	ProximityConsentRenewDue *time.Time
	VisibilityRadiusM        int
	RevealLevel              int
	Paused                   bool
	CreatedAt                time.Time
	UpdatedAt                time.Time
}

// DefaultPrivacySettings is what a user who has never touched presence
// settings implicitly has: feature off, invisible, paused, minimum
// blast-radius defaults for everything else so that if the app ever races
// ahead of Upsert-ing a first row, the safe direction (nothing shown, no
// consent) is what's read back. security.md §1.1: "a feature nasce
// desligada para todo usuário, inclusive contas novas."
func DefaultPrivacySettings(userID string) PrivacySettings {
	return PrivacySettings{
		UserID:             userID,
		PresenceVisibility: VisibilityInvisible,
		VisibilityRadiusM:  DefaultRadiusM,
		RevealLevel:        RevealLevelAnonymous,
		Paused:             true,
	}
}

// HasActiveConsent reports whether s currently authorizes proximity
// processing at instant now: consent must be explicitly enabled AND not
// past its renewal due date (security.md §1.1's 6-month re-confirmation).
// A record with consent enabled but no renew-due timestamp on file is
// treated as invalid (fail closed) rather than "valid forever" — that
// combination should never occur from GrantConsent, but if it ever does
// (e.g. a manual DB edit), the safe interpretation is "not consented".
func (s PrivacySettings) HasActiveConsent(now time.Time) bool {
	if !s.ProximityConsentEnabled {
		return false
	}
	if s.ProximityConsentRenewDue == nil {
		return false
	}
	return now.Before(*s.ProximityConsentRenewDue)
}

// IsValidRadius reports whether m is one of AllowedRadiiM.
func IsValidRadius(m int) bool {
	for _, r := range AllowedRadiiM {
		if r == m {
			return true
		}
	}
	return false
}

// IsValidRevealLevel reports whether l is one of the three defined levels.
func IsValidRevealLevel(l int) bool {
	return l == RevealLevelAnonymous || l == RevealLevelConnections || l == RevealLevelOpenDiscovery
}

// IsValidVisibility reports whether v is one of the three defined
// presence_visibility values.
func IsValidVisibility(v string) bool {
	return v == VisibilityInvisible || v == VisibilityFriendsOnly || v == VisibilityEveryone
}

// AuditLogEntry is one append-only row of the presence access audit log
// (security.md §1.8). It is written for every presence query where a
// requester actually received a bucket about a target — not for queries
// that returned nothing (blocked, out of radius, rate-limited, etc.),
// since those don't constitute an access to the target's presence.
type AuditLogEntry struct {
	ID          string
	RequesterID string
	TargetID    string
	OccurredAt  time.Time
	Bucket      DistanceBucket
	Endpoint    string
}

// Sentinel errors. Per backend-go.md §7, handlers/tests assert on these via
// errors.Is; no panics anywhere in this package.
var (
	ErrInvalidInput       = errors.New("presence: invalid input")
	ErrSettingsNotFound   = errors.New("presence: privacy settings not found")
	ErrConsentRequired    = errors.New("presence: proximity consent not granted")
	ErrConsentExpired     = errors.New("presence: proximity consent expired, renewal required")
	ErrCannotBlockSelf    = errors.New("presence: cannot block yourself")
	ErrBlockNotFound      = errors.New("presence: block not found")
	ErrInvalidRadius      = errors.New("presence: visibility_radius_m must be one of 150, 1000, 5000, 15000")
	ErrInvalidRevealLevel = errors.New("presence: reveal_level must be 0, 1 or 2")
	ErrInvalidVisibility  = errors.New("presence: presence_visibility must be invisible, friends_only or everyone")
	ErrIngestSaturated    = errors.New("presence: ingest pipeline saturated, slow down update frequency")
)
