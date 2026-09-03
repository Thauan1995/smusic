package library

import "context"

// PlaylistRepository persists and retrieves playlists.
type PlaylistRepository interface {
	Create(ctx context.Context, p Playlist) error
	GetByID(ctx context.Context, id string) (Playlist, error)
	ListByOwner(ctx context.Context, ownerID string) ([]Playlist, error)
}

// PlaylistTrackRepository persists and retrieves playlist entries.
type PlaylistTrackRepository interface {
	Add(ctx context.Context, pt PlaylistTrack) error
	Remove(ctx context.Context, playlistID, trackID string) error
	List(ctx context.Context, playlistID string) ([]PlaylistTrack, error)
	// MaxPosition returns the highest Position currently in playlistID, and
	// ok=false if the playlist has no tracks yet.
	MaxPosition(ctx context.Context, playlistID string) (position float64, ok bool, err error)
}

// LibraryTrackRepository persists and retrieves a user's saved tracks
// ("Músicas Curtidas").
type LibraryTrackRepository interface {
	Add(ctx context.Context, lt LibraryTrack) error
	Remove(ctx context.Context, userID, trackID string) error
	List(ctx context.Context, userID, cursor string, limit int) (items []LibraryTrack, nextCursor string, err error)
}

// TrackChecker lets library verify a track exists in the catalog module
// without reaching into catalog's tables directly (backend-go.md §1).
// catalog.Service.TrackExists implements this.
type TrackChecker interface {
	TrackExists(ctx context.Context, trackID string) (bool, error)
}
