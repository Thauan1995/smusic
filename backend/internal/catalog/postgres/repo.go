// Package postgres implements catalog's repository interfaces against
// Postgres via pgx, using pg_trgm for the search fallback
// (data-architecture.md §5.4 TODO: dedicated search engine later).
//
// Per backend-go.md §7's testing pyramid, this package is exercised by
// integration tests against a real database, not the hermetic unit suite —
// coverage:ignore for the unit-coverage number, documented here per
// 00-overview.md §2. catalog.Service's validation/dispatch logic (which
// this package fronts) is covered there with in-memory fakes implementing
// the same repository interfaces this file implements against Postgres.
//
// Three separate types (ArtistRepo/AlbumRepo/TrackRepo), each backed by the
// same pool, implement catalog's three repository interfaces: a single
// type can't implement all three, since each interface declares its own
// Create/GetByID/Search with different signatures.
//
// Search ordering: all three Search methods use simple keyset pagination
// ordered by (name-or-title ASC, id ASC), filtered by a pg_trgm similarity
// threshold. This is a deliberate simplification versus popularity-ranked
// results — data-architecture.md §5.4 already earmarks a dedicated search
// engine (Meilisearch) for real relevance ranking; pg_trgm here is
// explicitly the low-traffic fallback, not the final search experience.
package postgres

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"smusic/backend/internal/catalog"
)

// similarityThreshold is the minimum pg_trgm similarity() score for a row
// to be considered a match; below this, results are noise.
const similarityThreshold = 0.15

// --- cursor helpers (shared by all three repos) ---

type nameCursor struct {
	Name string `json:"n"`
	ID   string `json:"id"`
}

func encodeNameCursor(name, id string) string {
	b, _ := json.Marshal(nameCursor{Name: name, ID: id})
	return base64.RawURLEncoding.EncodeToString(b)
}

func decodeNameCursor(cursor string) (nameCursor, error) {
	if cursor == "" {
		return nameCursor{}, nil
	}
	b, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return nameCursor{}, fmt.Errorf("catalog: invalid cursor: %w", err)
	}
	var c nameCursor
	if err := json.Unmarshal(b, &c); err != nil {
		return nameCursor{}, fmt.Errorf("catalog: invalid cursor: %w", err)
	}
	return c, nil
}

// --- ArtistRepo ---

// ArtistRepo implements catalog.ArtistRepository.
type ArtistRepo struct {
	pool *pgxpool.Pool
}

// NewArtistRepo returns an ArtistRepo backed by pool.
func NewArtistRepo(pool *pgxpool.Pool) *ArtistRepo { return &ArtistRepo{pool: pool} }

func (r *ArtistRepo) Create(ctx context.Context, a catalog.Artist) error {
	const q = `
		INSERT INTO artists (id, name, slug, bio, image_url, verified, created_at, updated_at)
		VALUES ($1, $2, NULLIF($3, ''), NULLIF($4, ''), NULLIF($5, ''), $6, $7, $7)`
	_, err := r.pool.Exec(ctx, q, a.ID, a.Name, a.Slug, a.Bio, a.ImageURL, a.Verified, a.CreatedAt)
	return err
}

func (r *ArtistRepo) GetByID(ctx context.Context, id string) (catalog.Artist, error) {
	const q = `
		SELECT id, name, COALESCE(slug, ''), COALESCE(bio, ''), COALESCE(image_url, ''), verified, created_at, updated_at
		FROM artists WHERE id = $1`
	var a catalog.Artist
	err := r.pool.QueryRow(ctx, q, id).Scan(&a.ID, &a.Name, &a.Slug, &a.Bio, &a.ImageURL, &a.Verified, &a.CreatedAt, &a.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return catalog.Artist{}, catalog.ErrArtistNotFound
	}
	return a, err
}

func (r *ArtistRepo) Search(ctx context.Context, q, cursor string, limit int) (catalog.Page[catalog.Artist], error) {
	c, err := decodeNameCursor(cursor)
	if err != nil {
		return catalog.Page[catalog.Artist]{}, err
	}

	// The cursor predicate is only appended when a cursor is present:
	// comparing an empty-string sentinel against the `id` column (uuid)
	// would fail to parse ('' is not a valid uuid), so "no cursor yet"
	// must be a genuinely absent clause, not a sentinel value threaded
	// through a typed tuple comparison.
	query := `
		SELECT id, name, COALESCE(slug, ''), COALESCE(bio, ''), COALESCE(image_url, ''), verified, created_at, updated_at
		FROM artists
		WHERE similarity(name, $1) > $2`
	args := []any{q, similarityThreshold}
	if cursor != "" {
		query += ` AND (name, id) > ($3, $4::uuid)`
		args = append(args, c.Name, c.ID)
	}
	query += fmt.Sprintf(" ORDER BY name ASC, id ASC LIMIT $%d", len(args)+1)
	args = append(args, limit+1)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return catalog.Page[catalog.Artist]{}, err
	}
	defer rows.Close()

	var items []catalog.Artist
	for rows.Next() {
		var a catalog.Artist
		if err := rows.Scan(&a.ID, &a.Name, &a.Slug, &a.Bio, &a.ImageURL, &a.Verified, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return catalog.Page[catalog.Artist]{}, err
		}
		items = append(items, a)
	}
	if err := rows.Err(); err != nil {
		return catalog.Page[catalog.Artist]{}, err
	}

	var next string
	if len(items) > limit {
		last := items[limit-1]
		next = encodeNameCursor(last.Name, last.ID)
		items = items[:limit]
	}
	return catalog.Page[catalog.Artist]{Items: items, NextCursor: next}, nil
}

// --- AlbumRepo ---

// AlbumRepo implements catalog.AlbumRepository.
type AlbumRepo struct {
	pool *pgxpool.Pool
}

// NewAlbumRepo returns an AlbumRepo backed by pool.
func NewAlbumRepo(pool *pgxpool.Pool) *AlbumRepo { return &AlbumRepo{pool: pool} }

func (r *AlbumRepo) Create(ctx context.Context, a catalog.Album) error {
	const q = `
		INSERT INTO albums (id, title, primary_artist_id, release_date, album_type, cover_url, created_at, updated_at)
		VALUES ($1, $2, NULLIF($3, '')::uuid, $4, $5, NULLIF($6, ''), $7, $7)`
	_, err := r.pool.Exec(ctx, q, a.ID, a.Title, a.PrimaryArtistID, a.ReleaseDate, a.AlbumType, a.CoverURL, a.CreatedAt)
	return err
}

func (r *AlbumRepo) GetByID(ctx context.Context, id string) (catalog.Album, error) {
	const q = `
		SELECT id, title, COALESCE(primary_artist_id::text, ''), release_date, album_type, COALESCE(cover_url, ''), created_at, updated_at
		FROM albums WHERE id = $1`
	var a catalog.Album
	err := r.pool.QueryRow(ctx, q, id).Scan(&a.ID, &a.Title, &a.PrimaryArtistID, &a.ReleaseDate, &a.AlbumType, &a.CoverURL, &a.CreatedAt, &a.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return catalog.Album{}, catalog.ErrAlbumNotFound
	}
	return a, err
}

func (r *AlbumRepo) ListByArtist(ctx context.Context, artistID string) ([]catalog.Album, error) {
	const q = `
		SELECT id, title, COALESCE(primary_artist_id::text, ''), release_date, album_type, COALESCE(cover_url, ''), created_at, updated_at
		FROM albums WHERE primary_artist_id = $1 ORDER BY release_date DESC NULLS LAST, id`
	rows, err := r.pool.Query(ctx, q, artistID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var albums []catalog.Album
	for rows.Next() {
		var a catalog.Album
		if err := rows.Scan(&a.ID, &a.Title, &a.PrimaryArtistID, &a.ReleaseDate, &a.AlbumType, &a.CoverURL, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		albums = append(albums, a)
	}
	return albums, rows.Err()
}

func (r *AlbumRepo) Search(ctx context.Context, q, cursor string, limit int) (catalog.Page[catalog.Album], error) {
	c, err := decodeNameCursor(cursor)
	if err != nil {
		return catalog.Page[catalog.Album]{}, err
	}

	query := `
		SELECT id, title, COALESCE(primary_artist_id::text, ''), release_date, album_type, COALESCE(cover_url, ''), created_at, updated_at
		FROM albums
		WHERE similarity(title, $1) > $2`
	args := []any{q, similarityThreshold}
	if cursor != "" {
		query += ` AND (title, id) > ($3, $4::uuid)`
		args = append(args, c.Name, c.ID)
	}
	query += fmt.Sprintf(" ORDER BY title ASC, id ASC LIMIT $%d", len(args)+1)
	args = append(args, limit+1)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return catalog.Page[catalog.Album]{}, err
	}
	defer rows.Close()

	var items []catalog.Album
	for rows.Next() {
		var a catalog.Album
		if err := rows.Scan(&a.ID, &a.Title, &a.PrimaryArtistID, &a.ReleaseDate, &a.AlbumType, &a.CoverURL, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return catalog.Page[catalog.Album]{}, err
		}
		items = append(items, a)
	}
	if err := rows.Err(); err != nil {
		return catalog.Page[catalog.Album]{}, err
	}

	var next string
	if len(items) > limit {
		last := items[limit-1]
		next = encodeNameCursor(last.Title, last.ID)
		items = items[:limit]
	}
	return catalog.Page[catalog.Album]{Items: items, NextCursor: next}, nil
}

// --- TrackRepo ---

// TrackRepo implements catalog.TrackRepository.
type TrackRepo struct {
	pool *pgxpool.Pool
}

// NewTrackRepo returns a TrackRepo backed by pool.
func NewTrackRepo(pool *pgxpool.Pool) *TrackRepo { return &TrackRepo{pool: pool} }

func (r *TrackRepo) Create(ctx context.Context, t catalog.Track, assets []catalog.AudioAsset) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback after commit is a documented no-op

	const insertTrack = `
		INSERT INTO tracks (id, title, album_id, duration_ms, track_number, isrc, explicit, created_at, updated_at)
		VALUES ($1, $2, NULLIF($3, '')::uuid, $4, $5, NULLIF($6, ''), $7, $8, $8)`
	if _, err := tx.Exec(ctx, insertTrack, t.ID, t.Title, t.AlbumID, t.DurationMs, t.TrackNumber, t.ISRC, t.Explicit, t.CreatedAt); err != nil {
		return err
	}

	const insertCredit = `INSERT INTO track_artists (track_id, artist_id, role) VALUES ($1, $2, $3)` // #nosec G101 -- false positive: parameterized SQL text, no credential ("insertCredit" names an artist-credit row, not a security credential)
	for _, ta := range t.Artists {
		if _, err := tx.Exec(ctx, insertCredit, t.ID, ta.ArtistID, ta.Role); err != nil {
			return err
		}
	}

	const insertAsset = `
		INSERT INTO track_audio_assets (id, track_id, storage_uri, bitrate_kbps, codec, quality_tier, created_at, updated_at)
		VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6, $6)`
	for _, asset := range assets {
		if _, err := tx.Exec(ctx, insertAsset, t.ID, asset.StorageURI, asset.BitrateKbps, asset.Codec, asset.QualityTier, t.CreatedAt); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (r *TrackRepo) GetByID(ctx context.Context, id string) (catalog.Track, error) {
	const q = `
		SELECT id, title, COALESCE(album_id::text, ''), duration_ms, track_number, COALESCE(isrc, ''), explicit, popularity_score, created_at, updated_at
		FROM tracks WHERE id = $1`
	var t catalog.Track
	err := r.pool.QueryRow(ctx, q, id).Scan(&t.ID, &t.Title, &t.AlbumID, &t.DurationMs, &t.TrackNumber, &t.ISRC, &t.Explicit, &t.PopularityScore, &t.CreatedAt, &t.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return catalog.Track{}, catalog.ErrTrackNotFound
	}
	if err != nil {
		return catalog.Track{}, err
	}

	artists, err := r.trackArtists(ctx, id)
	if err != nil {
		return catalog.Track{}, err
	}
	t.Artists = artists
	return t, nil
}

func (r *TrackRepo) trackArtists(ctx context.Context, trackID string) ([]catalog.TrackArtist, error) {
	const q = `
		SELECT ta.artist_id, a.name, ta.role
		FROM track_artists ta JOIN artists a ON a.id = ta.artist_id
		WHERE ta.track_id = $1
		ORDER BY ta.role, a.name`
	rows, err := r.pool.Query(ctx, q, trackID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var artists []catalog.TrackArtist
	for rows.Next() {
		var ta catalog.TrackArtist
		if err := rows.Scan(&ta.ArtistID, &ta.ArtistName, &ta.Role); err != nil {
			return nil, err
		}
		artists = append(artists, ta)
	}
	return artists, rows.Err()
}

func (r *TrackRepo) ListByAlbum(ctx context.Context, albumID string) ([]catalog.Track, error) {
	const q = `
		SELECT id, title, COALESCE(album_id::text, ''), duration_ms, track_number, COALESCE(isrc, ''), explicit, popularity_score, created_at, updated_at
		FROM tracks WHERE album_id = $1 ORDER BY track_number NULLS LAST, id`
	rows, err := r.pool.Query(ctx, q, albumID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tracks []catalog.Track
	for rows.Next() {
		var t catalog.Track
		if err := rows.Scan(&t.ID, &t.Title, &t.AlbumID, &t.DurationMs, &t.TrackNumber, &t.ISRC, &t.Explicit, &t.PopularityScore, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		tracks = append(tracks, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range tracks {
		artists, err := r.trackArtists(ctx, tracks[i].ID)
		if err != nil {
			return nil, err
		}
		tracks[i].Artists = artists
	}
	return tracks, nil
}

func (r *TrackRepo) Search(ctx context.Context, q, cursor string, limit int) (catalog.Page[catalog.Track], error) {
	c, err := decodeNameCursor(cursor)
	if err != nil {
		return catalog.Page[catalog.Track]{}, err
	}

	query := `
		SELECT id, title, COALESCE(album_id::text, ''), duration_ms, track_number, COALESCE(isrc, ''), explicit, popularity_score, created_at, updated_at
		FROM tracks
		WHERE similarity(title, $1) > $2`
	args := []any{q, similarityThreshold}
	if cursor != "" {
		query += ` AND (title, id) > ($3, $4::uuid)`
		args = append(args, c.Name, c.ID)
	}
	query += fmt.Sprintf(" ORDER BY title ASC, id ASC LIMIT $%d", len(args)+1)
	args = append(args, limit+1)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return catalog.Page[catalog.Track]{}, err
	}
	defer rows.Close()

	var items []catalog.Track
	for rows.Next() {
		var t catalog.Track
		if err := rows.Scan(&t.ID, &t.Title, &t.AlbumID, &t.DurationMs, &t.TrackNumber, &t.ISRC, &t.Explicit, &t.PopularityScore, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return catalog.Page[catalog.Track]{}, err
		}
		items = append(items, t)
	}
	if err := rows.Err(); err != nil {
		return catalog.Page[catalog.Track]{}, err
	}

	var next string
	if len(items) > limit {
		last := items[limit-1]
		next = encodeNameCursor(last.Title, last.ID)
		items = items[:limit]
	}
	return catalog.Page[catalog.Track]{Items: items, NextCursor: next}, nil
}

func (r *TrackRepo) ListAudioAssets(ctx context.Context, trackID string) ([]catalog.AudioAsset, error) {
	const q = `
		SELECT id, track_id, storage_uri, bitrate_kbps, codec, quality_tier
		FROM track_audio_assets WHERE track_id = $1 ORDER BY bitrate_kbps DESC`
	rows, err := r.pool.Query(ctx, q, trackID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var assets []catalog.AudioAsset
	for rows.Next() {
		var a catalog.AudioAsset
		if err := rows.Scan(&a.ID, &a.TrackID, &a.StorageURI, &a.BitrateKbps, &a.Codec, &a.QualityTier); err != nil {
			return nil, err
		}
		assets = append(assets, a)
	}
	return assets, rows.Err()
}
