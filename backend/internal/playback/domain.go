// Package playback implements playback session state (play/pause/seek/
// next/queue) per backend-go.md §4's Reprodução contracts and §5 ("Estado
// de sessão de reprodução ... Redis como store primário"). Session state
// lives entirely in Redis (internal/playback/redisstore) — never Postgres —
// since it's high-frequency, low-value-per-write, and fine to lose
// (backend-go.md §5: "perda de estado de reprodução = pior caso, o cliente
// re-sincroniza").
//
// The actual audio bytes/CDN path is behind the MediaURLResolver interface
// (media/resolver.go). TODO(backend-go.md §2, media-edge-service): replace
// LocalResolver with real CDN-signed-URL generation once media-edge-service
// is extracted (Fatia 2) — this slice serves a local static test asset
// instead of standing up a CDN.
package playback

import (
	"errors"
	"time"
)

// SessionState is a playback session's current, fully-resynchronizable
// state (backend-go.md §4: GET .../state returns exactly this shape).
type SessionState struct {
	SessionID  string
	UserID     string
	DeviceID   string
	TrackID    string // "" if nothing loaded yet
	PositionMs int
	IsPlaying  bool
	Queue      []string // upcoming track IDs, front = next
	UpdatedAt  time.Time
}

// Sentinel errors.
var (
	ErrInvalidInput    = errors.New("playback: invalid input")
	ErrSessionNotFound = errors.New("playback: session not found")
	ErrForbidden       = errors.New("playback: requester does not own this session")
	ErrTrackNotFound   = errors.New("playback: track not found in catalog")
	ErrEmptyQueue      = errors.New("playback: queue is empty")
)
