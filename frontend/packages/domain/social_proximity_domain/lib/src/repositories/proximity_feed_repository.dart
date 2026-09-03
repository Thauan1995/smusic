import '../entities/nearby_listener.dart';
import '../entities/proximity_connection_state.dart';
import '../entities/proximity_visibility_mode.dart';

/// frontend-flutter.md section 4.1: "Abstração de domínio:
/// `ProximityFeedRepository` (interface em `social_proximity_domain`)
/// expõe `Stream<List<NearbyListener>>` - a feature nunca toca em
/// `WebSocketChannel` diretamente."
///
/// Implemented by `social_proximity_data`'s `WebSocketProximityFeedRepository`,
/// which owns the `ReconnectingWebSocketClient` (core_networking) *and* the
/// `LocationProvider` (core_platform) internally - this interface does not
/// take a location stream as an argument, because sending throttled
/// position updates to the backend is an implementation detail of "being
/// connected to presence", not something `social_proximity_domain`'s
/// notifiers need to orchestrate directly (frontend-flutter.md section
/// 4.1's throttle-before-send behavior lives entirely in that one class).
abstract interface class ProximityFeedRepository {
  /// [connect] must be called before [watch]/[connectionState] emit
  /// anything meaningful; safe to call again while already connected
  /// (no-op).
  Future<void> connect();

  Future<void> disconnect();

  /// The current nearby-listener set - backend-go.md section 4's
  /// `nearby_update`/`resync_full` frames both carry a full `users[]` array
  /// (see `WebSocketProximityFeedRepository`'s doc comment for the
  /// ASSUMPTION this implies about the wire protocol), so each emission on
  /// this stream *replaces* the previous list rather than delta-patching
  /// it.
  Stream<List<NearbyListener>> watch();

  Stream<ProximityConnectionState> get connectionState;

  /// backend-go.md section 4's `{type: "visibility", mode: ...}` frame.
  void setVisibility(ProximityVisibilityMode mode);

  /// backend-go.md section 4's `update` frame's optional `now_playing`
  /// field - `{track_id, position_ms}` on the wire (the *client's own*
  /// outbound shape, distinct from what an incoming `NearbyListener` carries
  /// for other users - see `social_proximity_data`'s `ProximityDtos` doc
  /// comment for that asymmetry). Pass `trackId: null` to clear (nothing
  /// currently playing); [positionMs] is only meaningful when [trackId] is
  /// non-null. Wiring this to `player_domain`'s actual now-playing stream is
  /// left to the app composition root - see `social_proximity_domain`'s
  /// task report for why that cross-feature wiring is out of this slice's
  /// scope.
  void updateNowPlaying({String? trackId, int positionMs = 0});
}
