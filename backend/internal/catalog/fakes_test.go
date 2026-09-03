package catalog

import (
	"context"
	"sort"
	"strings"
	"sync"
)

// --- fakeArtistRepo ---

type fakeArtistRepo struct {
	mu        sync.Mutex
	byID      map[string]Artist
	createErr error
	getErr    error
	searchErr error
}

func newFakeArtistRepo() *fakeArtistRepo { return &fakeArtistRepo{byID: map[string]Artist{}} }

func (f *fakeArtistRepo) Create(ctx context.Context, a Artist) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.createErr != nil {
		return f.createErr
	}
	f.byID[a.ID] = a
	return nil
}

func (f *fakeArtistRepo) GetByID(ctx context.Context, id string) (Artist, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.getErr != nil {
		return Artist{}, f.getErr
	}
	a, ok := f.byID[id]
	if !ok {
		return Artist{}, ErrArtistNotFound
	}
	return a, nil
}

func (f *fakeArtistRepo) Search(ctx context.Context, q, cursor string, limit int) (Page[Artist], error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.searchErr != nil {
		return Page[Artist]{}, f.searchErr
	}
	var matches []Artist
	for _, a := range f.byID {
		if strings.Contains(strings.ToLower(a.Name), strings.ToLower(q)) {
			matches = append(matches, a)
		}
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i].Name < matches[j].Name })
	return paginate(matches, cursor, limit, func(a Artist) string { return a.Name + "|" + a.ID }), nil
}

// --- fakeAlbumRepo ---

type fakeAlbumRepo struct {
	mu        sync.Mutex
	byID      map[string]Album
	createErr error
	getErr    error
	listErr   error
	searchErr error
}

func newFakeAlbumRepo() *fakeAlbumRepo { return &fakeAlbumRepo{byID: map[string]Album{}} }

func (f *fakeAlbumRepo) Create(ctx context.Context, a Album) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.createErr != nil {
		return f.createErr
	}
	f.byID[a.ID] = a
	return nil
}

func (f *fakeAlbumRepo) GetByID(ctx context.Context, id string) (Album, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.getErr != nil {
		return Album{}, f.getErr
	}
	a, ok := f.byID[id]
	if !ok {
		return Album{}, ErrAlbumNotFound
	}
	return a, nil
}

func (f *fakeAlbumRepo) ListByArtist(ctx context.Context, artistID string) ([]Album, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.listErr != nil {
		return nil, f.listErr
	}
	var out []Album
	for _, a := range f.byID {
		if a.PrimaryArtistID == artistID {
			out = append(out, a)
		}
	}
	return out, nil
}

func (f *fakeAlbumRepo) Search(ctx context.Context, q, cursor string, limit int) (Page[Album], error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.searchErr != nil {
		return Page[Album]{}, f.searchErr
	}
	var matches []Album
	for _, a := range f.byID {
		if strings.Contains(strings.ToLower(a.Title), strings.ToLower(q)) {
			matches = append(matches, a)
		}
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i].Title < matches[j].Title })
	return paginate(matches, cursor, limit, func(a Album) string { return a.Title + "|" + a.ID }), nil
}

// --- fakeTrackRepo ---

type fakeTrackRepo struct {
	mu        sync.Mutex
	byID      map[string]Track
	assets    map[string][]AudioAsset
	createErr error
	getErr    error
	listErr   error
	searchErr error
	assetsErr error
}

func newFakeTrackRepo() *fakeTrackRepo {
	return &fakeTrackRepo{byID: map[string]Track{}, assets: map[string][]AudioAsset{}}
}

func (f *fakeTrackRepo) Create(ctx context.Context, t Track, assets []AudioAsset) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.createErr != nil {
		return f.createErr
	}
	f.byID[t.ID] = t
	f.assets[t.ID] = assets
	return nil
}

func (f *fakeTrackRepo) GetByID(ctx context.Context, id string) (Track, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.getErr != nil {
		return Track{}, f.getErr
	}
	t, ok := f.byID[id]
	if !ok {
		return Track{}, ErrTrackNotFound
	}
	return t, nil
}

func (f *fakeTrackRepo) ListByAlbum(ctx context.Context, albumID string) ([]Track, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.listErr != nil {
		return nil, f.listErr
	}
	var out []Track
	for _, t := range f.byID {
		if t.AlbumID == albumID {
			out = append(out, t)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (f *fakeTrackRepo) Search(ctx context.Context, q, cursor string, limit int) (Page[Track], error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.searchErr != nil {
		return Page[Track]{}, f.searchErr
	}
	var matches []Track
	for _, t := range f.byID {
		if strings.Contains(strings.ToLower(t.Title), strings.ToLower(q)) {
			matches = append(matches, t)
		}
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i].Title < matches[j].Title })
	return paginate(matches, cursor, limit, func(t Track) string { return t.Title + "|" + t.ID }), nil
}

func (f *fakeTrackRepo) ListAudioAssets(ctx context.Context, trackID string) ([]AudioAsset, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.assetsErr != nil {
		return nil, f.assetsErr
	}
	return f.assets[trackID], nil
}

// paginate is a tiny "peek ahead" keyset-pagination helper shared by the
// fakes, mirroring the real Postgres implementation's contract (opaque
// cursor = last-seen sort key; NextCursor == "" means no more pages).
func paginate[T any](sorted []T, cursor string, limit int, keyOf func(T) string) Page[T] {
	start := 0
	if cursor != "" {
		for i, item := range sorted {
			if keyOf(item) > cursor {
				start = i
				break
			}
			start = i + 1
		}
	}
	rest := sorted[start:]
	if len(rest) > limit {
		return Page[T]{Items: rest[:limit], NextCursor: keyOf(rest[limit-1])}
	}
	return Page[T]{Items: rest}
}
