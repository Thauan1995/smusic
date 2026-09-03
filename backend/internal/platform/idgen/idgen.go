// Package idgen isolates entity ID generation behind an interface (backend-go.md
// §7: randomness, like time, must be injected at the edges so domain logic
// stays deterministic and testable). Production IDs are UUIDv7 (data-architecture.md
// §5.5): timestamp-ordered, so B-tree indexes on high-insert-volume tables
// (play_events, playlist_tracks, ...) don't fragment the way random UUIDv4
// would, while still being generated client-side without a DB round-trip.
package idgen

import (
	"fmt"
	"sync/atomic"

	"github.com/google/uuid"
)

// Generator produces new, globally unique entity IDs.
type Generator interface {
	NewID() string
}

// UUIDv7 is the production Generator.
type UUIDv7 struct{}

// NewID returns a new UUIDv7 string.
func (UUIDv7) NewID() string {
	id, err := uuid.NewV7()
	if err != nil {
		// coverage:ignore — uuid.NewV7 only returns an error when the
		// system's crypto/rand source itself fails to read, which is not a
		// condition that can be triggered from a hermetic unit test (it
		// would require sabotaging the OS entropy source). The fallback to
		// a random UUIDv4 keeps ID generation available (favoring
		// availability over strict time-ordering) instead of panicking,
		// consistent with the project's no-panic-for-control-flow rule.
		return uuid.NewString()
	}
	return id.String()
}

// Sequential is a deterministic Generator for tests: it returns predictable,
// strictly increasing IDs so assertions can reference exact values instead
// of "some UUID".
type Sequential struct {
	prefix string
	n      atomic.Int64
}

// NewSequential returns a Sequential generator. prefix is embedded in every
// generated ID to make it easy to tell, in a test failure message, which
// generator produced a given ID.
func NewSequential(prefix string) *Sequential {
	return &Sequential{prefix: prefix}
}

// NewID returns the next id in the sequence. It intentionally does not look
// like a UUID: unit tests use it against fake, in-memory repositories that
// treat IDs as opaque strings, never against a real Postgres uuid column.
func (s *Sequential) NewID() string {
	n := s.n.Add(1)
	return fmt.Sprintf("%s-%d", s.prefix, n)
}
