import 'dart:async';

import 'package:social_proximity_domain/social_proximity_domain.dart';

class FakeProximityFeedRepository implements ProximityFeedRepository {
  int connectCalls = 0;
  int disconnectCalls = 0;
  final List<ProximityVisibilityMode> visibilityCalls = [];
  final List<({String? trackId, int positionMs})> nowPlayingCalls = [];

  final StreamController<List<NearbyListener>> _listenersController =
      StreamController.broadcast();
  final StreamController<ProximityConnectionState> _connectionController =
      StreamController.broadcast();

  @override
  Future<void> connect() async {
    connectCalls++;
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
  void updateNowPlaying({String? trackId, int positionMs = 0}) =>
      nowPlayingCalls.add((trackId: trackId, positionMs: positionMs));

  void emitListeners(List<NearbyListener> listeners) => _listenersController.add(listeners);

  void emitConnectionState(ProximityConnectionState state) =>
      _connectionController.add(state);

  Future<void> dispose() async {
    await _listenersController.close();
    await _connectionController.close();
  }
}
