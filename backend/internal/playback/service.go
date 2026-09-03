package playback

import (
	"context"
	"errors"
	"fmt"
	"time"

	"smusic/backend/internal/platform/clock"
	"smusic/backend/internal/platform/idgen"
)

// Queue insert positions accepted by Enqueue.
const (
	EnqueuePositionNext = "next"
	EnqueuePositionEnd  = "end"
)

// Service implements playback session control. All I/O is behind
// interfaces (backend-go.md §7).
type Service struct {
	store      StateStore
	resolver   MediaURLResolver
	tracks     TrackChecker
	events     PlayEventRecorder
	clock      clock.Clock
	ids        idgen.Generator
	sessionTTL time.Duration
}

// NewService constructs a Service from its dependencies. sessionTTL is the
// Redis TTL applied (and renewed) on every session write.
func NewService(store StateStore, resolver MediaURLResolver, tracks TrackChecker, events PlayEventRecorder, clk clock.Clock, ids idgen.Generator, sessionTTL time.Duration) *Service {
	return &Service{store: store, resolver: resolver, tracks: tracks, events: events, clock: clk, ids: ids, sessionTTL: sessionTTL}
}

// CreateSession starts a new, empty playback session for userID.
func (s *Service) CreateSession(ctx context.Context, userID, deviceID string) (SessionState, error) {
	if userID == "" {
		return SessionState{}, fmt.Errorf("%w: user id is required", ErrInvalidInput)
	}
	state := SessionState{
		SessionID: s.ids.NewID(),
		UserID:    userID,
		DeviceID:  deviceID,
		UpdatedAt: s.clock.Now(),
	}
	if err := s.store.Save(ctx, state, s.sessionTTL); err != nil {
		return SessionState{}, fmt.Errorf("playback: create session: %w", err)
	}
	return state, nil
}

// GetState returns a session's current state.
func (s *Service) GetState(ctx context.Context, sessionID, requesterID string) (SessionState, error) {
	return s.getOwnedSession(ctx, sessionID, requesterID)
}

// PlayResult is returned by Play and Next: a resolved, short-lived stream
// URL for the track that started playing.
type PlayResult struct {
	TrackID   string
	StreamURL string
	ExpiresAt time.Time
	State     SessionState
}

// Play loads trackID into the session at positionMs and starts playback,
// resolving a fresh stream URL (backend-go.md §4: "stream_url ... sempre
// uma URL assinada de curta duração").
func (s *Service) Play(ctx context.Context, sessionID, requesterID, trackID string, positionMs int) (PlayResult, error) {
	if trackID == "" {
		return PlayResult{}, fmt.Errorf("%w: track id is required", ErrInvalidInput)
	}
	if positionMs < 0 {
		return PlayResult{}, fmt.Errorf("%w: position_ms must not be negative", ErrInvalidInput)
	}

	state, err := s.getOwnedSession(ctx, sessionID, requesterID)
	if err != nil {
		return PlayResult{}, err
	}

	streamURL, expiresAt, err := s.loadTrack(ctx, trackID)
	if err != nil {
		return PlayResult{}, err
	}

	state.TrackID = trackID
	state.PositionMs = positionMs
	state.IsPlaying = true
	state.UpdatedAt = s.clock.Now()

	if err := s.saveAndRecord(ctx, &state, trackID); err != nil {
		return PlayResult{}, err
	}

	return PlayResult{TrackID: trackID, StreamURL: streamURL, ExpiresAt: expiresAt, State: state}, nil
}

// Pause stops playback in place, keeping the current track/position.
func (s *Service) Pause(ctx context.Context, sessionID, requesterID string) (SessionState, error) {
	state, err := s.getOwnedSession(ctx, sessionID, requesterID)
	if err != nil {
		return SessionState{}, err
	}
	state.IsPlaying = false
	state.UpdatedAt = s.clock.Now()
	if err := s.store.Save(ctx, state, s.sessionTTL); err != nil {
		return SessionState{}, fmt.Errorf("playback: pause: %w", err)
	}
	return state, nil
}

// Seek moves the playback position within the current track.
func (s *Service) Seek(ctx context.Context, sessionID, requesterID string, positionMs int) (SessionState, error) {
	if positionMs < 0 {
		return SessionState{}, fmt.Errorf("%w: position_ms must not be negative", ErrInvalidInput)
	}
	state, err := s.getOwnedSession(ctx, sessionID, requesterID)
	if err != nil {
		return SessionState{}, err
	}
	state.PositionMs = positionMs
	state.UpdatedAt = s.clock.Now()
	if err := s.store.Save(ctx, state, s.sessionTTL); err != nil {
		return SessionState{}, fmt.Errorf("playback: seek: %w", err)
	}
	return state, nil
}

// Next advances to the next track in the queue.
func (s *Service) Next(ctx context.Context, sessionID, requesterID string) (PlayResult, error) {
	state, err := s.getOwnedSession(ctx, sessionID, requesterID)
	if err != nil {
		return PlayResult{}, err
	}
	if len(state.Queue) == 0 {
		return PlayResult{}, ErrEmptyQueue
	}

	trackID := state.Queue[0]
	streamURL, expiresAt, err := s.loadTrack(ctx, trackID)
	if err != nil {
		return PlayResult{}, err
	}

	state.TrackID = trackID
	state.PositionMs = 0
	state.IsPlaying = true
	state.Queue = state.Queue[1:]
	state.UpdatedAt = s.clock.Now()

	if err := s.saveAndRecord(ctx, &state, trackID); err != nil {
		return PlayResult{}, err
	}

	return PlayResult{TrackID: trackID, StreamURL: streamURL, ExpiresAt: expiresAt, State: state}, nil
}

// Enqueue adds trackIDs to the session's queue, either at the front
// (EnqueuePositionNext) or the back (EnqueuePositionEnd, the default).
func (s *Service) Enqueue(ctx context.Context, sessionID, requesterID string, trackIDs []string, position string) (SessionState, error) {
	if len(trackIDs) == 0 {
		return SessionState{}, fmt.Errorf("%w: at least one track id is required", ErrInvalidInput)
	}
	for _, id := range trackIDs {
		if id == "" {
			return SessionState{}, fmt.Errorf("%w: track ids must not be empty", ErrInvalidInput)
		}
	}

	state, err := s.getOwnedSession(ctx, sessionID, requesterID)
	if err != nil {
		return SessionState{}, err
	}

	for _, id := range trackIDs {
		exists, err := s.tracks.TrackExists(ctx, id)
		if err != nil {
			return SessionState{}, fmt.Errorf("playback: check track exists: %w", err)
		}
		if !exists {
			return SessionState{}, ErrTrackNotFound
		}
	}

	if position == EnqueuePositionNext {
		state.Queue = append(append([]string{}, trackIDs...), state.Queue...)
	} else {
		state.Queue = append(state.Queue, trackIDs...)
	}
	state.UpdatedAt = s.clock.Now()

	if err := s.store.Save(ctx, state, s.sessionTTL); err != nil {
		return SessionState{}, fmt.Errorf("playback: enqueue: %w", err)
	}
	return state, nil
}

func (s *Service) getOwnedSession(ctx context.Context, sessionID, requesterID string) (SessionState, error) {
	if sessionID == "" || requesterID == "" {
		return SessionState{}, fmt.Errorf("%w: session id and requester are required", ErrInvalidInput)
	}
	state, err := s.store.Load(ctx, sessionID)
	if err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			return SessionState{}, ErrSessionNotFound
		}
		return SessionState{}, fmt.Errorf("playback: load session: %w", err)
	}
	if state.UserID != requesterID {
		return SessionState{}, ErrForbidden
	}
	return state, nil
}

func (s *Service) loadTrack(ctx context.Context, trackID string) (string, time.Time, error) {
	exists, err := s.tracks.TrackExists(ctx, trackID)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("playback: check track exists: %w", err)
	}
	if !exists {
		return "", time.Time{}, ErrTrackNotFound
	}
	streamURL, expiresAt, err := s.resolver.Resolve(ctx, trackID)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("playback: resolve media url: %w", err)
	}
	return streamURL, expiresAt, nil
}

func (s *Service) saveAndRecord(ctx context.Context, state *SessionState, trackID string) error {
	if err := s.store.Save(ctx, *state, s.sessionTTL); err != nil {
		return fmt.Errorf("playback: save session: %w", err)
	}
	event := PlayEvent{
		ID:          s.ids.NewID(),
		UserID:      state.UserID,
		TrackID:     trackID,
		DeviceID:    state.DeviceID,
		PlayedAt:    state.UpdatedAt,
		ContextType: "playlist",
	}
	if err := s.events.Record(ctx, event); err != nil {
		return fmt.Errorf("playback: record play event: %w", err)
	}
	return nil
}
