package idgen

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUUIDv7_NewID(t *testing.T) {
	gen := UUIDv7{}

	a := gen.NewID()
	b := gen.NewID()

	require.NotEqual(t, a, b)

	parsed, err := uuid.Parse(a)
	require.NoError(t, err)
	assert.Equal(t, uuid.Version(7), parsed.Version())
}

func TestSequential_NewID(t *testing.T) {
	gen := NewSequential("test")

	a := gen.NewID()
	b := gen.NewID()
	c := gen.NewID()

	assert.Equal(t, "test-1", a)
	assert.Equal(t, "test-2", b)
	assert.Equal(t, "test-3", c)
}

func TestSequential_NewID_ConcurrentUse(t *testing.T) {
	gen := NewSequential("race")
	const n = 100
	ids := make(chan string, n)

	for i := 0; i < n; i++ {
		go func() { ids <- gen.NewID() }()
	}

	seen := make(map[string]bool, n)
	for i := 0; i < n; i++ {
		id := <-ids
		require.False(t, seen[id], "duplicate id generated under concurrent use: %s", id)
		seen[id] = true
	}
}
