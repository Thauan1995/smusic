// Package catalog implements minimal CRUD + search for tracks, albums, and
// artists (backend-go.md §4's "Catálogo" contracts; data-architecture.md
// §1.2). Search uses Postgres pg_trgm as a fallback engine for this slice —
// TODO(data-architecture.md §5.4): replace with a dedicated search engine
// (Meilisearch recommended) once relevance/fuzzy-matching needs outgrow
// what pg_trgm can do; the CatalogService.Search boundary is designed so
// swapping the backing TrackRepository/AlbumRepository/ArtistRepository
// implementation is the only change needed.
package catalog

import (
	"errors"
	"time"
)

// Artist is a simplified view of data-architecture.md §1.2's artists table.
type Artist struct {
	ID        string
	Name      string
	Slug      string
	Bio       string
	ImageURL  string
	Verified  bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Album types, mirroring albums.album_type.
const (
	AlbumTypeAlbum       = "album"
	AlbumTypeSingle      = "single"
	AlbumTypeEP          = "ep"
	AlbumTypeCompilation = "compilation"
)

// Album is a simplified view of data-architecture.md §1.2's albums table.
type Album struct {
	ID              string
	Title           string
	PrimaryArtistID string // "" if none
	ReleaseDate     *time.Time
	AlbumType       string
	CoverURL        string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// Track roles, mirroring track_artists.role.
const (
	ArtistRolePrimary  = "primary"
	ArtistRoleFeatured = "featured"
	ArtistRoleProducer = "producer"
	ArtistRoleComposer = "composer"
)

// TrackArtist is a denormalized (track_artists join artists) credit line.
type TrackArtist struct {
	ArtistID   string
	ArtistName string
	Role       string
}

// Track is a simplified view of data-architecture.md §1.2's tracks table,
// with its credits (track_artists) attached.
type Track struct {
	ID              string
	Title           string
	AlbumID         string // "" if none (single without a formal album)
	DurationMs      int
	TrackNumber     *int
	ISRC            string
	Explicit        bool
	PopularityScore float32
	Artists         []TrackArtist
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// Audio quality tiers and codecs, mirroring track_audio_assets.
const (
	QualityTierLow      = "low"
	QualityTierNormal   = "normal"
	QualityTierHigh     = "high"
	QualityTierLossless = "lossless"

	CodecAAC  = "aac"
	CodecOpus = "opus"
	CodecFLAC = "flac"
)

// AudioAsset is a single encoded rendition of a track
// (data-architecture.md §1.2 track_audio_assets: 1:N per track).
type AudioAsset struct {
	ID          string
	TrackID     string
	StorageURI  string
	BitrateKbps int
	Codec       string
	QualityTier string
}

// Sentinel errors.
var (
	ErrInvalidInput   = errors.New("catalog: invalid input")
	ErrTrackNotFound  = errors.New("catalog: track not found")
	ErrAlbumNotFound  = errors.New("catalog: album not found")
	ErrArtistNotFound = errors.New("catalog: artist not found")
)

// SearchType selects which entity kind a search targets
// (backend-go.md §4: "type=track|album|artist|playlist" — playlist search
// belongs to the library module and isn't handled here).
type SearchType string

const (
	SearchTypeTrack  SearchType = "track"
	SearchTypeAlbum  SearchType = "album"
	SearchTypeArtist SearchType = "artist"
)
