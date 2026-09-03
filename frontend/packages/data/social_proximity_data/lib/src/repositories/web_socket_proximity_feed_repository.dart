import 'dart:async';
import 'dart:convert';

import 'package:core_networking/core_networking.dart';
import 'package:core_platform/core_platform.dart';
import 'package:social_proximity_domain/social_proximity_domain.dart';

import '../dto/proximity_dtos.dart';

/// `social_proximity_domain.ProximityFeedRepository` implementation:
/// backend-go.md section 4's `/v1/presence/connect` WS contract, driven by
/// a `ReconnectingWebSocketClient` (core_networking, reconnect/backoff -
/// frontend-flutter.md section 4.3) and `core_platform`'s `LocationProvider`
/// (own-position updates - section 4.1).
///
/// [socketClient] is expected to already be constructed with a `uriBuilder`
/// that resolves `/v1/presence/connect` plus a fresh auth token per
/// (re)connect attempt (composed at the app root, out of this class's
/// concern - `ReconnectingWebSocketClient`'s `uriBuilder` is a plain
/// `Uri Function()`, called fresh on every attempt for exactly this reason).
///
/// **Throttling own-position updates** (frontend-flutter.md section 4.1:
/// "a cada 15-30s de movimento significativo") happens in two layers: the
/// `LocationProvider`'s own `distanceFilter` (movement-based, configured in
/// `GeolocatorLocationProvider`) and a time-based floor here
/// ([positionThrottle], default 20s) - belt and suspenders, since a
/// movement-triggered stream could still fire faster than the backend
/// wants under GPS jitter.
///
/// **now_playing wiring**: [updateNowPlaying] only stores the
/// `trackId`/`positionMs` to attach to the *next* throttled position update
/// - actually wiring this to `player_domain`'s real now-playing stream is
/// left to the app composition root (cross-feature dependency, out of this
/// slice's scope - see the task report).
///
/// **Heartbeats re-send the last known position as an "update" frame, not a
/// bare `heartbeat`** (security review finding, security.md section 1.2):
/// see [_sendHeartbeat]'s doc comment for why a stationary device (whose
/// [LocationProvider.watchPosition] stream may legitimately stop emitting)
/// would otherwise leave the same server-side jittered fix unrefreshed for
/// an entire session, defeating the "renewed every heartbeat" guarantee.
class WebSocketProximityFeedRepository implements ProximityFeedRepository {
  WebSocketProximityFeedRepository({
    required ReconnectingWebSocketClient socketClient,
    required LocationProvider locationProvider,
    this.heartbeatInterval = const Duration(seconds: 30),
    this.positionThrottle = const Duration(seconds: 20),
    this.locationAccuracy = LocationAccuracy.neighborhood,
    Timer Function(Duration duration, void Function() callback)? periodicTimerFactory,
    DateTime Function()? now,
  })  : _socketClient = socketClient,
        _locationProvider = locationProvider,
        _periodicTimerFactory = periodicTimerFactory ??
            ((duration, callback) => Timer.periodic(duration, (_) => callback())),
        _now = now ?? DateTime.now;

  final ReconnectingWebSocketClient _socketClient;
  final LocationProvider _locationProvider;

  /// backend-go.md section 3: "heartbeat recomendado a cada 30-45s".
  final Duration heartbeatInterval;

  final Duration positionThrottle;
  final LocationAccuracy locationAccuracy;
  final Timer Function(Duration duration, void Function() callback) _periodicTimerFactory;
  final DateTime Function() _now;

  final StreamController<List<NearbyListener>> _listenersController =
      StreamController.broadcast();
  final StreamController<ProximityConnectionState> _connectionController =
      StreamController.broadcast();

  StreamSubscription<dynamic>? _messagesSubscription;
  StreamSubscription<SocketConnectionPhase>? _phaseSubscription;
  StreamSubscription<GeoPosition>? _positionSubscription;
  Timer? _heartbeatTimer;
  DateTime? _lastPositionSentAt;

  /// The most recent position [LocationProvider.watchPosition] has emitted,
  /// regardless of whether [positionThrottle] let it through as an "update"
  /// frame. Kept so [_sendHeartbeat] (below) can re-send it on every
  /// heartbeat tick instead of a bare, position-less `heartbeat` frame - see
  /// that method's doc comment for why this matters for security.md section
  /// 1.2's jitter guarantee.
  GeoPosition? _lastKnownPosition;
  String? _pendingNowPlayingTrackId;
  int _pendingNowPlayingPositionMs = 0;
  bool _connected = false;

  @override
  Future<void> connect() async {
    if (_connected) return;
    _connected = true;

    _messagesSubscription = _socketClient.messages.listen(_handleMessage);
    _phaseSubscription = _socketClient.connectionPhase.listen(_handlePhase);
    _positionSubscription =
        _locationProvider.watchPosition(accuracy: locationAccuracy).listen(_handlePosition);
    _heartbeatTimer = _periodicTimerFactory(heartbeatInterval, _sendHeartbeat);

    _socketClient.start();
  }

  @override
  Future<void> disconnect() async {
    if (!_connected) return;
    _connected = false;
    await _messagesSubscription?.cancel();
    await _phaseSubscription?.cancel();
    await _positionSubscription?.cancel();
    _messagesSubscription = null;
    _phaseSubscription = null;
    _positionSubscription = null;
    _heartbeatTimer?.cancel();
    _heartbeatTimer = null;
    _lastPositionSentAt = null;
    _lastKnownPosition = null;
    await _socketClient.stop();
  }

  @override
  Stream<List<NearbyListener>> watch() => _listenersController.stream;

  @override
  Stream<ProximityConnectionState> get connectionState => _connectionController.stream;

  @override
  void setVisibility(ProximityVisibilityMode mode) {
    _send({'type': 'visibility', 'mode': ProximityDtos.visibilityModeToWire(mode)});
  }

  @override
  void updateNowPlaying({String? trackId, int positionMs = 0}) {
    _pendingNowPlayingTrackId = trackId;
    _pendingNowPlayingPositionMs = positionMs;
  }

  void _handleMessage(dynamic raw) {
    if (raw is! String) return;
    final Object? decoded;
    try {
      decoded = jsonDecode(raw);
    } on FormatException {
      return;
    }
    if (decoded is! Map<String, dynamic>) return;

    switch (decoded['type']) {
      case 'nearby_update':
      case 'resync_full':
        // Both frame types carry a full `users[]` snapshot per
        // backend-go.md section 4's documented shape - see
        // `ProximityFeedRepository.watch`'s doc comment for the ASSUMPTION
        // this implies (full snapshot per emission, not incremental
        // deltas).
        final users = decoded['users'];
        if (users is List && !_listenersController.isClosed) {
          _listenersController.add(ProximityDtos.listenersFromUsersJson(users));
        }
      case 'drain':
        // backend-go.md section 3: server-initiated graceful drain -
        // reconnect (a fresh attempt typically lands on a different
        // replica).
        unawaited(_socketClient.stop().then((_) => _socketClient.start()));
    }
  }

  void _handlePhase(SocketConnectionPhase phase) {
    if (_connectionController.isClosed) return;
    _connectionController.add(_mapPhase(phase));
  }

  static ProximityConnectionState _mapPhase(SocketConnectionPhase phase) {
    switch (phase) {
      case SocketConnectionPhase.connected:
        return ProximityConnectionState.connected;
      case SocketConnectionPhase.reconnecting:
        return ProximityConnectionState.reconnecting;
      case SocketConnectionPhase.connecting:
      case SocketConnectionPhase.disconnected:
        return ProximityConnectionState.offline;
    }
  }

  void _handlePosition(GeoPosition position) {
    // Cache the latest raw fix unconditionally - even a throttled-out
    // position (see below) is still the freshest ground truth
    // [_sendHeartbeat] should re-jitter around, per security.md section
    // 1.2.
    _lastKnownPosition = position;

    final now = _now();
    final last = _lastPositionSentAt;
    if (last != null && now.difference(last) < positionThrottle) return;
    _lastPositionSentAt = now;

    _sendUpdateFrame(position);
  }

  void _sendUpdateFrame(GeoPosition position) {
    final nowPlayingTrackId = _pendingNowPlayingTrackId;
    _send({
      'type': 'update',
      'lat': position.latitude,
      'lon': position.longitude,
      'accuracy_m': position.accuracyMeters,
      if (nowPlayingTrackId != null)
        'now_playing': ProximityDtos.updateNowPlayingToJson(
          trackId: nowPlayingTrackId,
          positionMs: _pendingNowPlayingPositionMs,
        ),
    });
  }

  /// security.md section 1.2: the server-side spatial jitter (backend
  /// `Jitterer`, applied in `NearbyService.ApplyUpdate`) must be "renovado a
  /// cada heartbeat de presença... não é fixo por usuário" - specifically so
  /// a stationary target's exposed position can't be calibrated down by
  /// repeated observation over the course of one long session. The backend
  /// can only re-jitter when it receives a fresh raw coordinate (a bare
  /// `heartbeat` frame carries none - backend-go.md section 4 - and the
  /// backend never stores the pre-jitter coordinate to re-jitter around
  /// later, by design, security.md section 1.5). A stationary device's
  /// [LocationProvider.watchPosition] stream can legitimately stop emitting
  /// (no movement past its distanceFilter), which - before this fix - meant
  /// no "update" frame at all for the rest of a long stationary session, so
  /// the *same* server-side jittered fix would sit unrefreshed for that
  /// entire time: exactly the "fixed per user, calibratable by repeated
  /// observation" case security.md 1.2 calls out. Resending the last known
  /// raw fix as a real "update" frame on every heartbeat tick (rather than a
  /// position-less "heartbeat" frame) forces a fresh, independent
  /// server-side jitter draw on this same cadence regardless of whether the
  /// device has physically moved - closing that gap without needing a wire
  /// protocol change (an "update" frame with an unchanged lat/lon is
  /// already valid per the existing contract). Falls back to a bare
  /// "heartbeat" only when no position has ever been received yet (e.g.
  /// permission just granted, no GPS fix landed) - there is nothing to
  /// (re-)jitter around in that case.
  void _sendHeartbeat() {
    final position = _lastKnownPosition;
    if (position == null) {
      _send({'type': 'heartbeat'});
      return;
    }
    _sendUpdateFrame(position);
  }

  void _send(Map<String, dynamic> frame) => _socketClient.send(jsonEncode(frame));

  Future<void> dispose() async {
    await disconnect();
    await _listenersController.close();
    await _connectionController.close();
  }
}
