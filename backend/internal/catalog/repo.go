package catalog

import "context"

// Page is a keyset-paginated result: cursor-based, not offset-based, per
// backend-go.md §4 ("offset degrada em catálogos grandes e é inconsistente
// sob escrita concorrente"). NextCursor is "" when there is no further
// page.
type Page[T any] struct {
	Items      []T
	NextCursor string
}

// ArtistRepository persists and retrieves artists.
type ArtistRepository interface {
	Create(ctx context.Context, a Artist) error
	GetByID(ctx context.Context, id string) (Artist, error)
	// Search returns artists whose name matches q (pg_trgm fallback,
	// data-architecture.md §5.4), ordered by (name, id) for stable keyset
	// pagination. cursor is the opaque token from a previous Page's
	// NextCursor, or "" for the first page.
	Search(ctx context.Context, q, cursor string, limit int) (Page[Artist], error)
}

// AlbumRepository persists and retrieves albums.
type AlbumRepository interface {
	Create(ctx context.Context, a Album) error
	GetByID(ctx context.Context, id string) (Album, error)
	ListByArtist(ctx context.Context, artistID string) ([]Album, error)
	Search(ctx context.Context, q, cursor string, limit int) (Page[Album], error)
}

// TrackRepository persists and retrieves tracks and their credits/assets.
type TrackRepository interface {
	// Create stores t along with its artist credits (t.Artists) and any
	// initial audio assets.
	Create(ctx context.Context, t Track, assets []AudioAsset) error
	GetByID(ctx context.Context, id string) (Track, error)
	ListByAlbum(ctx context.Context, albumID string) ([]Track, error)
	Search(ctx context.Context, q, cursor string, limit int) (Page[Track], error)
	ListAudioAssets(ctx context.Context, trackID string) ([]AudioAsset, error)
}
