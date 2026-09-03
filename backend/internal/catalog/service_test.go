package catalog

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"smusic/backend/internal/platform/clock"
	"smusic/backend/internal/platform/idgen"
)

var errBoom = errors.New("boom")

type testDeps struct {
	artists *fakeArtistRepo
	albums  *fakeAlbumRepo
	tracks  *fakeTrackRepo
	clock   *clock.Frozen
}

func newTestService(t *testing.T) (*Service, *testDeps) {
	t.Helper()
	d := &testDeps{
		artists: newFakeArtistRepo(),
		albums:  newFakeAlbumRepo(),
		tracks:  newFakeTrackRepo(),
		clock:   clock.NewFrozen(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)),
	}
	svc := NewService(d.artists, d.albums, d.tracks, d.clock, idgen.NewSequential("id"))
	return svc, d
}

// --- Artists ---

func TestCreateArtist_Success(t *testing.T) {
	svc, _ := newTestService(t)
	a, err := svc.CreateArtist(context.Background(), CreateArtistInput{Name: "  Daft Punk  ", Slug: "daft-punk"})
	require.NoError(t, err)
	assert.Equal(t, "Daft Punk", a.Name)
	assert.NotEmpty(t, a.ID)
}

func TestCreateArtist_MissingName(t *testing.T) {
	svc, _ := newTestService(t)
	_, err := svc.CreateArtist(context.Background(), CreateArtistInput{Name: "  "})
	assert.ErrorIs(t, err, ErrInvalidInput)
}

func TestCreateArtist_RepoError(t *testing.T) {
	svc, d := newTestService(t)
	d.artists.createErr = errBoom
	_, err := svc.CreateArtist(context.Background(), CreateArtistInput{Name: "X"})
	require.Error(t, err)
}

func TestGetArtist_Success(t *testing.T) {
	svc, _ := newTestService(t)
	created, err := svc.CreateArtist(context.Background(), CreateArtistInput{Name: "X"})
	require.NoError(t, err)

	got, err := svc.GetArtist(context.Background(), created.ID)
	require.NoError(t, err)
	assert.Equal(t, created, got)
}

func TestGetArtist_MissingID(t *testing.T) {
	svc, _ := newTestService(t)
	_, err := svc.GetArtist(context.Background(), "")
	assert.ErrorIs(t, err, ErrInvalidInput)
}

func TestGetArtist_NotFound(t *testing.T) {
	svc, _ := newTestService(t)
	_, err := svc.GetArtist(context.Background(), "nope")
	assert.ErrorIs(t, err, ErrArtistNotFound)
}

func TestGetArtist_OtherError(t *testing.T) {
	svc, d := newTestService(t)
	d.artists.getErr = errBoom
	_, err := svc.GetArtist(context.Background(), "x")
	require.Error(t, err)
	assert.False(t, errors.Is(err, ErrArtistNotFound))
}

// --- Albums ---

func TestCreateAlbum_Success(t *testing.T) {
	svc, _ := newTestService(t)
	artist, err := svc.CreateArtist(context.Background(), CreateArtistInput{Name: "X"})
	require.NoError(t, err)

	album, err := svc.CreateAlbum(context.Background(), CreateAlbumInput{Title: "Album", PrimaryArtistID: artist.ID})
	require.NoError(t, err)
	assert.Equal(t, AlbumTypeAlbum, album.AlbumType, "defaults to 'album'")
}

func TestCreateAlbum_MissingTitle(t *testing.T) {
	svc, _ := newTestService(t)
	_, err := svc.CreateAlbum(context.Background(), CreateAlbumInput{Title: " "})
	assert.ErrorIs(t, err, ErrInvalidInput)
}

func TestCreateAlbum_InvalidType(t *testing.T) {
	svc, _ := newTestService(t)
	_, err := svc.CreateAlbum(context.Background(), CreateAlbumInput{Title: "T", AlbumType: "bogus"})
	assert.ErrorIs(t, err, ErrInvalidInput)
}

func TestCreateAlbum_NonexistentArtist(t *testing.T) {
	svc, _ := newTestService(t)
	_, err := svc.CreateAlbum(context.Background(), CreateAlbumInput{Title: "T", PrimaryArtistID: "nope"})
	assert.ErrorIs(t, err, ErrInvalidInput)
}

func TestCreateAlbum_ArtistLookupOtherError(t *testing.T) {
	svc, d := newTestService(t)
	d.artists.getErr = errBoom
	_, err := svc.CreateAlbum(context.Background(), CreateAlbumInput{Title: "T", PrimaryArtistID: "x"})
	require.Error(t, err)
	assert.False(t, errors.Is(err, ErrInvalidInput))
}

func TestCreateAlbum_RepoError(t *testing.T) {
	svc, d := newTestService(t)
	d.albums.createErr = errBoom
	_, err := svc.CreateAlbum(context.Background(), CreateAlbumInput{Title: "T"})
	require.Error(t, err)
}

func TestGetAlbum_Success(t *testing.T) {
	svc, _ := newTestService(t)
	artist, err := svc.CreateArtist(context.Background(), CreateArtistInput{Name: "X"})
	require.NoError(t, err)
	album, err := svc.CreateAlbum(context.Background(), CreateAlbumInput{Title: "T", PrimaryArtistID: artist.ID})
	require.NoError(t, err)
	_, err = svc.CreateTrack(context.Background(), CreateTrackInput{
		Title: "Track 1", AlbumID: album.ID, DurationMs: 1000,
		Artists: []TrackArtist{{ArtistID: artist.ID, Role: ArtistRolePrimary}},
	})
	require.NoError(t, err)

	got, tracks, err := svc.GetAlbum(context.Background(), album.ID)
	require.NoError(t, err)
	assert.Equal(t, album.ID, got.ID)
	require.Len(t, tracks, 1)
	assert.Equal(t, "Track 1", tracks[0].Title)
}

func TestGetAlbum_MissingID(t *testing.T) {
	svc, _ := newTestService(t)
	_, _, err := svc.GetAlbum(context.Background(), "")
	assert.ErrorIs(t, err, ErrInvalidInput)
}

func TestGetAlbum_NotFound(t *testing.T) {
	svc, _ := newTestService(t)
	_, _, err := svc.GetAlbum(context.Background(), "nope")
	assert.ErrorIs(t, err, ErrAlbumNotFound)
}

func TestGetAlbum_OtherError(t *testing.T) {
	svc, d := newTestService(t)
	d.albums.getErr = errBoom
	_, _, err := svc.GetAlbum(context.Background(), "x")
	require.Error(t, err)
}

func TestGetAlbum_ListTracksError(t *testing.T) {
	svc, d := newTestService(t)
	album, err := svc.CreateAlbum(context.Background(), CreateAlbumInput{Title: "T"})
	require.NoError(t, err)
	d.tracks.listErr = errBoom

	_, _, err = svc.GetAlbum(context.Background(), album.ID)
	require.Error(t, err)
}

// --- Tracks ---

func validTrackInput() CreateTrackInput {
	return CreateTrackInput{
		Title: "Song", DurationMs: 200_000,
		Artists: []TrackArtist{{ArtistID: "artist-1", Role: ArtistRolePrimary}},
	}
}

func TestCreateTrack_Success(t *testing.T) {
	svc, _ := newTestService(t)
	tr, err := svc.CreateTrack(context.Background(), validTrackInput())
	require.NoError(t, err)
	assert.Equal(t, "Song", tr.Title)
}

func TestCreateTrack_MissingTitle(t *testing.T) {
	svc, _ := newTestService(t)
	in := validTrackInput()
	in.Title = " "
	_, err := svc.CreateTrack(context.Background(), in)
	assert.ErrorIs(t, err, ErrInvalidInput)
}

func TestCreateTrack_InvalidDuration(t *testing.T) {
	svc, _ := newTestService(t)
	in := validTrackInput()
	in.DurationMs = 0
	_, err := svc.CreateTrack(context.Background(), in)
	assert.ErrorIs(t, err, ErrInvalidInput)
}

func TestCreateTrack_NoArtists(t *testing.T) {
	svc, _ := newTestService(t)
	in := validTrackInput()
	in.Artists = nil
	_, err := svc.CreateTrack(context.Background(), in)
	assert.ErrorIs(t, err, ErrInvalidInput)
}

func TestCreateTrack_InvalidArtistCredit(t *testing.T) {
	svc, _ := newTestService(t)

	in := validTrackInput()
	in.Artists = []TrackArtist{{ArtistID: "", Role: ArtistRolePrimary}}
	_, err := svc.CreateTrack(context.Background(), in)
	assert.ErrorIs(t, err, ErrInvalidInput)

	in2 := validTrackInput()
	in2.Artists = []TrackArtist{{ArtistID: "a1", Role: "bogus"}}
	_, err = svc.CreateTrack(context.Background(), in2)
	assert.ErrorIs(t, err, ErrInvalidInput)
}

func TestCreateTrack_NonexistentAlbum(t *testing.T) {
	svc, _ := newTestService(t)
	in := validTrackInput()
	in.AlbumID = "nope"
	_, err := svc.CreateTrack(context.Background(), in)
	assert.ErrorIs(t, err, ErrInvalidInput)
}

func TestCreateTrack_AlbumLookupOtherError(t *testing.T) {
	svc, d := newTestService(t)
	d.albums.getErr = errBoom
	in := validTrackInput()
	in.AlbumID = "x"
	_, err := svc.CreateTrack(context.Background(), in)
	require.Error(t, err)
	assert.False(t, errors.Is(err, ErrInvalidInput))
}

func TestCreateTrack_RepoError(t *testing.T) {
	svc, d := newTestService(t)
	d.tracks.createErr = errBoom
	_, err := svc.CreateTrack(context.Background(), validTrackInput())
	require.Error(t, err)
}

func TestGetTrack_Success(t *testing.T) {
	svc, _ := newTestService(t)
	tr, err := svc.CreateTrack(context.Background(), validTrackInput())
	require.NoError(t, err)

	got, assets, err := svc.GetTrack(context.Background(), tr.ID)
	require.NoError(t, err)
	assert.Equal(t, tr.ID, got.ID)
	assert.Empty(t, assets)
}

func TestGetTrack_MissingID(t *testing.T) {
	svc, _ := newTestService(t)
	_, _, err := svc.GetTrack(context.Background(), "")
	assert.ErrorIs(t, err, ErrInvalidInput)
}

func TestGetTrack_NotFound(t *testing.T) {
	svc, _ := newTestService(t)
	_, _, err := svc.GetTrack(context.Background(), "nope")
	assert.ErrorIs(t, err, ErrTrackNotFound)
}

func TestGetTrack_OtherError(t *testing.T) {
	svc, d := newTestService(t)
	d.tracks.getErr = errBoom
	_, _, err := svc.GetTrack(context.Background(), "x")
	require.Error(t, err)
}

func TestGetTrack_AssetsError(t *testing.T) {
	svc, d := newTestService(t)
	tr, err := svc.CreateTrack(context.Background(), validTrackInput())
	require.NoError(t, err)
	d.tracks.assetsErr = errBoom

	_, _, err = svc.GetTrack(context.Background(), tr.ID)
	require.Error(t, err)
}

func TestTrackExists(t *testing.T) {
	svc, _ := newTestService(t)
	tr, err := svc.CreateTrack(context.Background(), validTrackInput())
	require.NoError(t, err)

	ok, err := svc.TrackExists(context.Background(), tr.ID)
	require.NoError(t, err)
	assert.True(t, ok)

	ok, err = svc.TrackExists(context.Background(), "nope")
	require.NoError(t, err)
	assert.False(t, ok)

	ok, err = svc.TrackExists(context.Background(), "")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestTrackExists_OtherError(t *testing.T) {
	svc, d := newTestService(t)
	d.tracks.getErr = errBoom
	_, err := svc.TrackExists(context.Background(), "x")
	require.Error(t, err)
}

// --- Search ---

func TestSearch_MissingQuery(t *testing.T) {
	svc, _ := newTestService(t)
	_, err := svc.Search(context.Background(), SearchInput{Query: " "})
	assert.ErrorIs(t, err, ErrInvalidInput)
}

func TestSearch_DefaultsToTrackType(t *testing.T) {
	svc, _ := newTestService(t)
	_, err := svc.CreateTrack(context.Background(), validTrackInput())
	require.NoError(t, err)

	out, err := svc.Search(context.Background(), SearchInput{Query: "song"})
	require.NoError(t, err)
	assert.Len(t, out.Tracks, 1)
}

func TestSearch_Album(t *testing.T) {
	svc, _ := newTestService(t)
	_, err := svc.CreateAlbum(context.Background(), CreateAlbumInput{Title: "Discovery"})
	require.NoError(t, err)

	out, err := svc.Search(context.Background(), SearchInput{Query: "disco", Type: SearchTypeAlbum})
	require.NoError(t, err)
	require.Len(t, out.Albums, 1)
	assert.Equal(t, "Discovery", out.Albums[0].Title)
}

func TestSearch_Artist(t *testing.T) {
	svc, _ := newTestService(t)
	_, err := svc.CreateArtist(context.Background(), CreateArtistInput{Name: "Daft Punk"})
	require.NoError(t, err)

	out, err := svc.Search(context.Background(), SearchInput{Query: "daft", Type: SearchTypeArtist})
	require.NoError(t, err)
	require.Len(t, out.Artists, 1)
}

func TestSearch_UnsupportedType(t *testing.T) {
	svc, _ := newTestService(t)
	_, err := svc.Search(context.Background(), SearchInput{Query: "x", Type: "playlist"})
	assert.ErrorIs(t, err, ErrInvalidInput)
}

func TestSearch_LimitClamping(t *testing.T) {
	svc, d := newTestService(t)
	for i := 0; i < 3; i++ {
		_, err := svc.CreateTrack(context.Background(), validTrackInput())
		require.NoError(t, err)
	}

	_, err := svc.Search(context.Background(), SearchInput{Query: "song", Limit: -1})
	require.NoError(t, err)
	_, err = svc.Search(context.Background(), SearchInput{Query: "song", Limit: 10_000})
	require.NoError(t, err)
	_ = d
}

func TestSearch_TrackRepoError(t *testing.T) {
	svc, d := newTestService(t)
	d.tracks.searchErr = errBoom
	_, err := svc.Search(context.Background(), SearchInput{Query: "x", Type: SearchTypeTrack})
	require.Error(t, err)
}

func TestSearch_AlbumRepoError(t *testing.T) {
	svc, d := newTestService(t)
	d.albums.searchErr = errBoom
	_, err := svc.Search(context.Background(), SearchInput{Query: "x", Type: SearchTypeAlbum})
	require.Error(t, err)
}

func TestSearch_ArtistRepoError(t *testing.T) {
	svc, d := newTestService(t)
	d.artists.searchErr = errBoom
	_, err := svc.Search(context.Background(), SearchInput{Query: "x", Type: SearchTypeArtist})
	require.Error(t, err)
}

func TestSearch_Pagination(t *testing.T) {
	svc, _ := newTestService(t)
	for i := 0; i < 5; i++ {
		_, err := svc.CreateTrack(context.Background(), validTrackInput())
		require.NoError(t, err)
	}

	out, err := svc.Search(context.Background(), SearchInput{Query: "song", Limit: 2})
	require.NoError(t, err)
	assert.Len(t, out.Tracks, 2)
	assert.NotEmpty(t, out.NextCursor)

	out2, err := svc.Search(context.Background(), SearchInput{Query: "song", Limit: 2, Cursor: out.NextCursor})
	require.NoError(t, err)
	assert.Len(t, out2.Tracks, 2)
}
