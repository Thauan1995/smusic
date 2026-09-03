package playback

import (
	"context"
	"time"
)

// StateStore persists ephemeral playback session state (backend-go.md §5:
// Redis is the primary store, not Postgres). redisstore.Store implements
// this against real Redis; unit tests use an in-memory fake.
type StateStore interface {
	Save(ctx context.Context, s SessionState, ttl time.Duration) error
	Load(ctx context.Context, sessionID string) (SessionState, error)
	Delete(ctx context.Context, sessionID string) error
}

// MediaURLResolver resolves a track to a short-lived, playable URL.
// media.LocalResolver implements this for this slice (see domain.go's
// package doc for the media-edge-service TODO).
type MediaURLResolver interface {
	Resolve(ctx context.Context, trackID string) (url string, expiresAt time.Time, err error)
}

// TrackChecker lets playback verify a track exists in the catalog module
// without reaching into catalog's tables directly (backend-go.md §1).
// catalog.Service.TrackExists implements this.
type TrackChecker interface {
	TrackExists(ctx context.Context, trackID string) (bool, error)
}

// PlayEventRecorder records a "play started" event for history/analytics
// (data-architecture.md §1.4 play_events). Optional in the sense that a
// nil-safe no-op implementation is provided (NoopPlayEventRecorder) for
// deployments that don't need it yet.
type PlayEventRecorder interface {
	Record(ctx context.Context, e PlayEvent) error
}

// PlayEvent is the minimal fact record backing data-architecture.md §1.4's
// play_events table.
type PlayEvent struct {
	ID          string
	UserID      string
	TrackID     string
	DeviceID    string
	PlayedAt    time.Time
	ContextType string
}

// NoopPlayEventRecorder discards every event. Used where history recording
// isn't wired up yet.
type NoopPlayEventRecorder struct{}

// Record does nothing and never errors.
func (NoopPlayEventRecorder) Record(context.Context, PlayEvent) error { return nil }
