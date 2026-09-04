package presence

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"math"
)

// GeoPosition is a plain lat/lon pair. It is used ONLY as an in-process,
// transient value inside this package and internal/presence/redisstore —
// per security.md §1.5/§4.3 ("a coordenada bruta ... nunca é persistida em
// texto claro associada ao user_id em nenhum armazenamento de longa
// duração"), this type is never round-tripped through Postgres, never
// logged, and — going one step further than the letter of security.md §1.2
// (which only mandates jitter "before computing the bucket") — this
// implementation never even writes the RAW value to Redis: jitter is
// applied once, in NearbyService, immediately after a raw coordinate is
// received off the wire, and only the jittered GeoPosition is ever stored
// or compared again. See NearbyService.ApplyUpdate's doc comment.
type GeoPosition struct {
	Lat, Lon float64
}

const earthRadiusM = 6371000.0

// DistanceMeters returns the great-circle (haversine) distance between a
// and b in meters. Accurate enough at the scales this feature cares about
// (150m–15km); no need for a more precise ellipsoidal model.
func DistanceMeters(a, b GeoPosition) float64 {
	lat1 := a.Lat * math.Pi / 180
	lat2 := b.Lat * math.Pi / 180
	dLat := (b.Lat - a.Lat) * math.Pi / 180
	dLon := (b.Lon - a.Lon) * math.Pi / 180

	sinDLat2 := math.Sin(dLat / 2)
	sinDLon2 := math.Sin(dLon / 2)
	h := sinDLat2*sinDLat2 + math.Cos(lat1)*math.Cos(lat2)*sinDLon2*sinDLon2
	c := 2 * math.Atan2(math.Sqrt(h), math.Sqrt(1-h))
	return earthRadiusM * c
}

// JitterRadiusM is security.md §1.2's mandatory spatial jitter magnitude
// ("±75 m"), applied server-side before any bucket is computed, and
// renewed (re-randomized) on every heartbeat/update — never fixed per
// user, specifically so repeated observation can't be used to average the
// noise away.
const JitterRadiusM = 75.0

// Jitterer displaces a raw position by a random offset. Extracted as an
// interface — like clock.Clock and idgen.Generator elsewhere in this
// codebase — so tests can inject deterministic or property-checked
// randomness instead of depending on the real RNG (backend-go.md §7:
// "Isolamento de I/O nas bordas... geração de aleatoriedade... sempre
// injetada via interface").
type Jitterer interface {
	Jitter(pos GeoPosition) (GeoPosition, error)
}

// RandJitterer is the production Jitterer: displaces pos by a
// uniformly-random point inside a disk of radius JitterRadiusM (uniform
// over the disk's AREA, not over the radius — sampling r uniformly in
// [0, R) would bias samples toward the center; the sqrt below corrects
// that). Uses crypto/rand (via secureFloat64), not math/rand — jitter is
// this product's core anti-triangulation defense (security.md §1.2/§6:
// "um bypass do jitter/bucket pode pontuar CVSS médio mas é crítico para
// este produto"), so its unpredictability is treated as a real security
// property, not just "not fixed per user"; the extra crypto/rand call is
// negligible next to one presence heartbeat's Redis round-trip. Returns
// an error (rather than the panic a cryptographic helper might otherwise
// reach for) for the same reason token.SecureRefreshGenerator.New and
// password.Hasher.Hash do: a failing crypto/rand.Read is threaded up as
// a normal Go error, per this project's no-panic-for-control-flow rule.
type RandJitterer struct{}

// Jitter implements Jitterer.
func (RandJitterer) Jitter(pos GeoPosition) (GeoPosition, error) {
	a, err := secureFloat64()
	if err != nil {
		return GeoPosition{}, fmt.Errorf("presence: jitter: %w", err)
	}
	r, err := secureFloat64()
	if err != nil {
		return GeoPosition{}, fmt.Errorf("presence: jitter: %w", err)
	}
	angle := a * 2 * math.Pi
	radius := JitterRadiusM * math.Sqrt(r)
	return offsetMeters(pos, radius*math.Cos(angle), radius*math.Sin(angle)), nil
}

// secureFloat64 returns a cryptographically random float64 in [0, 1),
// using crypto/rand rather than math/rand — see RandJitterer's doc
// comment for why that matters here. Standard technique: draw 53 random
// mantissa bits (float64's precision) from a uint64 and scale by 2^-53.
//
// coverage:ignore — the error path below (crypto/rand.Read failing) means
// the OS entropy source itself is broken; same documented-impossible-in-
// practice branch shape as password.Hasher.Hash's salt generation (see
// argon2.go), which this mirrors deliberately for consistency.
func secureFloat64() (float64, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return 0, err
	}
	const mantissaBits = 53
	n := binary.BigEndian.Uint64(b[:]) >> (64 - mantissaBits)
	return float64(n) / float64(uint64(1)<<mantissaBits), nil
}

// offsetMeters returns pos displaced by (dNorthM, dEastM) meters, using the
// standard equirectangular approximation — accurate to well under a meter
// of error at the ≤15km scales this feature operates at, which is
// negligible next to the 75m jitter itself.
func offsetMeters(pos GeoPosition, dNorthM, dEastM float64) GeoPosition {
	dLat := dNorthM / earthRadiusM * (180 / math.Pi)
	dLon := dEastM / (earthRadiusM * math.Cos(pos.Lat*math.Pi/180)) * (180 / math.Pi)
	return GeoPosition{Lat: pos.Lat + dLat, Lon: pos.Lon + dLon}
}

// FixedJitterer is a deterministic Jitterer for tests: it always applies
// the same (north, east) meter offset, regardless of input position.
type FixedJitterer struct {
	NorthM, EastM float64
}

// Jitter implements Jitterer.
func (f FixedJitterer) Jitter(pos GeoPosition) (GeoPosition, error) {
	return offsetMeters(pos, f.NorthM, f.EastM), nil
}

// SequenceJitterer is a deterministic Jitterer for tests that need jitter
// to differ across successive calls (e.g. to assert "jitter varies between
// heartbeats" without depending on real randomness). It cycles through
// Offsets in order, repeating the last one once exhausted.
type SequenceJitterer struct {
	Offsets []FixedJitterer
	n       int
}

// Jitter implements Jitterer.
func (s *SequenceJitterer) Jitter(pos GeoPosition) (GeoPosition, error) {
	i := s.n
	if i >= len(s.Offsets) {
		i = len(s.Offsets) - 1
	}
	if i < 0 {
		return pos, nil
	}
	s.n++
	return s.Offsets[i].Jitter(pos)
}

// BucketFor maps a (post-jitter) distance in meters to security.md §1.2's
// 4-tier relative-distance bucket. Returns BucketNone for distances at or
// beyond 15km — the product's hard ceiling (security.md §1.3): such a pair
// should already have been excluded by the mutual-radius check before
// BucketFor is ever called, so BucketNone here is a defensive fallback, not
// an expected path.
func BucketFor(distanceM float64) DistanceBucket {
	switch {
	case distanceM < 150:
		return Bucket1
	case distanceM < 1000:
		return Bucket2
	case distanceM < 5000:
		return Bucket3
	case distanceM < 15000:
		return Bucket4
	default:
		return BucketNone
	}
}
