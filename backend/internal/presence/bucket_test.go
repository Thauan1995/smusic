package presence

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDistanceMeters_SamePoint(t *testing.T) {
	p := GeoPosition{Lat: -23.5505, Lon: -46.6333}
	assert.InDelta(t, 0, DistanceMeters(p, p), 0.001)
}

func TestDistanceMeters_KnownDistance(t *testing.T) {
	// São Paulo to Rio de Janeiro is roughly 357km (great-circle).
	sp := GeoPosition{Lat: -23.5505, Lon: -46.6333}
	rj := GeoPosition{Lat: -22.9068, Lon: -43.1729}
	d := DistanceMeters(sp, rj)
	assert.InDelta(t, 357000, d, 15000) // within 15km of the known approximate value
}

func TestDistanceMeters_Symmetric(t *testing.T) {
	a := GeoPosition{Lat: 10, Lon: 20}
	b := GeoPosition{Lat: 10.01, Lon: 20.02}
	assert.InDelta(t, DistanceMeters(a, b), DistanceMeters(b, a), 0.0001)
}

func TestBucketFor_Boundaries(t *testing.T) {
	cases := []struct {
		distance float64
		want     DistanceBucket
	}{
		{0, Bucket1},
		{149.999, Bucket1},
		{150, Bucket2},
		{999.999, Bucket2},
		{1000, Bucket3},
		{4999.999, Bucket3},
		{5000, Bucket4},
		{14999.999, Bucket4},
		{15000, BucketNone},
		{20000, BucketNone},
	}
	for _, c := range cases {
		assert.Equalf(t, c.want, BucketFor(c.distance), "distance=%v", c.distance)
	}
}

func TestDistanceBucket_Label(t *testing.T) {
	assert.Equal(t, "Bem pertinho", Bucket1.Label())
	assert.Equal(t, "No seu bairro", Bucket2.Label())
	assert.Equal(t, "Na sua região", Bucket3.Label())
	assert.Equal(t, "Na sua cidade", Bucket4.Label())
	assert.Equal(t, "", BucketNone.Label())
}

// TestFixedJitterer_OffsetMagnitude verifies offsetMeters (exercised via
// FixedJitterer) actually displaces the position by approximately the
// requested distance, using DistanceMeters as an independent check.
func TestFixedJitterer_OffsetMagnitude(t *testing.T) {
	origin := GeoPosition{Lat: -23.5505, Lon: -46.6333}
	j := FixedJitterer{NorthM: 75, EastM: 0}
	moved, err := j.Jitter(origin)
	assert.NoError(t, err)
	assert.InDelta(t, 75, DistanceMeters(origin, moved), 0.5)
	assert.Greater(t, moved.Lat, origin.Lat) // north = increasing latitude
}

func TestFixedJitterer_EastOffset(t *testing.T) {
	origin := GeoPosition{Lat: 0, Lon: 0}
	j := FixedJitterer{NorthM: 0, EastM: 100}
	moved, err := j.Jitter(origin)
	assert.NoError(t, err)
	assert.InDelta(t, 100, DistanceMeters(origin, moved), 0.5)
	assert.Greater(t, moved.Lon, origin.Lon)
}

// TestRandJitterer_WithinRadius asserts the privacy-critical invariant that
// jitter NEVER displaces a position by more than JitterRadiusM, across many
// samples.
func TestRandJitterer_WithinRadius(t *testing.T) {
	origin := GeoPosition{Lat: -23.5505, Lon: -46.6333}
	j := RandJitterer{}
	for i := 0; i < 2000; i++ {
		moved, err := j.Jitter(origin)
		assert.NoError(t, err)
		d := DistanceMeters(origin, moved)
		assert.LessOrEqualf(t, d, JitterRadiusM+0.01, "jitter exceeded radius on iteration %d: %f", i, d)
	}
}

// TestRandJitterer_VariesAcrossCalls is the privacy invariant from the
// task: jitter must NOT be fixed per user — it must differ between
// heartbeats. A fixed offset would let an attacker average repeated
// observations back to the true position.
func TestRandJitterer_VariesAcrossCalls(t *testing.T) {
	origin := GeoPosition{Lat: -23.5505, Lon: -46.6333}
	j := RandJitterer{}
	first, err := j.Jitter(origin)
	assert.NoError(t, err)
	distinct := 0
	for i := 0; i < 50; i++ {
		next, err := j.Jitter(origin)
		assert.NoError(t, err)
		if next != first {
			distinct++
		}
	}
	// Overwhelmingly likely all 50 differ from the first; require most to,
	// to avoid any (astronomically unlikely) flake from the RNG.
	assert.Greater(t, distinct, 45)
}

// TestRandJitterer_UniformOverArea checks the jitter isn't biased toward
// the center (which sampling radius uniformly, instead of radius*sqrt(u),
// would cause) — a coarse statistical sanity check, not a rigorous
// distribution test.
func TestRandJitterer_UniformOverArea(t *testing.T) {
	origin := GeoPosition{Lat: 0, Lon: 0}
	j := RandJitterer{}
	const n = 4000
	nearCenter := 0 // within half the radius
	for i := 0; i < n; i++ {
		moved, err := j.Jitter(origin)
		assert.NoError(t, err)
		if DistanceMeters(origin, moved) < JitterRadiusM/2 {
			nearCenter++
		}
	}
	// Uniform-over-area expectation: area of half-radius disk / area of
	// full disk = 1/4, so ~25% should land within half the radius. A
	// radius-uniform (buggy) sampler would put ~50% within half the
	// radius. Allow generous slack for randomness.
	frac := float64(nearCenter) / n
	assert.Less(t, frac, 0.35, "jitter appears biased toward the center (radius-uniform instead of area-uniform)")
}

func TestSequenceJitterer_CyclesThenRepeatsLast(t *testing.T) {
	origin := GeoPosition{Lat: 0, Lon: 0}
	s := &SequenceJitterer{Offsets: []FixedJitterer{
		{NorthM: 10},
		{NorthM: 20},
	}}
	first, err := s.Jitter(origin)
	assert.NoError(t, err)
	second, err := s.Jitter(origin)
	assert.NoError(t, err)
	third, err := s.Jitter(origin) // exhausted, repeats last offset (20m)
	assert.NoError(t, err)

	assert.NotEqual(t, first, second)
	assert.Equal(t, second, third)
	assert.InDelta(t, 10, DistanceMeters(origin, first), 0.5)
	assert.InDelta(t, 20, DistanceMeters(origin, second), 0.5)
}

func TestSequenceJitterer_Empty(t *testing.T) {
	origin := GeoPosition{Lat: 1, Lon: 2}
	s := &SequenceJitterer{}
	moved, err := s.Jitter(origin)
	assert.NoError(t, err)
	assert.Equal(t, origin, moved)
}

func TestEarthRadiusConsistency(t *testing.T) {
	// Sanity check the offsetMeters/DistanceMeters round trip is internally
	// consistent for a range of magnitudes.
	for _, m := range []float64{1, 10, 75, 1000, 5000} {
		origin := GeoPosition{Lat: 45, Lon: 45}
		moved := offsetMeters(origin, m, 0)
		assert.InDelta(t, m, DistanceMeters(origin, moved), math.Max(0.5, m*0.01))
	}
}
