package ws

import (
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"smusic/backend/internal/presence"
)

func TestBucketCode(t *testing.T) {
	assert.Equal(t, "under_150m", bucketCode(presence.Bucket1))
	assert.Equal(t, "150m_1km", bucketCode(presence.Bucket2))
	assert.Equal(t, "1km_5km", bucketCode(presence.Bucket3))
	assert.Equal(t, "5km_15km", bucketCode(presence.Bucket4))
	assert.Equal(t, "", bucketCode(presence.BucketNone))
}

func TestToOutboundFrame(t *testing.T) {
	f := presence.Frame{
		Type: presence.FrameNearbyUpdate,
		Users: []presence.NearbyResult{
			{UserID: "u1", DisplayName: "Alice", AvatarURL: "http://x/a.png", Bucket: presence.Bucket2, TrackID: "t1"},
			{UserID: "u2", Bucket: presence.Bucket1},
		},
	}
	out := toOutboundFrame(f)
	assert.Equal(t, presence.FrameNearbyUpdate, out.Type)
	require.Len(t, out.Users, 2)
}

func TestToOutboundFrame_NoTrackID_OmitsNowPlaying(t *testing.T) {
	f := presence.Frame{Users: []presence.NearbyResult{{UserID: "u1", Bucket: presence.Bucket1}}}
	out := toOutboundFrame(f)
	assert.Nil(t, out.Users[0].NowPlaying)
}

func TestToOutboundFrame_TrackID_SetsNowPlaying(t *testing.T) {
	f := presence.Frame{Users: []presence.NearbyResult{{UserID: "u1", Bucket: presence.Bucket1, TrackID: "t1"}}}
	out := toOutboundFrame(f)
	require.NotNil(t, out.Users[0].NowPlaying)
	assert.Equal(t, "t1", out.Users[0].NowPlaying.TrackID)
}

func TestToOutboundFrame_DrainCarriesReconnectHint(t *testing.T) {
	f := presence.Frame{Type: presence.FrameDrain, ReconnectHint: "try another replica"}
	out := toOutboundFrame(f)
	assert.Equal(t, "try another replica", out.ReconnectHint)
	assert.Empty(t, out.Users)
}

// TestUserFrame_StructurallyCannotCarryCoordinates mirrors
// internal/presence's NearbyResult structural test at the wire-protocol
// layer: the JSON shape actually sent to a client must never carry a
// coordinate/geohash/exact-distance field (security.md §1.2).
func TestUserFrame_StructurallyCannotCarryCoordinates(t *testing.T) {
	typ := reflect.TypeOf(userFrame{})
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		assert.NotEqualf(t, reflect.Float64, f.Type.Kind(), "field %s must not be a float64", f.Name)
		lower := strings.ToLower(f.Name)
		assert.NotContains(t, lower, "lat")
		assert.NotContains(t, lower, "lon")
		assert.NotContains(t, lower, "geohash")
	}
}
