import 'dart:async';

import 'package:social_proximity_domain/social_proximity_domain.dart';

/// Local copy of `social_proximity_domain`'s test-only fake (that package's
/// `test/support/` isn't part of its public `lib/` export surface, so it
/// isn't importable from here - same reasoning `player_ui`'s
/// `FakePlaybackQueueController` documents for itself).
class FakeProximityFeedRepository implements ProximityFeedRepository {
  int connectCalls = 0;
  int disconnectCalls = 0;
  Object? connectError;
  final List<ProximityVisibilityMode> visibilityCalls = [];

  final StreamController<List<NearbyListener>> _listenersController =
      StreamController.broadcast();
  final StreamController<ProximityConnectionState> _connectionController =
      StreamController.broadcast();

  @override
  Future<void> connect() async {
    connectCalls++;
    if (connectError != null) throw connectError!;
  }

  @override
  Future<void> disconnect() async {
    disconnectCalls++;
  }

  @override
  Stream<List<NearbyListener>> watch() => _listenersController.stream;

  @override
  Stream<ProximityConnectionState> get connectionState => _connectionController.stream;

  @override
  void setVisibility(ProximityVisibilityMode mode) => visibilityCalls.add(mode);

  @override
  void updateNowPlaying({String? trackId, int positionMs = 0}) {}

  void emitListeners(List<NearbyListener> listeners) => _listenersController.add(listeners);

  void emitConnectionState(ProximityConnectionState state) => _connectionController.add(state);

  Future<void> dispose() async {
    await _listenersController.close();
    await _connectionController.close();
  }
}
