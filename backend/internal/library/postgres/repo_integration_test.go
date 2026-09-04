//go:build integration

// See .vibeflow/specs/backend-integration-test-coverage.md.
package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	authpg "smusic/backend/internal/auth/postgres"
	catalogpg "smusic/backend/internal/catalog/postgres"
	"smusic/backend/internal/library"
	"smusic/backend/internal/platform/dbx/dbxtest"
	"smusic/backend/internal/platform/idgen"

	"smusic/backend/internal/auth"
	"smusic/backend/internal/catalog"
)

func newID() string { return idgen.UUIDv7{}.NewID() }

func TestIntegration_PlaylistRepo(t *testing.T) {
	pool := dbxtest.NewPool(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)

	userID := newID()
	require.NoError(t, authpg.New(pool).Create(ctx, auth.User{
		ID: userID, Email: "playlist-owner@example.com", DisplayName: "Owner",
		Status: auth.UserStatusActive, CreatedAt: now, UpdatedAt: now,
	}))

	r := NewPlaylistRepo(pool)
	p := library.Playlist{ID: newID(), OwnerID: userID, Title: "Integration Playlist", Visibility: library.VisibilityPrivate, CreatedAt: now, UpdatedAt: now}
	require.NoError(t, r.Create(ctx, p))

	got, err := r.GetByID(ctx, p.ID)
	require.NoError(t, err)
	assert.Equal(t, p.Title, got.Title)

	_, err = r.GetByID(ctx, newID())
	assert.ErrorIs(t, err, library.ErrPlaylistNotFound)

	byOwner, err := r.ListByOwner(ctx, userID)
	require.NoError(t, err)
	require.Len(t, byOwner, 1)
	assert.Equal(t, p.ID, byOwner[0].ID)
}

func TestIntegration_PlaylistTrackRepo(t *testing.T) {
	pool := dbxtest.NewPool(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)

	userID := newID()
	require.NoError(t, authpg.New(pool).Create(ctx, auth.User{
		ID: userID, Email: "pt-owner@example.com", DisplayName: "Owner",
		Status: auth.UserStatusActive, CreatedAt: now, UpdatedAt: now,
	}))
	artistRepo := catalogpg.NewArtistRepo(pool)
	artist := catalog.Artist{ID: newID(), Name: "PT Artist", CreatedAt: now, UpdatedAt: now}
	require.NoError(t, artistRepo.Create(ctx, artist))
	trackRepo := catalogpg.NewTrackRepo(pool)
	track := catalog.Track{
		ID: newID(), Title: "PT Track", DurationMs: 180000,
		Artists: []catalog.TrackArtist{{ArtistID: artist.ID, Role: catalog.ArtistRolePrimary}},
		CreatedAt: now, UpdatedAt: now,
	}
	require.NoError(t, trackRepo.Create(ctx, track, nil))

	playlistRepo := NewPlaylistRepo(pool)
	playlist := library.Playlist{ID: newID(), OwnerID: userID, Title: "PT Playlist", Visibility: library.VisibilityPrivate, CreatedAt: now, UpdatedAt: now}
	require.NoError(t, playlistRepo.Create(ctx, playlist))

	r := NewPlaylistTrackRepo(pool)

	// MaxPosition on an empty playlist: ok=false, per the interface's
	// documented "no tracks yet" contract.
	_, ok, err := r.MaxPosition(ctx, playlist.ID)
	require.NoError(t, err)
	assert.False(t, ok)

	require.NoError(t, r.Add(ctx, library.PlaylistTrack{
		ID: newID(), PlaylistID: playlist.ID, TrackID: track.ID, Position: 1024, AddedBy: userID, AddedAt: now,
	}))

	max, ok, err := r.MaxPosition(ctx, playlist.ID)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, float64(1024), max)

	list, err := r.List(ctx, playlist.ID)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, track.ID, list[0].TrackID)

	require.NoError(t, r.Remove(ctx, playlist.ID, track.ID))
	list, err = r.List(ctx, playlist.ID)
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestIntegration_LibraryTrackRepo(t *testing.T) {
	pool := dbxtest.NewPool(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)

	userID := newID()
	require.NoError(t, authpg.New(pool).Create(ctx, auth.User{
		ID: userID, Email: "lt-owner@example.com", DisplayName: "Owner",
		Status: auth.UserStatusActive, CreatedAt: now, UpdatedAt: now,
	}))
	artistRepo := catalogpg.NewArtistRepo(pool)
	artist := catalog.Artist{ID: newID(), Name: "LT Artist", CreatedAt: now, UpdatedAt: now}
	require.NoError(t, artistRepo.Create(ctx, artist))
	trackRepo := catalogpg.NewTrackRepo(pool)
	track := catalog.Track{
		ID: newID(), Title: "LT Track", DurationMs: 150000,
		Artists: []catalog.TrackArtist{{ArtistID: artist.ID, Role: catalog.ArtistRolePrimary}},
		CreatedAt: now, UpdatedAt: now,
	}
	require.NoError(t, trackRepo.Create(ctx, track, nil))

	r := NewLibraryTrackRepo(pool)
	require.NoError(t, r.Add(ctx, library.LibraryTrack{UserID: userID, TrackID: track.ID, AddedAt: now}))

	list, _, err := r.List(ctx, userID, "", 10)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, track.ID, list[0].TrackID)

	require.NoError(t, r.Remove(ctx, userID, track.ID))
	list, _, err = r.List(ctx, userID, "", 10)
	require.NoError(t, err)
	assert.Empty(t, list)
}
