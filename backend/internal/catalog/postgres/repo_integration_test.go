//go:build integration

// See .vibeflow/specs/backend-integration-test-coverage.md.
package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"smusic/backend/internal/catalog"
	"smusic/backend/internal/platform/dbx/dbxtest"
	"smusic/backend/internal/platform/idgen"
)

func newID() string { return idgen.UUIDv7{}.NewID() }

func TestIntegration_ArtistRepo(t *testing.T) {
	pool := dbxtest.NewPool(t)
	r := NewArtistRepo(pool)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)

	a := catalog.Artist{ID: newID(), Name: "Integration Artist", CreatedAt: now, UpdatedAt: now}
	require.NoError(t, r.Create(ctx, a))

	got, err := r.GetByID(ctx, a.ID)
	require.NoError(t, err)
	assert.Equal(t, a.Name, got.Name)

	_, err = r.GetByID(ctx, newID())
	assert.ErrorIs(t, err, catalog.ErrArtistNotFound)

	page, err := r.Search(ctx, "Integration", "", 10)
	require.NoError(t, err)
	require.Len(t, page.Items, 1)
	assert.Equal(t, a.ID, page.Items[0].ID)
}

func TestIntegration_AlbumRepo(t *testing.T) {
	pool := dbxtest.NewPool(t)
	artistRepo := NewArtistRepo(pool)
	r := NewAlbumRepo(pool)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)

	artist := catalog.Artist{ID: newID(), Name: "Album Test Artist", CreatedAt: now, UpdatedAt: now}
	require.NoError(t, artistRepo.Create(ctx, artist))

	al := catalog.Album{ID: newID(), Title: "Integration Album", PrimaryArtistID: artist.ID, AlbumType: catalog.AlbumTypeAlbum, CreatedAt: now, UpdatedAt: now}
	require.NoError(t, r.Create(ctx, al))

	got, err := r.GetByID(ctx, al.ID)
	require.NoError(t, err)
	assert.Equal(t, al.Title, got.Title)

	_, err = r.GetByID(ctx, newID())
	assert.ErrorIs(t, err, catalog.ErrAlbumNotFound)

	byArtist, err := r.ListByArtist(ctx, artist.ID)
	require.NoError(t, err)
	require.Len(t, byArtist, 1)
	assert.Equal(t, al.ID, byArtist[0].ID)

	page, err := r.Search(ctx, "Integration", "", 10)
	require.NoError(t, err)
	require.Len(t, page.Items, 1)
}

func TestIntegration_TrackRepo(t *testing.T) {
	pool := dbxtest.NewPool(t)
	artistRepo := NewArtistRepo(pool)
	albumRepo := NewAlbumRepo(pool)
	r := NewTrackRepo(pool)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)

	artist := catalog.Artist{ID: newID(), Name: "Track Test Artist", CreatedAt: now, UpdatedAt: now}
	require.NoError(t, artistRepo.Create(ctx, artist))
	album := catalog.Album{ID: newID(), Title: "Track Test Album", PrimaryArtistID: artist.ID, AlbumType: catalog.AlbumTypeAlbum, CreatedAt: now, UpdatedAt: now}
	require.NoError(t, albumRepo.Create(ctx, album))

	track := catalog.Track{
		ID: newID(), Title: "Integration Track", AlbumID: album.ID, DurationMs: 210000,
		Artists:   []catalog.TrackArtist{{ArtistID: artist.ID, Role: catalog.ArtistRolePrimary}},
		CreatedAt: now, UpdatedAt: now,
	}
	assets := []catalog.AudioAsset{{ID: newID(), TrackID: track.ID, StorageURI: "s3://x", BitrateKbps: 128, Codec: catalog.CodecAAC, QualityTier: catalog.QualityTierNormal}}
	require.NoError(t, r.Create(ctx, track, assets))

	got, err := r.GetByID(ctx, track.ID)
	require.NoError(t, err)
	assert.Equal(t, track.Title, got.Title)
	require.Len(t, got.Artists, 1, "GetByID hydrates track_artists via the unexported trackArtists helper")
	assert.Equal(t, artist.ID, got.Artists[0].ArtistID)

	_, err = r.GetByID(ctx, newID())
	assert.ErrorIs(t, err, catalog.ErrTrackNotFound)

	byAlbum, err := r.ListByAlbum(ctx, album.ID)
	require.NoError(t, err)
	require.Len(t, byAlbum, 1)

	page, err := r.Search(ctx, "Integration Track", "", 10)
	require.NoError(t, err)
	require.Len(t, page.Items, 1)

	gotAssets, err := r.ListAudioAssets(ctx, track.ID)
	require.NoError(t, err)
	require.Len(t, gotAssets, 1)
	assert.Equal(t, "s3://x", gotAssets[0].StorageURI)
}
