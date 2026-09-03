package clock

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestReal_Now(t *testing.T) {
	before := time.Now().UTC()
	got := Real{}.Now()
	after := time.Now().UTC()

	assert.False(t, got.Before(before))
	assert.False(t, got.After(after))
	assert.Equal(t, time.UTC, got.Location())
}

func TestFrozen(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	f := NewFrozen(start)

	assert.Equal(t, start, f.Now())
	assert.Equal(t, start, f.Now(), "Now() must not mutate state")

	next := f.Advance(time.Hour)
	assert.Equal(t, start.Add(time.Hour), next)
	assert.Equal(t, start.Add(time.Hour), f.Now())
}
