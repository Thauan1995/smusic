package library

import (
	"context"
	"sort"
	"sync"
)

// --- fakePlaylistRepo ---

type fakePlaylistRepo struct {
	mu        sync.Mutex
	byID      map[string]Playlist
	createErr error
	getErr    error
	listErr   error
}

func newFakePlaylistRepo() *fakePlaylistRepo { return &fakePlaylistRepo{byID: map[string]Playlist{}} }

func (f *fakePlaylistRepo) Create(ctx context.Context, p Playlist) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.createErr != nil {
		return f.createErr
	}
	f.byID[p.ID] = p
	return nil
}

func (f *fakePlaylistRepo) GetByID(ctx context.Context, id string) (Playlist, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.getErr != nil {
		return Playlist{}, f.getErr
	}
	p, ok := f.byID[id]
	if !ok {
		return Playlist{}, ErrPlaylistNotFound
	}
	return p, nil
}

func (f *fakePlaylistRepo) ListByOwner(ctx context.Context, ownerID string) ([]Playlist, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.listErr != nil {
		return nil, f.listErr
	}
	var out []Playlist
	for _, p := range f.byID {
		if p.OwnerID == ownerID {
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// --- fakePlaylistTrackRepo ---

type fakePlaylistTrackRepo struct {
	mu         sync.Mutex
	byPlaylist map[string][]PlaylistTrack
	addErr     error
	removeErr  error
	listErr    error
	maxPosErr  error
}

func newFakePlaylistTrackRepo() *fakePlaylistTrackRepo {
	return &fakePlaylistTrackRepo{byPlaylist: map[string][]PlaylistTrack{}}
}

func (f *fakePlaylistTrackRepo) Add(ctx context.Context, pt PlaylistTrack) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.addErr != nil {
		return f.addErr
	}
	f.byPlaylist[pt.PlaylistID] = append(f.byPlaylist[pt.PlaylistID], pt)
	return nil
}

func (f *fakePlaylistTrackRepo) Remove(ctx context.Context, playlistID, trackID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.removeErr != nil {
		return f.removeErr
	}
	items := f.byPlaylist[playlistID]
	for i, it := range items {
		if it.TrackID == trackID {
			f.byPlaylist[playlistID] = append(items[:i], items[i+1:]...)
			return nil
		}
	}
	return ErrTrackNotInPlaylist
}

func (f *fakePlaylistTrackRepo) List(ctx context.Context, playlistID string) ([]PlaylistTrack, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.listErr != nil {
		return nil, f.listErr
	}
	items := append([]PlaylistTrack(nil), f.byPlaylist[playlistID]...)
	sort.Slice(items, func(i, j int) bool { return items[i].Position < items[j].Position })
	return items, nil
}

func (f *fakePlaylistTrackRepo) MaxPosition(ctx context.Context, playlistID string) (float64, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.maxPosErr != nil {
		return 0, false, f.maxPosErr
	}
	items := f.byPlaylist[playlistID]
	if len(items) == 0 {
		return 0, false, nil
	}
	max := items[0].Position
	for _, it := range items[1:] {
		if it.Position > max {
			max = it.Position
		}
	}
	return max, true, nil
}

// --- fakeLibraryTrackRepo ---

type fakeLibraryTrackRepo struct {
	mu        sync.Mutex
	byUser    map[string]map[string]LibraryTrack
	addErr    error
	removeErr error
	listErr   error
}

func newFakeLibraryTrackRepo() *fakeLibraryTrackRepo {
	return &fakeLibraryTrackRepo{byUser: map[string]map[string]LibraryTrack{}}
}

func (f *fakeLibraryTrackRepo) Add(ctx context.Context, lt LibraryTrack) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.addErr != nil {
		return f.addErr
	}
	if f.byUser[lt.UserID] == nil {
		f.byUser[lt.UserID] = map[string]LibraryTrack{}
	}
	f.byUser[lt.UserID][lt.TrackID] = lt
	return nil
}

func (f *fakeLibraryTrackRepo) Remove(ctx context.Context, userID, trackID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.removeErr != nil {
		return f.removeErr
	}
	delete(f.byUser[userID], trackID)
	return nil
}

func (f *fakeLibraryTrackRepo) List(ctx context.Context, userID, cursor string, limit int) ([]LibraryTrack, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.listErr != nil {
		return nil, "", f.listErr
	}
	var items []LibraryTrack
	for _, lt := range f.byUser[userID] {
		items = append(items, lt)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].TrackID < items[j].TrackID })

	start := 0
	if cursor != "" {
		for i, it := range items {
			if it.TrackID > cursor {
				start = i
				break
			}
			start = i + 1
		}
	}
	rest := items[start:]
	if len(rest) > limit {
		return rest[:limit], rest[limit-1].TrackID, nil
	}
	return rest, "", nil
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
