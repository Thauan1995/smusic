// Package ws implements the client-facing WebSocket transport for
// backend-go.md §4's presence protocol
// (WS /v1/presence/connect). It is intentionally a thin adapter: every
// privacy decision is made by internal/presence's NearbyService/Hub; this
// package only (de)serializes frames and drives the connection lifecycle
// (backend-go.md §7's "handlers finos" principle applied to a WS handler
// instead of an HTTP one).
package ws

import "smusic/backend/internal/presence"

// Client -> server frame types (backend-go.md §4).
const (
	TypeUpdate     = "update"
	TypeHeartbeat  = "heartbeat"
	TypeVisibility = "visibility"
)

// inboundFrame is the wire shape of every client -> server message; fields
// are optional depending on Type.
type inboundFrame struct {
	Type       string        `json:"type"`
	Lat        *float64      `json:"lat,omitempty"`
	Lon        *float64      `json:"lon,omitempty"`
	AccuracyM  *float64      `json:"accuracy_m,omitempty"`
	NowPlaying *nowPlayingIn `json:"now_playing,omitempty"`
	Mode       string        `json:"mode,omitempty"` // for type=visibility
}

type nowPlayingIn struct {
	TrackID    string `json:"track_id"`
	PositionMs int    `json:"position_ms,omitempty"`
}

// outboundFrame is the wire shape of every server -> client message.
type outboundFrame struct {
	Type          string      `json:"type"`
	Users         []userFrame `json:"users,omitempty"`
	ReconnectHint string      `json:"reconnect_hint,omitempty"`
}

// userFrame is one entry of a nearby_update/resync_full frame's "users"
// list. security.md §1.2: DistanceBucket/DistanceLabel are the ONLY
// positional fields — there is no lat/lon/geohash/exact-distance field in
// this type, by construction, so it's structurally impossible for the WS
// transport layer to leak one even if NearbyResult were ever misused
// upstream.
type userFrame struct {
	UserID         string         `json:"user_id"`
	DisplayName    string         `json:"display_name,omitempty"`
	AvatarURL      string         `json:"avatar_url,omitempty"`
	DistanceBucket string         `json:"distance_bucket"`
	DistanceLabel  string         `json:"distance_label"`
	NowPlaying     *nowPlayingOut `json:"now_playing,omitempty"`
}

type nowPlayingOut struct {
	TrackID string `json:"track_id"`
}

// bucketCode returns the stable machine-readable code for b, per
// security.md §1.2's table (never an exact distance).
func bucketCode(b presence.DistanceBucket) string {
	switch b {
	case presence.Bucket1:
		return "under_150m"
	case presence.Bucket2:
		return "150m_1km"
	case presence.Bucket3:
		return "1km_5km"
	case presence.Bucket4:
		return "5km_15km"
	default:
		return ""
	}
}

func toOutboundFrame(f presence.Frame) outboundFrame {
	out := outboundFrame{Type: f.Type, ReconnectHint: f.ReconnectHint}
	for _, u := range f.Users {
		uf := userFrame{
			UserID:         u.UserID,
			DisplayName:    u.DisplayName,
			AvatarURL:      u.AvatarURL,
			DistanceBucket: bucketCode(u.Bucket),
			DistanceLabel:  u.Bucket.Label(),
		}
		if u.TrackID != "" {
			uf.NowPlaying = &nowPlayingOut{TrackID: u.TrackID}
		}
		out.Users = append(out.Users, uf)
	}
	return out
}
