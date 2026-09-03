package presence

import (
	"context"
	"time"
)

// PresenceEntry is the ephemeral, per-user record held in the presence
// index while a user is actively discoverable. Every field here is
// short-lived (TTL-bound, security.md §1.5) and Position is ALWAYS the
// jittered position (see GeoPosition's doc comment) — this type is never
// constructed with a raw coordinate.
type PresenceEntry struct {
	UserID     string
	Position   GeoPosition
	TrackID    string // "" if nothing playing or not shared
	ShareTrack bool   // mirrors PrivacySettings.PresenceShareTrack at write time
	Visibility string // mirrors PrivacySettings.PresenceVisibility at write time
	UpdatedAt  time.Time
}

// NearbyCandidate is one raw hit from a geospatial search: just enough to
// run the distance/radius/rate-limit gauntlet before deciding whether it's
// worth spending a second Redis round-trip (Detail) hydrating the rest.
type NearbyCandidate struct {
	UserID   string
	Position GeoPosition
}

// GeoIndex is the presence-service's ephemeral geospatial store —
// data-architecture.md §3/§4: Redis GEOADD/GEOSEARCH, TTL-based expiry,
// never a durable table. internal/presence/redisstore implements this;
// NearbyService depends only on this interface (backend-go.md §7).
type GeoIndex interface {
	// Upsert (re)inserts/replaces entry, resetting its TTL to ttl. Called
	// on every client "update" frame (backend-go.md §4).
	Upsert(ctx context.Context, entry PresenceEntry, ttl time.Duration) error
	// Touch renews the TTL of userID's existing entry to ttl without
	// changing its position (called on a client "heartbeat" frame, which
	// per backend-go.md §4 carries no coordinates). ok is false if no
	// entry exists (never inserted, or already expired) — the caller
	// should NOT report an error to the client in that case; it should
	// prompt the client to send a fresh "update" instead.
	Touch(ctx context.Context, userID string, ttl time.Duration) (pos GeoPosition, ok bool, err error)
	// Remove deletes userID from the index immediately — security.md
	// §1.4's "efeito imediato" for pause/invisible/consent-revoke/disconnect.
	// Removing an absent user is not an error (idempotent).
	Remove(ctx context.Context, userID string) error
	// Search returns every candidate within radiusM meters of pos,
	// excluding excludeUserID. Order is unspecified.
	Search(ctx context.Context, pos GeoPosition, radiusM float64, excludeUserID string) ([]NearbyCandidate, error)
	// Detail hydrates the non-positional fields for userIDs. IDs whose
	// entry has expired between Search and this call are simply omitted
	// from the result map (not an error) — a natural consequence of TTL
	// expiry racing a concurrent query, treated as "that user is no longer
	// present" rather than a failure.
	Detail(ctx context.Context, userIDs []string) (map[string]PresenceEntry, error)
}
