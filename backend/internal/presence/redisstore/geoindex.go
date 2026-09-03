// Package redisstore implements presence.GeoIndex against Redis
// (data-architecture.md §3/§4: GEOADD/GEOSEARCH + a companion detail hash,
// TTL-bound, never a durable table — security.md §1.5).
//
// Known Redis limitation and how this package works around it: a GEO key
// is implemented internally as a sorted set, and Redis has no native
// per-member TTL on a sorted set (unlike a plain key, or — since Redis
// 7.4 — hash fields via HEXPIRE, which this codebase's go-redis/v9 client
// version predates relying on). So the *hash* (`presence:detail:{userID}`)
// is the source of truth for "is this user still present": it carries a
// real per-key TTL, renewed on every Upsert/Touch. The GEO sorted set
// itself is best-effort and can contain stale members whose detail hash
// has already expired; every read path (Search+Detail) treats "candidate
// in the GEO set but missing from the detail hash" as "not present," which
// is exactly presence.GeoIndex.Detail's documented contract. A periodic
// sweep to proactively ZREM stale GEO members (rather than only filtering
// them lazily at read time) is a documented TODO — not a correctness gap
// (nothing is ever exposed for an expired user), only a memory-tidiness
// one, bounded by presence's own short TTL (90s) naturally limiting how
// stale the GEO set can ever get.
package redisstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"smusic/backend/internal/presence"
)

// geoKey is a single, global Redis GEO key for this slice. data-architecture.md
// §4.4 recommends sharding this by coarse region (e.g. a 2-3 char geohash
// prefix) so a single Redis instance's GEO sorted set doesn't become a
// scaling bottleneck and so Redis Cluster can distribute it. That sharding
// is an orthogonal *scaling* concern, not a *correctness* or *privacy* one
// (GEOSEARCH's radius query is correct regardless of how many logical
// shards the key space is split into) — documented as a deferred TODO,
// consistent with how Fatia 1 deferred gRPC/Prometheus/etc. The GeoIndex
// interface this package implements doesn't leak this decision to callers,
// so adding sharding later only touches this file.
const geoKey = "presence:geo"

func detailKey(userID string) string { return "presence:detail:" + userID }

// Store implements presence.GeoIndex against a Redis client.
type Store struct {
	client *redis.Client
}

// New returns a Store backed by client.
func New(client *redis.Client) *Store {
	return &Store{client: client}
}

type detail struct {
	TrackID    string    `json:"track_id"`
	ShareTrack bool      `json:"share_track"`
	Visibility string    `json:"visibility"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// Upsert implements presence.GeoIndex.
func (s *Store) Upsert(ctx context.Context, entry presence.PresenceEntry, ttl time.Duration) error {
	if err := s.client.GeoAdd(ctx, geoKey, &redis.GeoLocation{
		Name:      entry.UserID,
		Longitude: entry.Position.Lon,
		Latitude:  entry.Position.Lat,
	}).Err(); err != nil {
		return fmt.Errorf("redisstore: geoadd: %w", err)
	}

	d := detail{TrackID: entry.TrackID, ShareTrack: entry.ShareTrack, Visibility: entry.Visibility, UpdatedAt: entry.UpdatedAt}
	b, err := json.Marshal(d)
	if err != nil {
		// coverage:ignore — d is a plain struct of strings/bool/time.Time;
		// none of these can fail json.Marshal. See redisstore's playback
		// counterpart for the same documented reasoning.
		return fmt.Errorf("redisstore: marshal detail: %w", err)
	}
	if err := s.client.Set(ctx, detailKey(entry.UserID), b, ttl).Err(); err != nil {
		// coverage:ignore — requires the Redis connection to succeed on
		// GEOADD and then fail on the very next command (SET); not
		// reproducible with miniredis.SetError, which fails every
		// subsequent command uniformly rather than a specific one (same
		// documented limitation as internal/platform/cache/ratelimiter.go's
		// EXPIRE-after-INCR branch). The general "this Redis command
		// failed" path IS covered (TestStore_Upsert_GeoAddError exercises
		// the GEOADD failure one line above).
		return fmt.Errorf("redisstore: set detail: %w", err)
	}
	return nil
}

// Touch implements presence.GeoIndex.
func (s *Store) Touch(ctx context.Context, userID string, ttl time.Duration) (presence.GeoPosition, bool, error) {
	b, err := s.client.Get(ctx, detailKey(userID)).Bytes()
	if errors.Is(err, redis.Nil) {
		return presence.GeoPosition{}, false, nil
	}
	if err != nil {
		return presence.GeoPosition{}, false, fmt.Errorf("redisstore: get detail: %w", err)
	}

	positions, err := s.client.GeoPos(ctx, geoKey, userID).Result()
	if err != nil {
		return presence.GeoPosition{}, false, fmt.Errorf("redisstore: geopos: %w", err)
	}
	if len(positions) == 0 || positions[0] == nil {
		// Detail hash still alive but the GEO member is gone (e.g. an
		// explicit Remove raced with this call, or the two structures
		// briefly diverged) — treat as "not present" rather than
		// half-resurrecting a position-less entry.
		return presence.GeoPosition{}, false, nil
	}

	if err := s.client.Expire(ctx, detailKey(userID), ttl).Err(); err != nil {
		// coverage:ignore — requires GET and GEOPOS to both succeed and
		// then EXPIRE to fail; same miniredis fault-injection limitation
		// as Upsert's SET branch above.
		return presence.GeoPosition{}, false, fmt.Errorf("redisstore: renew ttl: %w", err)
	}

	var d detail
	if err := json.Unmarshal(b, &d); err != nil {
		// coverage:ignore — b was written by this same Store's Upsert as
		// json.Marshal(detail{...}), a shape this Unmarshal always
		// succeeds against; would only fail if something else wrote
		// malformed JSON to this exact key, outside this package's control.
		return presence.GeoPosition{}, false, fmt.Errorf("redisstore: unmarshal detail: %w", err)
	}

	return presence.GeoPosition{Lat: positions[0].Latitude, Lon: positions[0].Longitude}, true, nil
}

// Remove implements presence.GeoIndex.
func (s *Store) Remove(ctx context.Context, userID string) error {
	if err := s.client.ZRem(ctx, geoKey, userID).Err(); err != nil {
		return fmt.Errorf("redisstore: zrem: %w", err)
	}
	if err := s.client.Del(ctx, detailKey(userID)).Err(); err != nil {
		// coverage:ignore — requires ZREM to succeed and the very next
		// command (DEL) to fail; same miniredis fault-injection limitation
		// noted above.
		return fmt.Errorf("redisstore: del detail: %w", err)
	}
	return nil
}

// Search implements presence.GeoIndex.
//
// Deliberately uses Client.GeoSearch (member names only) followed by a
// GeoPos batch, rather than the single-call Client.GeoSearchLocation —
// go-redis v9 (every version checked, up to and including v9.22.0 as used
// here) has a confirmed bug in GeoSearchLocation: it builds the GEOSEARCH
// command's argument list once in the GeoSearchLocation method and then
// NewGeoSearchLocationCmd's constructor builds and appends the SAME
// FROMLONLAT/BYRADIUS/WITHCOORD arguments a second time, sending Redis a
// command with duplicated tokens that Redis correctly rejects with "ERR
// syntax error" (reproduced against both a real go-redis+miniredis pair in
// this package's tests and confirmed by reading
// geo_commands.go/command.go's source directly). Filed nowhere upstream by
// this project (out of scope to chase during this slice), worked around
// here instead: GeoSearch's argument-building path has no such double-append
// and is unaffected.
func (s *Store) Search(ctx context.Context, pos presence.GeoPosition, radiusM float64, excludeUserID string) ([]presence.NearbyCandidate, error) {
	names, err := s.client.GeoSearch(ctx, geoKey, &redis.GeoSearchQuery{
		Longitude:  pos.Lon,
		Latitude:   pos.Lat,
		Radius:     radiusM,
		RadiusUnit: "m",
	}).Result()
	if err != nil {
		return nil, fmt.Errorf("redisstore: geosearch: %w", err)
	}
	if len(names) == 0 {
		return nil, nil
	}

	positions, err := s.client.GeoPos(ctx, geoKey, names...).Result()
	if err != nil {
		// coverage:ignore — requires GEOSEARCH to succeed and the
		// follow-up GEOPOS to fail; same miniredis fault-injection
		// limitation noted above.
		return nil, fmt.Errorf("redisstore: geopos: %w", err)
	}

	out := make([]presence.NearbyCandidate, 0, len(names))
	for i, name := range names {
		if name == excludeUserID {
			continue
		}
		if i >= len(positions) || positions[i] == nil {
			// coverage:ignore — requires a member to be present for
			// GEOSEARCH and then removed before the immediately-following
			// GEOPOS call within this same method invocation; a genuine
			// race window, not reproducible deterministically in a unit
			// test without sabotaging the Redis client between two calls
			// (miniredis has no hook for that). The general "member absent
			// from GeoPos results" shape is otherwise exercised by every
			// other Search test via real, non-racy data.
			continue
		}
		out = append(out, presence.NearbyCandidate{
			UserID:   name,
			Position: presence.GeoPosition{Lat: positions[i].Latitude, Lon: positions[i].Longitude},
		})
	}
	return out, nil
}

// Detail implements presence.GeoIndex.
func (s *Store) Detail(ctx context.Context, userIDs []string) (map[string]presence.PresenceEntry, error) {
	out := make(map[string]presence.PresenceEntry, len(userIDs))
	if len(userIDs) == 0 {
		return out, nil
	}

	keys := make([]string, len(userIDs))
	for i, id := range userIDs {
		keys[i] = detailKey(id)
	}
	vals, err := s.client.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, fmt.Errorf("redisstore: mget detail: %w", err)
	}

	for i, v := range vals {
		if v == nil {
			continue // expired between Search and Detail — omitted, per GeoIndex.Detail's contract
		}
		str, ok := v.(string)
		if !ok {
			// coverage:ignore — go-redis's MGet always returns either nil
			// or a string for a plain string-valued key (which is all this
			// key ever holds, per Upsert); this defensive branch guards
			// against a key type this package never itself writes.
			continue
		}
		var d detail
		if err := json.Unmarshal([]byte(str), &d); err != nil {
			// coverage:ignore — str was written by this same Store's
			// Upsert as json.Marshal(detail{...}); see Touch's identical
			// justification above.
			return nil, fmt.Errorf("redisstore: unmarshal detail: %w", err)
		}
		out[userIDs[i]] = presence.PresenceEntry{
			UserID:     userIDs[i],
			TrackID:    d.TrackID,
			ShareTrack: d.ShareTrack,
			Visibility: d.Visibility,
			UpdatedAt:  d.UpdatedAt,
		}
	}
	return out, nil
}
