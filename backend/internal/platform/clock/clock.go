// Package clock isolates access to wall-clock time behind an interface, so
// domain/service code never calls time.Now() directly. This is required by
// backend-go.md §7 ("Isolamento de I/O nas bordas"): time is one of the two
// classic sources of non-determinism in unit tests, and injecting it lets
// every time-dependent business rule (token expiry, TTL renewal, "added_at"
// timestamps) be tested deterministically with Frozen.
package clock

import "time"

// Clock returns the current time. Production code uses Real; tests use
// Frozen (or Advancing) to get deterministic, reproducible timestamps.
type Clock interface {
	Now() time.Time
}

// Real is the production Clock, backed by time.Now().
type Real struct{}

// Now returns the current UTC time.
func (Real) Now() time.Time { return time.Now().UTC() }

// Frozen is a test Clock that always returns the same instant unless
// advanced explicitly.
type Frozen struct {
	t time.Time
}

// NewFrozen returns a Frozen clock starting at t.
func NewFrozen(t time.Time) *Frozen { return &Frozen{t: t} }

// Now returns the frozen instant.
func (f *Frozen) Now() time.Time { return f.t }

// Advance moves the frozen clock forward by d and returns the new instant.
func (f *Frozen) Advance(d time.Duration) time.Time {
	f.t = f.t.Add(d)
	return f.t
}
