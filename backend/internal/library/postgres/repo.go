// Package postgres implements library's repository interfaces against
// Postgres via pgx. Per backend-go.md §7's testing pyramid, this package is
// exercised by integration tests against a real database, not the
// hermetic unit suite — coverage:ignore for the unit-coverage number,
// documented here per 00-overview.md §2. library.Service's business logic
// (ownership checks, position math, validation) is covered there with
// in-memory fakes implementing the same interfaces this file implements
// against Postgres.
//
// Three separate types (PlaylistRepo/PlaylistTrackRepo/LibraryTrackRepo)
// back the three repository interfaces: PlaylistTrackRepository and
// LibraryTrackRepository both declare Add/Remove/List, so one type can't
// implement both.
package postgres

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"smusic/backend/internal/library"
)

// --- PlaylistRepo ---

// PlaylistRepo implements library.PlaylistRepository.
type PlaylistRepo struct {
	pool *pgxpool.Pool
}

// NewPlaylistRepo returns a PlaylistRepo backed by pool.
func NewPlaylistRepo(pool *pgxpool.Pool) *PlaylistRepo { return &PlaylistRepo{pool: pool} }

func (r *PlaylistRepo) Create(ctx context.Context, p library.Playlist) error {
	const q = `
		INSERT INTO playlists (id, owner_id, title, description, visibility, cover_url, created_at, updated_at)
		VALUES ($1, $2, $3, NULLIF($4, ''), $5, NULLIF($6, ''), $7, $7)`
	_, err := r.pool.Exec(ctx, q, p.ID, p.OwnerID, p.Title, p.Description, p.Visibility, p.CoverURL, p.CreatedAt)
	return err
}

func (r *PlaylistRepo) GetByID(ctx context.Context, id string) (library.Playlist, error) {
	const q = `
		SELECT id, owner_id, title, COALESCE(description, ''), visibility, COALESCE(cover_url, ''), created_at, updated_at
		FROM playlists WHERE id = $1`
	var p library.Playlist
	err := r.pool.QueryRow(ctx, q, id).Scan(&p.ID, &p.OwnerID, &p.Title, &p.Description, &p.Visibility, &p.CoverURL, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return library.Playlist{}, library.ErrPlaylistNotFound
	}
	return p, err
}

func (r *PlaylistRepo) ListByOwner(ctx context.Context, ownerID string) ([]library.Playlist, error) {
	const q = `
		SELECT id, owner_id, title, COALESCE(description, ''), visibility, COALESCE(cover_url, ''), created_at, updated_at
		FROM playlists WHERE owner_id = $1 ORDER BY created_at DESC, id`
	rows, err := r.pool.Query(ctx, q, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var playlists []library.Playlist
	for rows.Next() {
		var p library.Playlist
		if err := rows.Scan(&p.ID, &p.OwnerID, &p.Title, &p.Description, &p.Visibility, &p.CoverURL, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		playlists = append(playlists, p)
	}
	return playlists, rows.Err()
}

// --- PlaylistTrackRepo ---

// PlaylistTrackRepo implements library.PlaylistTrackRepository.
type PlaylistTrackRepo struct {
	pool *pgxpool.Pool
}

// NewPlaylistTrackRepo returns a PlaylistTrackRepo backed by pool.
func NewPlaylistTrackRepo(pool *pgxpool.Pool) *PlaylistTrackRepo {
	return &PlaylistTrackRepo{pool: pool}
}

func (r *PlaylistTrackRepo) Add(ctx context.Context, pt library.PlaylistTrack) error {
	const q = `
		INSERT INTO playlist_tracks (id, playlist_id, track_id, position, added_by, added_at)
		VALUES ($1, $2, $3, $4, NULLIF($5, '')::uuid, $6)`
	_, err := r.pool.Exec(ctx, q, pt.ID, pt.PlaylistID, pt.TrackID, pt.Position, pt.AddedBy, pt.AddedAt)
	return err
}

func (r *PlaylistTrackRepo) Remove(ctx context.Context, playlistID, trackID string) error {
	const q = `DELETE FROM playlist_tracks WHERE playlist_id = $1 AND track_id = $2`
	tag, err := r.pool.Exec(ctx, q, playlistID, trackID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return library.ErrTrackNotInPlaylist
	}
	return nil
}

func (r *PlaylistTrackRepo) List(ctx context.Context, playlistID string) ([]library.PlaylistTrack, error) {
	const q = `
		SELECT id, playlist_id, track_id, position, COALESCE(added_by::text, ''), added_at
		FROM playlist_tracks WHERE playlist_id = $1 ORDER BY position ASC`
	rows, err := r.pool.Query(ctx, q, playlistID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []library.PlaylistTrack
	for rows.Next() {
		var pt library.PlaylistTrack
		if err := rows.Scan(&pt.ID, &pt.PlaylistID, &pt.TrackID, &pt.Position, &pt.AddedBy, &pt.AddedAt); err != nil {
			return nil, err
		}
		items = append(items, pt)
	}
	return items, rows.Err()
}

func (r *PlaylistTrackRepo) MaxPosition(ctx context.Context, playlistID string) (float64, bool, error) {
	const q = `SELECT MAX(position) FROM playlist_tracks WHERE playlist_id = $1`
	var max *float64
	if err := r.pool.QueryRow(ctx, q, playlistID).Scan(&max); err != nil {
		return 0, false, err
	}
	if max == nil {
		return 0, false, nil
	}
	return *max, true, nil
}

// --- LibraryTrackRepo ---

// LibraryTrackRepo implements library.LibraryTrackRepository.
type LibraryTrackRepo struct {
	pool *pgxpool.Pool
}

// NewLibraryTrackRepo returns a LibraryTrackRepo backed by pool.
func NewLibraryTrackRepo(pool *pgxpool.Pool) *LibraryTrackRepo { return &LibraryTrackRepo{pool: pool} }

func (r *LibraryTrackRepo) Add(ctx context.Context, lt library.LibraryTrack) error {
	const q = `
		INSERT INTO library_tracks (user_id, track_id, added_at)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id, track_id) DO NOTHING`
	_, err := r.pool.Exec(ctx, q, lt.UserID, lt.TrackID, lt.AddedAt)
	return err
}

func (r *LibraryTrackRepo) Remove(ctx context.Context, userID, trackID string) error {
	const q = `DELETE FROM library_tracks WHERE user_id = $1 AND track_id = $2`
	_, err := r.pool.Exec(ctx, q, userID, trackID)
	return err
}

type cursor struct {
	AddedAt string `json:"a"`
	TrackID string `json:"t"`
}

func (r *LibraryTrackRepo) List(ctx context.Context, userID, cursorStr string, limit int) ([]library.LibraryTrack, string, error) {
	var c cursor
	if cursorStr != "" {
		b, err := base64.RawURLEncoding.DecodeString(cursorStr)
		if err != nil {
			return nil, "", fmt.Errorf("library: invalid cursor: %w", err)
		}
		if err := json.Unmarshal(b, &c); err != nil {
			return nil, "", fmt.Errorf("library: invalid cursor: %w", err)
		}
	}

	// As in catalog/postgres's Search methods: the cursor predicate is only
	// appended when a cursor is present. track_id is a uuid column, so a
	// '' sentinel threaded through a typed tuple comparison would fail to
	// parse rather than just "match nothing".
	q := `SELECT user_id, track_id, added_at FROM library_tracks WHERE user_id = $1`
	args := []any{userID}
	if cursorStr != "" {
		q += ` AND (added_at, track_id) < ($2::timestamptz, $3::uuid)`
		args = append(args, c.AddedAt, c.TrackID)
	}
	q += fmt.Sprintf(" ORDER BY added_at DESC, track_id DESC LIMIT $%d", len(args)+1)
	args = append(args, limit+1)

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	var items []library.LibraryTrack
	for rows.Next() {
		var lt library.LibraryTrack
		if err := rows.Scan(&lt.UserID, &lt.TrackID, &lt.AddedAt); err != nil {
			return nil, "", err
		}
		items = append(items, lt)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}

	var next string
	if len(items) > limit {
		last := items[limit-1]
		b, _ := json.Marshal(cursor{AddedAt: last.AddedAt.Format("2006-01-02T15:04:05.999999999Z07:00"), TrackID: last.TrackID})
		next = base64.RawURLEncoding.EncodeToString(b)
		items = items[:limit]
	}
	return items, next, nil
}
