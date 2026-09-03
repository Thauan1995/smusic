// Package redisstore implements playback.StateStore against Redis
// (backend-go.md §5: "Estado de sessão de reprodução (playback-state):
// Redis como store primário de estado efêmero (não Postgres)").
//
// Unlike most repository implementations in this codebase, this one is
// unit-tested directly (store_test.go) using miniredis — a pure-Go,
// in-memory Redis implementation — rather than deferred to the
// integration tier. That's possible here specifically because Redis
// commands (unlike arbitrary SQL) have a small, well-defined surface that
// miniredis faithfully emulates; it isn't possible for the Postgres
// repositories elsewhere in this codebase, which is why those remain
// integration-tested only (documented in each of their package docs).
package redisstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"smusic/backend/internal/playback"
)

const keyPrefix = "playback:session:"

// Store implements playback.StateStore against a Redis client.
type Store struct {
	client *redis.Client
}

// New returns a Store backed by client.
func New(client *redis.Client) *Store {
	return &Store{client: client}
}

func sessionKey(id string) string { return keyPrefix + id }

// Save writes s, overwriting any previous state for the same session, with
// TTL renewed to ttl (backend-go.md §5: TTL renewed on every activity so
// an idle session eventually expires and frees memory).
func (s *Store) Save(ctx context.Context, state playback.SessionState, ttl time.Duration) error {
	b, err := json.Marshal(state)
	if err != nil {
		// coverage:ignore — SessionState is a plain struct of strings,
		// ints, a bool, a []string and a time.Time; none of these can fail
		// to marshal. Forcing a failure would require injecting a
		// non-marshalable type into the struct, which isn't a real
		// failure mode this code needs to defend against.
		return fmt.Errorf("redisstore: marshal state: %w", err)
	}
	if err := s.client.Set(ctx, sessionKey(state.SessionID), b, ttl).Err(); err != nil {
		return fmt.Errorf("redisstore: save: %w", err)
	}
	return nil
}

// Load returns the current state for sessionID, or
// playback.ErrSessionNotFound if it doesn't exist (never existed, or
// expired).
func (s *Store) Load(ctx context.Context, sessionID string) (playback.SessionState, error) {
	b, err := s.client.Get(ctx, sessionKey(sessionID)).Bytes()
	if errors.Is(err, redis.Nil) {
		return playback.SessionState{}, playback.ErrSessionNotFound
	}
	if err != nil {
		return playback.SessionState{}, fmt.Errorf("redisstore: load: %w", err)
	}
	var state playback.SessionState
	if err := json.Unmarshal(b, &state); err != nil {
		return playback.SessionState{}, fmt.Errorf("redisstore: unmarshal state: %w", err)
	}
	return state, nil
}

// Delete removes a session's state. Deleting a nonexistent session is not
// an error (idempotent).
func (s *Store) Delete(ctx context.Context, sessionID string) error {
	if err := s.client.Del(ctx, sessionKey(sessionID)).Err(); err != nil {
		return fmt.Errorf("redisstore: delete: %w", err)
	}
	return nil
}
