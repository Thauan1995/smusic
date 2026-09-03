package playback

import (
	"context"
	"sync"
	"time"
)

// --- fakeStateStore ---

type fakeStateStore struct {
	mu      sync.Mutex
	byID    map[string]SessionState
	saveErr error
	loadErr error
	delErr  error
}

func newFakeStateStore() *fakeStateStore { return &fakeStateStore{byID: map[string]SessionState{}} }

func (f *fakeStateStore) Save(ctx context.Context, s SessionState, ttl time.Duration) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.saveErr != nil {
		return f.saveErr
	}
	f.byID[s.SessionID] = s
	return nil
}

func (f *fakeStateStore) Load(ctx context.Context, sessionID string) (SessionState, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.loadErr != nil {
		return SessionState{}, f.loadErr
	}
	s, ok := f.byID[sessionID]
	if !ok {
		return SessionState{}, ErrSessionNotFound
	}
	return s, nil
}

func (f *fakeStateStore) Delete(ctx context.Context, sessionID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.delErr != nil {
		return f.delErr
	}
	delete(f.byID, sessionID)
	return nil
}

// --- fakeResolver ---

type fakeResolver struct {
	mu         sync.Mutex
	resolveErr error
	calls      int
}

func (f *fakeResolver) Resolve(ctx context.Context, trackID string) (string, time.Time, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.resolveErr != nil {
		return "", time.Time{}, f.resolveErr
	}
	f.calls++
	return "https://media.example/" + trackID, time.Now().Add(10 * time.Minute), nil
}

// --- fakeTrackChecker ---

type fakeTrackChecker struct {
	mu       sync.Mutex
	existing map[string]bool
	err      error
}

func newFakeTrackChecker(existing ...string) *fakeTrackChecker {
	m := map[string]bool{}
	for _, id := range existing {
		m[id] = true
	}
	return &fakeTrackChecker{existing: m}
}

func (f *fakeTrackChecker) TrackExists(ctx context.Context, trackID string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return false, f.err
	}
	return f.existing[trackID], nil
}

// --- fakeEventRecorder ---

type fakeEventRecorder struct {
	mu     sync.Mutex
	events []PlayEvent
	err    error
}

func (f *fakeEventRecorder) Record(ctx context.Context, e PlayEvent) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.events = append(f.events, e)
	return nil
}
