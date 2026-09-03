package library

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"smusic/backend/internal/platform/clock"
	"smusic/backend/internal/platform/idgen"
)

const (
	// positionGap is the fractional spacing left between consecutive
	// playlist entries (data-architecture.md §1.3: "usar espaçamento
	// fracionário ... para evitar reindexação em massa a cada
	// drag-and-drop"). A large gap means many inserts can happen between
	// any two tracks before a rebalance is ever needed.
	positionGap = 1024.0

	defaultSavedTracksLimit = 50
	maxSavedTracksLimit     = 200
)

// Position hints accepted by AddTrack.
const (
	PositionStart = "start"
	PositionEnd   = "end"
)

// Service implements playlists and favorites. All I/O is behind
// interfaces (backend-go.md §7).
type Service struct {
	playlists      PlaylistRepository
	playlistTracks PlaylistTrackRepository
	libraryTracks  LibraryTrackRepository
	tracks         TrackChecker
	clock          clock.Clock
	ids            idgen.Generator
}

// NewService constructs a Service from its dependencies.
func NewService(playlists PlaylistRepository, playlistTracks PlaylistTrackRepository, libraryTracks LibraryTrackRepository, tracks TrackChecker, clk clock.Clock, ids idgen.Generator) *Service {
	return &Service{
		playlists:      playlists,
		playlistTracks: playlistTracks,
		libraryTracks:  libraryTracks,
		tracks:         tracks,
		clock:          clk,
		ids:            ids,
	}
}

// CreatePlaylistInput is the input to Service.CreatePlaylist.
type CreatePlaylistInput struct {
	OwnerID     string
	Title       string
	Description string
	Visibility  string
}

// CreatePlaylist validates and stores a new playlist.
func (s *Service) CreatePlaylist(ctx context.Context, in CreatePlaylistInput) (Playlist, error) {
	if in.OwnerID == "" {
		return Playlist{}, fmt.Errorf("%w: owner is required", ErrInvalidInput)
	}
	title := strings.TrimSpace(in.Title)
	if title == "" {
		return Playlist{}, fmt.Errorf("%w: title is required", ErrInvalidInput)
	}
	visibility := in.Visibility
	if visibility == "" {
		visibility = VisibilityPrivate
	}
	if !isValidVisibility(visibility) {
		return Playlist{}, fmt.Errorf("%w: invalid visibility %q", ErrInvalidInput, visibility)
	}

	now := s.clock.Now()
	p := Playlist{
		ID:          s.ids.NewID(),
		OwnerID:     in.OwnerID,
		Title:       title,
		Description: in.Description,
		Visibility:  visibility,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.playlists.Create(ctx, p); err != nil {
		return Playlist{}, fmt.Errorf("library: create playlist: %w", err)
	}
	return p, nil
}

// ListPlaylists returns every playlist owned by ownerID.
func (s *Service) ListPlaylists(ctx context.Context, ownerID string) ([]Playlist, error) {
	if ownerID == "" {
		return nil, fmt.Errorf("%w: owner is required", ErrInvalidInput)
	}
	playlists, err := s.playlists.ListByOwner(ctx, ownerID)
	if err != nil {
		return nil, fmt.Errorf("library: list playlists: %w", err)
	}
	return playlists, nil
}

// ListPlaylistTracks returns a playlist's entries. A private playlist is
// only visible to its owner; any other visibility is viewable by anyone
// (collaborative editing by non-owners is a documented TODO — out of
// scope for this slice, only the owner may mutate a playlist below).
func (s *Service) ListPlaylistTracks(ctx context.Context, playlistID, requesterID string) (Playlist, []PlaylistTrack, error) {
	if playlistID == "" {
		return Playlist{}, nil, fmt.Errorf("%w: playlist id is required", ErrInvalidInput)
	}
	p, err := s.getPlaylist(ctx, playlistID)
	if err != nil {
		return Playlist{}, nil, err
	}
	if p.Visibility == VisibilityPrivate && p.OwnerID != requesterID {
		return Playlist{}, nil, ErrNotOwner
	}

	tracks, err := s.playlistTracks.List(ctx, playlistID)
	if err != nil {
		return Playlist{}, nil, fmt.Errorf("library: list playlist tracks: %w", err)
	}
	return p, tracks, nil
}

// AddTrack appends (or, with position=PositionStart, prepends) trackID to
// playlistID. Only the playlist's owner may add tracks in this slice.
func (s *Service) AddTrack(ctx context.Context, playlistID, requesterID, trackID, position string) (PlaylistTrack, error) {
	if trackID == "" {
		return PlaylistTrack{}, fmt.Errorf("%w: track id is required", ErrInvalidInput)
	}
	if _, err := s.getOwnedPlaylist(ctx, playlistID, requesterID); err != nil {
		return PlaylistTrack{}, err
	}

	exists, err := s.tracks.TrackExists(ctx, trackID)
	if err != nil {
		return PlaylistTrack{}, fmt.Errorf("library: check track exists: %w", err)
	}
	if !exists {
		return PlaylistTrack{}, ErrTrackNotFound
	}

	pos, err := s.computePosition(ctx, playlistID, position)
	if err != nil {
		return PlaylistTrack{}, err
	}

	pt := PlaylistTrack{
		ID:         s.ids.NewID(),
		PlaylistID: playlistID,
		TrackID:    trackID,
		Position:   pos,
		AddedBy:    requesterID,
		AddedAt:    s.clock.Now(),
	}
	if err := s.playlistTracks.Add(ctx, pt); err != nil {
		return PlaylistTrack{}, fmt.Errorf("library: add track: %w", err)
	}
	return pt, nil
}

// RemoveTrack removes trackID from playlistID. Only the playlist's owner
// may remove tracks.
func (s *Service) RemoveTrack(ctx context.Context, playlistID, requesterID, trackID string) error {
	if trackID == "" {
		return fmt.Errorf("%w: track id is required", ErrInvalidInput)
	}
	if _, err := s.getOwnedPlaylist(ctx, playlistID, requesterID); err != nil {
		return err
	}
	if err := s.playlistTracks.Remove(ctx, playlistID, trackID); err != nil {
		if errors.Is(err, ErrTrackNotInPlaylist) {
			return ErrTrackNotInPlaylist
		}
		return fmt.Errorf("library: remove track: %w", err)
	}
	return nil
}

// SaveTrack adds trackID to userID's favorites ("Músicas Curtidas").
// Idempotent by design at the repository layer (adding an already-saved
// track is not an error).
func (s *Service) SaveTrack(ctx context.Context, userID, trackID string) error {
	if userID == "" || trackID == "" {
		return fmt.Errorf("%w: user and track are required", ErrInvalidInput)
	}
	exists, err := s.tracks.TrackExists(ctx, trackID)
	if err != nil {
		return fmt.Errorf("library: check track exists: %w", err)
	}
	if !exists {
		return ErrTrackNotFound
	}
	if err := s.libraryTracks.Add(ctx, LibraryTrack{UserID: userID, TrackID: trackID, AddedAt: s.clock.Now()}); err != nil {
		return fmt.Errorf("library: save track: %w", err)
	}
	return nil
}

// UnsaveTrack removes trackID from userID's favorites. Idempotent: removing
// a track that was never saved is not an error.
func (s *Service) UnsaveTrack(ctx context.Context, userID, trackID string) error {
	if userID == "" || trackID == "" {
		return fmt.Errorf("%w: user and track are required", ErrInvalidInput)
	}
	if err := s.libraryTracks.Remove(ctx, userID, trackID); err != nil {
		return fmt.Errorf("library: unsave track: %w", err)
	}
	return nil
}

// ListSavedTracks returns a keyset-paginated page of userID's favorites.
func (s *Service) ListSavedTracks(ctx context.Context, userID, cursor string, limit int) ([]LibraryTrack, string, error) {
	if userID == "" {
		return nil, "", fmt.Errorf("%w: user is required", ErrInvalidInput)
	}
	if limit <= 0 {
		limit = defaultSavedTracksLimit
	}
	if limit > maxSavedTracksLimit {
		limit = maxSavedTracksLimit
	}
	items, next, err := s.libraryTracks.List(ctx, userID, cursor, limit)
	if err != nil {
		return nil, "", fmt.Errorf("library: list saved tracks: %w", err)
	}
	return items, next, nil
}

func (s *Service) getPlaylist(ctx context.Context, playlistID string) (Playlist, error) {
	p, err := s.playlists.GetByID(ctx, playlistID)
	if err != nil {
		if errors.Is(err, ErrPlaylistNotFound) {
			return Playlist{}, ErrPlaylistNotFound
		}
		return Playlist{}, fmt.Errorf("library: get playlist: %w", err)
	}
	return p, nil
}

func (s *Service) getOwnedPlaylist(ctx context.Context, playlistID, requesterID string) (Playlist, error) {
	if playlistID == "" || requesterID == "" {
		return Playlist{}, fmt.Errorf("%w: playlist id and requester are required", ErrInvalidInput)
	}
	p, err := s.getPlaylist(ctx, playlistID)
	if err != nil {
		return Playlist{}, err
	}
	if p.OwnerID != requesterID {
		return Playlist{}, ErrNotOwner
	}
	return p, nil
}

func (s *Service) computePosition(ctx context.Context, playlistID, position string) (float64, error) {
	if position == PositionStart {
		// List's contract (both the fake and the real Postgres repo) is to
		// return entries ordered ascending by position, so items[0] — if
		// present — is always the current minimum; no need to scan.
		items, err := s.playlistTracks.List(ctx, playlistID)
		if err != nil {
			return 0, fmt.Errorf("library: list playlist tracks: %w", err)
		}
		if len(items) == 0 {
			return positionGap, nil
		}
		return items[0].Position / 2, nil
	}

	maxPos, ok, err := s.playlistTracks.MaxPosition(ctx, playlistID)
	if err != nil {
		return 0, fmt.Errorf("library: max position: %w", err)
	}
	if !ok {
		return positionGap, nil
	}
	return maxPos + positionGap, nil
}

func isValidVisibility(v string) bool {
	switch v {
	case VisibilityPrivate, VisibilityUnlisted, VisibilityPublic, VisibilityCollaborative:
		return true
	default:
		return false
	}
}
