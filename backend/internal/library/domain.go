// Package library implements playlists and favorites ("Músicas Curtidas")
// per backend-go.md §4's Biblioteca contracts and data-architecture.md
// §1.3. It never touches catalog's tables directly — track existence is
// checked through the TrackChecker interface (backend-go.md §1: "nunca
// acesso direto a tabelas de outro domínio").
package library

import (
	"errors"
	"time"
)

// Playlist visibility, mirroring data-architecture.md §1.3's
// playlists.visibility enum.
const (
	VisibilityPrivate       = "private"
	VisibilityUnlisted      = "unlisted"
	VisibilityPublic        = "public"
	VisibilityCollaborative = "collaborative"
)

// Playlist is a simplified view of data-architecture.md §1.3's playlists
// table.
type Playlist struct {
	ID          string
	OwnerID     string
	Title       string
	Description string
	Visibility  string
	CoverURL    string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// PlaylistTrack is one entry in a playlist (data-architecture.md §1.3
// playlist_tracks). Position is fractional (NUMERIC in Postgres) so
// inserting between two existing entries never requires reindexing the
// whole playlist.
type PlaylistTrack struct {
	ID         string
	PlaylistID string
	TrackID    string
	Position   float64
	AddedBy    string
	AddedAt    time.Time
}

// LibraryTrack is a "saved"/favorited track (data-architecture.md §1.3
// library_tracks).
type LibraryTrack struct {
	UserID  string
	TrackID string
	AddedAt time.Time
}

// Sentinel errors.
var (
	ErrInvalidInput       = errors.New("library: invalid input")
	ErrPlaylistNotFound   = errors.New("library: playlist not found")
	ErrNotOwner           = errors.New("library: requester does not own this playlist")
	ErrTrackNotFound      = errors.New("library: track not found in catalog")
	ErrTrackNotInPlaylist = errors.New("library: track not in playlist")
)
