package library

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
	playlists      *fakePlaylistRepo
	playlistTracks *fakePlaylistTrackRepo
	libraryTracks  *fakeLibraryTrackRepo
	tracks         *fakeTrackChecker
	clock          *clock.Frozen
}

func newTestService(t *testing.T, existingTracks ...string) (*Service, *testDeps) {
	t.Helper()
	d := &testDeps{
		playlists:      newFakePlaylistRepo(),
		playlistTracks: newFakePlaylistTrackRepo(),
		libraryTracks:  newFakeLibraryTrackRepo(),
		tracks:         newFakeTrackChecker(existingTracks...),
		clock:          clock.NewFrozen(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)),
	}
	svc := NewService(d.playlists, d.playlistTracks, d.libraryTracks, d.tracks, d.clock, idgen.NewSequential("id"))
	return svc, d
}

// --- CreatePlaylist ---

func TestCreatePlaylist_Success(t *testing.T) {
	svc, _ := newTestService(t)
	p, err := svc.CreatePlaylist(context.Background(), CreatePlaylistInput{OwnerID: "u1", Title: "  My Mix  "})
	require.NoError(t, err)
	assert.Equal(t, "My Mix", p.Title)
	assert.Equal(t, VisibilityPrivate, p.Visibility, "defaults to private")
}

func TestCreatePlaylist_MissingOwner(t *testing.T) {
	svc, _ := newTestService(t)
	_, err := svc.CreatePlaylist(context.Background(), CreatePlaylistInput{Title: "X"})
	assert.ErrorIs(t, err, ErrInvalidInput)
}

func TestCreatePlaylist_MissingTitle(t *testing.T) {
	svc, _ := newTestService(t)
	_, err := svc.CreatePlaylist(context.Background(), CreatePlaylistInput{OwnerID: "u1", Title: " "})
	assert.ErrorIs(t, err, ErrInvalidInput)
}

func TestCreatePlaylist_InvalidVisibility(t *testing.T) {
	svc, _ := newTestService(t)
	_, err := svc.CreatePlaylist(context.Background(), CreatePlaylistInput{OwnerID: "u1", Title: "X", Visibility: "bogus"})
	assert.ErrorIs(t, err, ErrInvalidInput)
}

func TestCreatePlaylist_RepoError(t *testing.T) {
	svc, d := newTestService(t)
	d.playlists.createErr = errBoom
	_, err := svc.CreatePlaylist(context.Background(), CreatePlaylistInput{OwnerID: "u1", Title: "X"})
	require.Error(t, err)
}

// --- ListPlaylists ---

func TestListPlaylists_Success(t *testing.T) {
	svc, _ := newTestService(t)
	_, err := svc.CreatePlaylist(context.Background(), CreatePlaylistInput{OwnerID: "u1", Title: "A"})
	require.NoError(t, err)
	_, err = svc.CreatePlaylist(context.Background(), CreatePlaylistInput{OwnerID: "u2", Title: "B"})
	require.NoError(t, err)

	got, err := svc.ListPlaylists(context.Background(), "u1")
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "A", got[0].Title)
}

func TestListPlaylists_MissingOwner(t *testing.T) {
	svc, _ := newTestService(t)
	_, err := svc.ListPlaylists(context.Background(), "")
	assert.ErrorIs(t, err, ErrInvalidInput)
}

func TestListPlaylists_RepoError(t *testing.T) {
	svc, d := newTestService(t)
	d.playlists.listErr = errBoom
	_, err := svc.ListPlaylists(context.Background(), "u1")
	require.Error(t, err)
}

// --- ListPlaylistTracks ---

func TestListPlaylistTracks_OwnerSuccess(t *testing.T) {
	svc, _ := newTestService(t, "t1")
	p, err := svc.CreatePlaylist(context.Background(), CreatePlaylistInput{OwnerID: "u1", Title: "A"})
	require.NoError(t, err)
	_, err = svc.AddTrack(context.Background(), p.ID, "u1", "t1", "")
	require.NoError(t, err)

	got, tracks, err := svc.ListPlaylistTracks(context.Background(), p.ID, "u1")
	require.NoError(t, err)
	assert.Equal(t, p.ID, got.ID)
	require.Len(t, tracks, 1)
}

func TestListPlaylistTracks_MissingID(t *testing.T) {
	svc, _ := newTestService(t)
	_, _, err := svc.ListPlaylistTracks(context.Background(), "", "u1")
	assert.ErrorIs(t, err, ErrInvalidInput)
}

func TestListPlaylistTracks_NotFound(t *testing.T) {
	svc, _ := newTestService(t)
	_, _, err := svc.ListPlaylistTracks(context.Background(), "nope", "u1")
	assert.ErrorIs(t, err, ErrPlaylistNotFound)
}

func TestListPlaylistTracks_OtherGetError(t *testing.T) {
	svc, d := newTestService(t)
	d.playlists.getErr = errBoom
	_, _, err := svc.ListPlaylistTracks(context.Background(), "x", "u1")
	require.Error(t, err)
}

func TestListPlaylistTracks_PrivateDeniedForNonOwner(t *testing.T) {
	svc, _ := newTestService(t)
	p, err := svc.CreatePlaylist(context.Background(), CreatePlaylistInput{OwnerID: "u1", Title: "A"})
	require.NoError(t, err)

	_, _, err = svc.ListPlaylistTracks(context.Background(), p.ID, "u2")
	assert.ErrorIs(t, err, ErrNotOwner)
}

func TestListPlaylistTracks_PublicVisibleToNonOwner(t *testing.T) {
	svc, _ := newTestService(t)
	p, err := svc.CreatePlaylist(context.Background(), CreatePlaylistInput{OwnerID: "u1", Title: "A", Visibility: VisibilityPublic})
	require.NoError(t, err)

	_, _, err = svc.ListPlaylistTracks(context.Background(), p.ID, "u2")
	assert.NoError(t, err)
}

func TestListPlaylistTracks_ListError(t *testing.T) {
	svc, d := newTestService(t)
	p, err := svc.CreatePlaylist(context.Background(), CreatePlaylistInput{OwnerID: "u1", Title: "A"})
	require.NoError(t, err)
	d.playlistTracks.listErr = errBoom

	_, _, err = svc.ListPlaylistTracks(context.Background(), p.ID, "u1")
	require.Error(t, err)
}

// --- AddTrack ---

func TestAddTrack_AppendsToEnd(t *testing.T) {
	svc, _ := newTestService(t, "t1", "t2")
	p, err := svc.CreatePlaylist(context.Background(), CreatePlaylistInput{OwnerID: "u1", Title: "A"})
	require.NoError(t, err)

	pt1, err := svc.AddTrack(context.Background(), p.ID, "u1", "t1", "")
	require.NoError(t, err)
	pt2, err := svc.AddTrack(context.Background(), p.ID, "u1", "t2", PositionEnd)
	require.NoError(t, err)

	assert.Greater(t, pt2.Position, pt1.Position)
}

func TestAddTrack_PrependsToStart(t *testing.T) {
	svc, _ := newTestService(t, "t1", "t2")
	p, err := svc.CreatePlaylist(context.Background(), CreatePlaylistInput{OwnerID: "u1", Title: "A"})
	require.NoError(t, err)

	pt1, err := svc.AddTrack(context.Background(), p.ID, "u1", "t1", "")
	require.NoError(t, err)
	pt2, err := svc.AddTrack(context.Background(), p.ID, "u1", "t2", PositionStart)
	require.NoError(t, err)

	assert.Less(t, pt2.Position, pt1.Position)
}

func TestAddTrack_PrependsToEmptyPlaylist(t *testing.T) {
	svc, _ := newTestService(t, "t1")
	p, err := svc.CreatePlaylist(context.Background(), CreatePlaylistInput{OwnerID: "u1", Title: "A"})
	require.NoError(t, err)

	pt, err := svc.AddTrack(context.Background(), p.ID, "u1", "t1", PositionStart)
	require.NoError(t, err)
	assert.Equal(t, float64(positionGap), pt.Position)
}

func TestAddTrack_PrependsWithMultipleExistingTracks(t *testing.T) {
	svc, _ := newTestService(t, "t1", "t2", "t3")
	p, err := svc.CreatePlaylist(context.Background(), CreatePlaylistInput{OwnerID: "u1", Title: "A"})
	require.NoError(t, err)

	_, err = svc.AddTrack(context.Background(), p.ID, "u1", "t1", "")
	require.NoError(t, err)
	_, err = svc.AddTrack(context.Background(), p.ID, "u1", "t2", "")
	require.NoError(t, err)

	pt3, err := svc.AddTrack(context.Background(), p.ID, "u1", "t3", PositionStart)
	require.NoError(t, err)

	_, tracks, err := svc.ListPlaylistTracks(context.Background(), p.ID, "u1")
	require.NoError(t, err)
	require.Len(t, tracks, 3)
	assert.Equal(t, "t3", tracks[0].TrackID, "must be positioned before both existing tracks")
	assert.Less(t, pt3.Position, tracks[1].Position)
}

func TestAddTrack_EmptyPlaylistID(t *testing.T) {
	svc, _ := newTestService(t, "t1")
	_, err := svc.AddTrack(context.Background(), "", "u1", "t1", "")
	assert.ErrorIs(t, err, ErrInvalidInput)
}

func TestAddTrack_MissingTrackID(t *testing.T) {
	svc, _ := newTestService(t)
	p, err := svc.CreatePlaylist(context.Background(), CreatePlaylistInput{OwnerID: "u1", Title: "A"})
	require.NoError(t, err)

	_, err = svc.AddTrack(context.Background(), p.ID, "u1", "", "")
	assert.ErrorIs(t, err, ErrInvalidInput)
}

func TestAddTrack_PlaylistNotFound(t *testing.T) {
	svc, _ := newTestService(t)
	_, err := svc.AddTrack(context.Background(), "nope", "u1", "t1", "")
	assert.ErrorIs(t, err, ErrPlaylistNotFound)
}

func TestAddTrack_NotOwner(t *testing.T) {
	svc, _ := newTestService(t, "t1")
	p, err := svc.CreatePlaylist(context.Background(), CreatePlaylistInput{OwnerID: "u1", Title: "A"})
	require.NoError(t, err)

	_, err = svc.AddTrack(context.Background(), p.ID, "u2", "t1", "")
	assert.ErrorIs(t, err, ErrNotOwner)
}

func TestAddTrack_TrackDoesNotExist(t *testing.T) {
	svc, _ := newTestService(t)
	p, err := svc.CreatePlaylist(context.Background(), CreatePlaylistInput{OwnerID: "u1", Title: "A"})
	require.NoError(t, err)

	_, err = svc.AddTrack(context.Background(), p.ID, "u1", "nope", "")
	assert.ErrorIs(t, err, ErrTrackNotFound)
}

func TestAddTrack_TrackCheckerError(t *testing.T) {
	svc, d := newTestService(t)
	p, err := svc.CreatePlaylist(context.Background(), CreatePlaylistInput{OwnerID: "u1", Title: "A"})
	require.NoError(t, err)
	d.tracks.err = errBoom

	_, err = svc.AddTrack(context.Background(), p.ID, "u1", "t1", "")
	require.Error(t, err)
}

func TestAddTrack_ComputePositionListError(t *testing.T) {
	svc, d := newTestService(t, "t1")
	p, err := svc.CreatePlaylist(context.Background(), CreatePlaylistInput{OwnerID: "u1", Title: "A"})
	require.NoError(t, err)
	d.playlistTracks.listErr = errBoom

	_, err = svc.AddTrack(context.Background(), p.ID, "u1", "t1", PositionStart)
	require.Error(t, err)
}

func TestAddTrack_MaxPositionError(t *testing.T) {
	svc, d := newTestService(t, "t1")
	p, err := svc.CreatePlaylist(context.Background(), CreatePlaylistInput{OwnerID: "u1", Title: "A"})
	require.NoError(t, err)
	d.playlistTracks.maxPosErr = errBoom

	_, err = svc.AddTrack(context.Background(), p.ID, "u1", "t1", "")
	require.Error(t, err)
}

func TestAddTrack_RepoAddError(t *testing.T) {
	svc, d := newTestService(t, "t1")
	p, err := svc.CreatePlaylist(context.Background(), CreatePlaylistInput{OwnerID: "u1", Title: "A"})
	require.NoError(t, err)
	d.playlistTracks.addErr = errBoom

	_, err = svc.AddTrack(context.Background(), p.ID, "u1", "t1", "")
	require.Error(t, err)
}

// --- RemoveTrack ---

func TestRemoveTrack_Success(t *testing.T) {
	svc, _ := newTestService(t, "t1")
	p, err := svc.CreatePlaylist(context.Background(), CreatePlaylistInput{OwnerID: "u1", Title: "A"})
	require.NoError(t, err)
	_, err = svc.AddTrack(context.Background(), p.ID, "u1", "t1", "")
	require.NoError(t, err)

	require.NoError(t, svc.RemoveTrack(context.Background(), p.ID, "u1", "t1"))

	_, tracks, err := svc.ListPlaylistTracks(context.Background(), p.ID, "u1")
	require.NoError(t, err)
	assert.Empty(t, tracks)
}

func TestRemoveTrack_MissingTrackID(t *testing.T) {
	svc, _ := newTestService(t)
	p, err := svc.CreatePlaylist(context.Background(), CreatePlaylistInput{OwnerID: "u1", Title: "A"})
	require.NoError(t, err)
	err = svc.RemoveTrack(context.Background(), p.ID, "u1", "")
	assert.ErrorIs(t, err, ErrInvalidInput)
}

func TestRemoveTrack_NotOwner(t *testing.T) {
	svc, _ := newTestService(t)
	p, err := svc.CreatePlaylist(context.Background(), CreatePlaylistInput{OwnerID: "u1", Title: "A"})
	require.NoError(t, err)
	err = svc.RemoveTrack(context.Background(), p.ID, "u2", "t1")
	assert.ErrorIs(t, err, ErrNotOwner)
}

func TestRemoveTrack_NotInPlaylist(t *testing.T) {
	svc, _ := newTestService(t)
	p, err := svc.CreatePlaylist(context.Background(), CreatePlaylistInput{OwnerID: "u1", Title: "A"})
	require.NoError(t, err)
	err = svc.RemoveTrack(context.Background(), p.ID, "u1", "nope")
	assert.ErrorIs(t, err, ErrTrackNotInPlaylist)
}

func TestRemoveTrack_RepoOtherError(t *testing.T) {
	svc, d := newTestService(t)
	p, err := svc.CreatePlaylist(context.Background(), CreatePlaylistInput{OwnerID: "u1", Title: "A"})
	require.NoError(t, err)
	d.playlistTracks.removeErr = errBoom

	err = svc.RemoveTrack(context.Background(), p.ID, "u1", "t1")
	require.Error(t, err)
	assert.False(t, errors.Is(err, ErrTrackNotInPlaylist))
}

// --- SaveTrack / UnsaveTrack / ListSavedTracks ---

func TestSaveTrack_Success(t *testing.T) {
	svc, _ := newTestService(t, "t1")
	require.NoError(t, svc.SaveTrack(context.Background(), "u1", "t1"))

	items, _, err := svc.ListSavedTracks(context.Background(), "u1", "", 0)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "t1", items[0].TrackID)
}

func TestSaveTrack_MissingArgs(t *testing.T) {
	svc, _ := newTestService(t)
	assert.ErrorIs(t, svc.SaveTrack(context.Background(), "", "t1"), ErrInvalidInput)
	assert.ErrorIs(t, svc.SaveTrack(context.Background(), "u1", ""), ErrInvalidInput)
}

func TestSaveTrack_NotFound(t *testing.T) {
	svc, _ := newTestService(t)
	err := svc.SaveTrack(context.Background(), "u1", "nope")
	assert.ErrorIs(t, err, ErrTrackNotFound)
}

func TestSaveTrack_CheckerError(t *testing.T) {
	svc, d := newTestService(t)
	d.tracks.err = errBoom
	err := svc.SaveTrack(context.Background(), "u1", "t1")
	require.Error(t, err)
}

func TestSaveTrack_RepoError(t *testing.T) {
	svc, d := newTestService(t, "t1")
	d.libraryTracks.addErr = errBoom
	err := svc.SaveTrack(context.Background(), "u1", "t1")
	require.Error(t, err)
}

func TestUnsaveTrack_Success(t *testing.T) {
	svc, _ := newTestService(t, "t1")
	require.NoError(t, svc.SaveTrack(context.Background(), "u1", "t1"))
	require.NoError(t, svc.UnsaveTrack(context.Background(), "u1", "t1"))

	items, _, err := svc.ListSavedTracks(context.Background(), "u1", "", 0)
	require.NoError(t, err)
	assert.Empty(t, items)
}

func TestUnsaveTrack_MissingArgs(t *testing.T) {
	svc, _ := newTestService(t)
	assert.ErrorIs(t, svc.UnsaveTrack(context.Background(), "", "t1"), ErrInvalidInput)
}

func TestUnsaveTrack_RepoError(t *testing.T) {
	svc, d := newTestService(t)
	d.libraryTracks.removeErr = errBoom
	err := svc.UnsaveTrack(context.Background(), "u1", "t1")
	require.Error(t, err)
}

func TestListSavedTracks_MissingUser(t *testing.T) {
	svc, _ := newTestService(t)
	_, _, err := svc.ListSavedTracks(context.Background(), "", "", 0)
	assert.ErrorIs(t, err, ErrInvalidInput)
}

func TestListSavedTracks_LimitClamping(t *testing.T) {
	svc, _ := newTestService(t)
	_, _, err := svc.ListSavedTracks(context.Background(), "u1", "", -1)
	require.NoError(t, err)
	_, _, err = svc.ListSavedTracks(context.Background(), "u1", "", 100_000)
	require.NoError(t, err)
}

func TestListSavedTracks_RepoError(t *testing.T) {
	svc, d := newTestService(t)
	d.libraryTracks.listErr = errBoom
	_, _, err := svc.ListSavedTracks(context.Background(), "u1", "", 0)
	require.Error(t, err)
}

func TestListSavedTracks_Pagination(t *testing.T) {
	svc, _ := newTestService(t, "t1", "t2", "t3")
	for _, id := range []string{"t1", "t2", "t3"} {
		require.NoError(t, svc.SaveTrack(context.Background(), "u1", id))
	}

	page1, cursor, err := svc.ListSavedTracks(context.Background(), "u1", "", 2)
	require.NoError(t, err)
	assert.Len(t, page1, 2)
	assert.NotEmpty(t, cursor)

	page2, _, err := svc.ListSavedTracks(context.Background(), "u1", cursor, 2)
	require.NoError(t, err)
	assert.Len(t, page2, 1)
}
