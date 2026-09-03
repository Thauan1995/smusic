package catalog

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"smusic/backend/internal/platform/clock"
	"smusic/backend/internal/platform/idgen"
)

// defaultSearchLimit and maxSearchLimit bound the page size backend-go.md
// §4's cursor-paginated search endpoint accepts.
const (
	defaultSearchLimit = 20
	maxSearchLimit     = 100
)

// Service implements catalog CRUD + search. All I/O is behind interfaces
// (backend-go.md §7), so business rules (validation, default limits) are
// unit-testable with in-memory fakes.
type Service struct {
	artists ArtistRepository
	albums  AlbumRepository
	tracks  TrackRepository
	clock   clock.Clock
	ids     idgen.Generator
}

// NewService constructs a Service from its dependencies.
func NewService(artists ArtistRepository, albums AlbumRepository, tracks TrackRepository, clk clock.Clock, ids idgen.Generator) *Service {
	return &Service{artists: artists, albums: albums, tracks: tracks, clock: clk, ids: ids}
}

// CreateArtistInput is the input to Service.CreateArtist.
type CreateArtistInput struct {
	Name     string
	Slug     string
	Bio      string
	ImageURL string
}

// CreateArtist validates and stores a new artist.
func (s *Service) CreateArtist(ctx context.Context, in CreateArtistInput) (Artist, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return Artist{}, fmt.Errorf("%w: name is required", ErrInvalidInput)
	}

	now := s.clock.Now()
	a := Artist{
		ID:        s.ids.NewID(),
		Name:      name,
		Slug:      strings.TrimSpace(in.Slug),
		Bio:       in.Bio,
		ImageURL:  in.ImageURL,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.artists.Create(ctx, a); err != nil {
		return Artist{}, fmt.Errorf("catalog: create artist: %w", err)
	}
	return a, nil
}

// GetArtist returns an artist by ID.
func (s *Service) GetArtist(ctx context.Context, id string) (Artist, error) {
	if id == "" {
		return Artist{}, fmt.Errorf("%w: id is required", ErrInvalidInput)
	}
	a, err := s.artists.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, ErrArtistNotFound) {
			return Artist{}, ErrArtistNotFound
		}
		return Artist{}, fmt.Errorf("catalog: get artist: %w", err)
	}
	return a, nil
}

// CreateAlbumInput is the input to Service.CreateAlbum.
type CreateAlbumInput struct {
	Title           string
	PrimaryArtistID string
	AlbumType       string
	CoverURL        string
}

// CreateAlbum validates and stores a new album.
func (s *Service) CreateAlbum(ctx context.Context, in CreateAlbumInput) (Album, error) {
	title := strings.TrimSpace(in.Title)
	if title == "" {
		return Album{}, fmt.Errorf("%w: title is required", ErrInvalidInput)
	}
	albumType := in.AlbumType
	if albumType == "" {
		albumType = AlbumTypeAlbum
	}
	if !isValidAlbumType(albumType) {
		return Album{}, fmt.Errorf("%w: invalid album_type %q", ErrInvalidInput, albumType)
	}

	if in.PrimaryArtistID != "" {
		if _, err := s.GetArtist(ctx, in.PrimaryArtistID); err != nil {
			if errors.Is(err, ErrArtistNotFound) {
				return Album{}, fmt.Errorf("%w: primary_artist_id does not exist", ErrInvalidInput)
			}
			return Album{}, err
		}
	}

	now := s.clock.Now()
	a := Album{
		ID:              s.ids.NewID(),
		Title:           title,
		PrimaryArtistID: in.PrimaryArtistID,
		AlbumType:       albumType,
		CoverURL:        in.CoverURL,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := s.albums.Create(ctx, a); err != nil {
		return Album{}, fmt.Errorf("catalog: create album: %w", err)
	}
	return a, nil
}

// GetAlbum returns an album by ID, with its tracks.
func (s *Service) GetAlbum(ctx context.Context, id string) (Album, []Track, error) {
	if id == "" {
		return Album{}, nil, fmt.Errorf("%w: id is required", ErrInvalidInput)
	}
	album, err := s.albums.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, ErrAlbumNotFound) {
			return Album{}, nil, ErrAlbumNotFound
		}
		return Album{}, nil, fmt.Errorf("catalog: get album: %w", err)
	}
	tracks, err := s.tracks.ListByAlbum(ctx, id)
	if err != nil {
		return Album{}, nil, fmt.Errorf("catalog: list album tracks: %w", err)
	}
	return album, tracks, nil
}

// CreateTrackInput is the input to Service.CreateTrack.
type CreateTrackInput struct {
	Title       string
	AlbumID     string
	DurationMs  int
	TrackNumber *int
	ISRC        string
	Explicit    bool
	Artists     []TrackArtist
	Assets      []AudioAsset
}

// CreateTrack validates and stores a new track with its credits and
// (optional) initial audio assets.
func (s *Service) CreateTrack(ctx context.Context, in CreateTrackInput) (Track, error) {
	title := strings.TrimSpace(in.Title)
	if title == "" {
		return Track{}, fmt.Errorf("%w: title is required", ErrInvalidInput)
	}
	if in.DurationMs <= 0 {
		return Track{}, fmt.Errorf("%w: duration_ms must be positive", ErrInvalidInput)
	}
	if len(in.Artists) == 0 {
		return Track{}, fmt.Errorf("%w: at least one artist credit is required", ErrInvalidInput)
	}
	for _, ta := range in.Artists {
		if ta.ArtistID == "" || !isValidArtistRole(ta.Role) {
			return Track{}, fmt.Errorf("%w: invalid artist credit", ErrInvalidInput)
		}
	}

	if in.AlbumID != "" {
		if _, err := s.albums.GetByID(ctx, in.AlbumID); err != nil {
			if errors.Is(err, ErrAlbumNotFound) {
				return Track{}, fmt.Errorf("%w: album_id does not exist", ErrInvalidInput)
			}
			return Track{}, fmt.Errorf("catalog: get album: %w", err)
		}
	}

	now := s.clock.Now()
	t := Track{
		ID:          s.ids.NewID(),
		Title:       title,
		AlbumID:     in.AlbumID,
		DurationMs:  in.DurationMs,
		TrackNumber: in.TrackNumber,
		ISRC:        in.ISRC,
		Explicit:    in.Explicit,
		Artists:     in.Artists,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.tracks.Create(ctx, t, in.Assets); err != nil {
		return Track{}, fmt.Errorf("catalog: create track: %w", err)
	}
	return t, nil
}

// GetTrack returns a track by ID, with its available audio assets.
func (s *Service) GetTrack(ctx context.Context, id string) (Track, []AudioAsset, error) {
	if id == "" {
		return Track{}, nil, fmt.Errorf("%w: id is required", ErrInvalidInput)
	}
	track, err := s.tracks.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, ErrTrackNotFound) {
			return Track{}, nil, ErrTrackNotFound
		}
		return Track{}, nil, fmt.Errorf("catalog: get track: %w", err)
	}
	assets, err := s.tracks.ListAudioAssets(ctx, id)
	if err != nil {
		return Track{}, nil, fmt.Errorf("catalog: list audio assets: %w", err)
	}
	return track, assets, nil
}

// TrackExists reports whether a track with the given ID exists. It backs
// the library module's TrackChecker interface (backend-go.md §1: modules
// only reach each other through explicit interfaces, never another
// domain's tables directly).
func (s *Service) TrackExists(ctx context.Context, id string) (bool, error) {
	_, _, err := s.GetTrack(ctx, id)
	if err != nil {
		if errors.Is(err, ErrTrackNotFound) || errors.Is(err, ErrInvalidInput) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// SearchInput is the input to Service.Search.
type SearchInput struct {
	Query  string
	Type   SearchType
	Cursor string
	Limit  int
}

// SearchOutput carries whichever result slice matches the requested Type;
// the other two are always empty.
type SearchOutput struct {
	Tracks     []Track
	Albums     []Album
	Artists    []Artist
	NextCursor string
}

// Search dispatches to the repository matching in.Type (default: track).
func (s *Service) Search(ctx context.Context, in SearchInput) (SearchOutput, error) {
	q := strings.TrimSpace(in.Query)
	if q == "" {
		return SearchOutput{}, fmt.Errorf("%w: q is required", ErrInvalidInput)
	}

	limit := in.Limit
	if limit <= 0 {
		limit = defaultSearchLimit
	}
	if limit > maxSearchLimit {
		limit = maxSearchLimit
	}

	searchType := in.Type
	if searchType == "" {
		searchType = SearchTypeTrack
	}

	switch searchType {
	case SearchTypeTrack:
		page, err := s.tracks.Search(ctx, q, in.Cursor, limit)
		if err != nil {
			return SearchOutput{}, fmt.Errorf("catalog: search tracks: %w", err)
		}
		return SearchOutput{Tracks: page.Items, NextCursor: page.NextCursor}, nil
	case SearchTypeAlbum:
		page, err := s.albums.Search(ctx, q, in.Cursor, limit)
		if err != nil {
			return SearchOutput{}, fmt.Errorf("catalog: search albums: %w", err)
		}
		return SearchOutput{Albums: page.Items, NextCursor: page.NextCursor}, nil
	case SearchTypeArtist:
		page, err := s.artists.Search(ctx, q, in.Cursor, limit)
		if err != nil {
			return SearchOutput{}, fmt.Errorf("catalog: search artists: %w", err)
		}
		return SearchOutput{Artists: page.Items, NextCursor: page.NextCursor}, nil
	default:
		return SearchOutput{}, fmt.Errorf("%w: unsupported type %q", ErrInvalidInput, searchType)
	}
}

func isValidAlbumType(t string) bool {
	switch t {
	case AlbumTypeAlbum, AlbumTypeSingle, AlbumTypeEP, AlbumTypeCompilation:
		return true
	default:
		return false
	}
}

func isValidArtistRole(r string) bool {
	switch r {
	case ArtistRolePrimary, ArtistRoleFeatured, ArtistRoleProducer, ArtistRoleComposer:
		return true
	default:
		return false
	}
}
