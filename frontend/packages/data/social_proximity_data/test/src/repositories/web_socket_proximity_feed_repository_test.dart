import 'dart:async';
import 'dart:convert';

import 'package:core_networking/core_networking.dart';
import 'package:core_networking/testing.dart';
import 'package:core_platform/testing.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:social_proximity_data/social_proximity_data.dart';
import 'package:social_proximity_domain/social_proximity_domain.dart';

void main() {
  late FakeSocketTransport transport;
  late ReconnectingWebSocketClient socketClient;
  late FakeLocationProvider locationProvider;
  late WebSocketProximityFeedRepository repository;
  late void Function()? heartbeatCallback;
  late DateTime fakeNow;

  Future<void> tick() => Future<void>.delayed(Duration.zero);

  setUp(() {
    heartbeatCallback = null;
    fakeNow = DateTime(2026, 1, 1);
    locationProvider = FakeLocationProvider();
    socketClient = ReconnectingWebSocketClient(
      uriBuilder: () => Uri.parse('wss://api.smusic.test/v1/presence/connect'),
      transportFactory: (uri) {
        transport = FakeSocketTransport();
        return transport;
      },
      delay: (_) => Future<void>.value(),
    );
    repository = WebSocketProximityFeedRepository(
      socketClient: socketClient,
      locationProvider: locationProvider,
      positionThrottle: const Duration(seconds: 20),
      periodicTimerFactory: (duration, callback) {
        heartbeatCallback = callback;
        return Timer(const Duration(days: 999), () {});
      },
      now: () => fakeNow,
    );
  });

  tearDown(() async {
    await repository.dispose();
  });

  test('connect starts the socket client and subscribes to location at neighborhood accuracy', () async {
    await repository.connect();
    await tick();
    expect(socketClient.isRunning, isTrue);
    expect(locationProvider.watchPositionCalls, [LocationAccuracy.neighborhood]);
  });

  test('connect is idempotent (no duplicate subscriptions)', () async {
    await repository.connect();
    await repository.connect();
    await tick();
    expect(locationProvider.watchPositionCalls, hasLength(1));
  });

  test('disconnect before connect is a no-op', () async {
    await repository.disconnect();
    expect(socketClient.isRunning, isFalse);
  });

  test('disconnect stops the socket client and heartbeat', () async {
    await repository.connect();
    await tick();
    await repository.disconnect();
    expect(socketClient.isRunning, isFalse);
  });

  test('send (e.g. setVisibility) before connect is a no-op, not a throw', () {
    expect(() => repository.setVisibility(ProximityVisibilityMode.everyone), returnsNormally);
  });

  group('nearby_update / resync_full frames', () {
    test('nearby_update frame emits mapped listeners', () async {
      await repository.connect();
      await tick();
      final results = <List<NearbyListener>>[];
      repository.watch().listen(results.add);

      transport.emitMessage(jsonEncode({
        'type': 'nearby_update',
        'users': [
          {'user_id': 'u1', 'distance_bucket': 'under_150m'},
        ],
      }));
      await tick();

      expect(results.single.single.userId, 'u1');
    });

    test('resync_full frame emits mapped listeners (possibly empty)', () async {
      await repository.connect();
      await tick();
      final results = <List<NearbyListener>>[];
      repository.watch().listen(results.add);

      transport.emitMessage(jsonEncode({'type': 'resync_full', 'users': <Map<String, dynamic>>[]}));
      await tick();

      expect(results.single, isEmpty);
    });

    test('a frame whose users field is not a list is ignored', () async {
      await repository.connect();
      await tick();
      final results = <List<NearbyListener>>[];
      repository.watch().listen(results.add);

      transport.emitMessage(jsonEncode({'type': 'nearby_update', 'users': 'not a list'}));
      await tick();

      expect(results, isEmpty);
    });

    test('malformed JSON is ignored rather than thrown', () async {
      await repository.connect();
      await tick();
      transport.emitMessage('not json {{{');
      await tick();
    });

    test('a non-string message is ignored', () async {
      await repository.connect();
      await tick();
      transport.emitMessage(42);
      await tick();
    });

    test('a JSON value that is not an object is ignored', () async {
      await repository.connect();
      await tick();
      transport.emitMessage('[1,2,3]');
      await tick();
    });

    test('an unrecognized frame type is ignored', () async {
      await repository.connect();
      await tick();
      transport.emitMessage(jsonEncode({'type': 'something_else'}));
      await tick();
    });
  });

  test('drain frame triggers a socket stop+restart', () async {
    await repository.connect();
    await tick();
    final firstTransport = transport;

    transport.emitMessage(jsonEncode({'type': 'drain', 'reconnect_hint': 'retry-elsewhere'}));
    await tick();
    await tick();

    expect(firstTransport.closed, isTrue);
    expect(socketClient.isRunning, isTrue);
  });

  test('connection phase changes are mapped and forwarded', () async {
    await repository.connect();
    final states = <ProximityConnectionState>[];
    repository.connectionState.listen(states.add);
    await tick();
    expect(states, contains(ProximityConnectionState.connected));

    await transport.emitDone();
    await tick();
    expect(states, contains(ProximityConnectionState.reconnecting));
  });

  group('position throttling and now_playing', () {
    test('sends an update frame for the first position, throttles the next, resumes after the window', () async {
      await repository.connect();
      await tick();

      locationProvider.emitPosition(
        GeoPosition(latitude: 1, longitude: 2, timestamp: fakeNow, accuracyMeters: 5),
      );
      await tick();
      expect(transport.sentMessages, hasLength(1));
      final firstFrame = jsonDecode(transport.sentMessages.single as String) as Map<String, dynamic>;
      expect(firstFrame['type'], 'update');
      expect(firstFrame['lat'], 1);
      expect(firstFrame['accuracy_m'], 5);
      expect(firstFrame.containsKey('now_playing'), isFalse);

      // Still within the throttle window - dropped.
      locationProvider.emitPosition(
        GeoPosition(latitude: 3, longitude: 4, timestamp: fakeNow, accuracyMeters: 5),
      );
      await tick();
      expect(transport.sentMessages, hasLength(1));

      // Past the throttle window, and now_playing was set in between.
      fakeNow = fakeNow.add(const Duration(seconds: 21));
      repository.updateNowPlaying(trackId: 't1', positionMs: 1500);
      locationProvider.emitPosition(
        GeoPosition(latitude: 3, longitude: 4, timestamp: fakeNow, accuracyMeters: 5),
      );
      await tick();
      expect(transport.sentMessages, hasLength(2));
      final secondFrame = jsonDecode(transport.sentMessages.last as String) as Map<String, dynamic>;
      expect(secondFrame['now_playing'], {'track_id': 't1', 'position_ms': 1500});
    });

    test('updateNowPlaying(trackId: null) clears now_playing from subsequent updates', () async {
      await repository.connect();
      await tick();
      repository.updateNowPlaying(trackId: 't1', positionMs: 1000);
      repository.updateNowPlaying();

      locationProvider.emitPosition(
        GeoPosition(latitude: 1, longitude: 2, timestamp: fakeNow, accuracyMeters: 5),
      );
      await tick();
      final frame = jsonDecode(transport.sentMessages.single as String) as Map<String, dynamic>;
      expect(frame.containsKey('now_playing'), isFalse);
    });
  });

  test('setVisibility sends a visibility frame with the wire mode', () async {
    await repository.connect();
    await tick();
    repository.setVisibility(ProximityVisibilityMode.friendsOnly);
    expect(
      transport.sentMessages,
      [jsonEncode({'type': 'visibility', 'mode': 'friends_only'})],
    );
  });

  test('the heartbeat timer sends a bare heartbeat frame when no position is known yet', () async {
    await repository.connect();
    await tick();
    heartbeatCallback?.call();
    expect(transport.sentMessages, [jsonEncode({'type': 'heartbeat'})]);
  });

  // security.md section 1.2's server-side jitter must be "renovado a cada
  // heartbeat de presença... não é fixo por usuário" - but the backend can
  // only re-jitter when it receives a fresh raw coordinate, and a bare
  // `heartbeat` frame carries none. A stationary device's location stream
  // can legitimately stop emitting new positions (no movement past its
  // distanceFilter), so if the heartbeat timer only ever sent
  // `{'type': 'heartbeat'}`, the same server-side jittered fix would sit
  // unrefreshed for the device's entire stationary session - exactly the
  // "fixed per user" case security.md 1.2 says must not happen. This group
  // asserts the fix: once a position is known, every heartbeat tick
  // resends it as a real "update" frame (forcing a fresh server-side jitter
  // draw), not a position-less "heartbeat".
  group('heartbeat re-sends last known position (security.md 1.2 jitter renewal)', () {
    test('once a position has been received, a heartbeat tick sends it as an update frame, '
        'not a bare heartbeat', () async {
      await repository.connect();
      await tick();

      locationProvider.emitPosition(
        GeoPosition(latitude: 10, longitude: 20, timestamp: fakeNow, accuracyMeters: 8),
      );
      await tick();
      expect(transport.sentMessages, hasLength(1)); // the throttled "update" from movement

      heartbeatCallback?.call();
      await tick();

      expect(transport.sentMessages, hasLength(2));
      final heartbeatFrame = jsonDecode(transport.sentMessages.last as String) as Map<String, dynamic>;
      expect(heartbeatFrame['type'], 'update');
      expect(heartbeatFrame['lat'], 10);
      expect(heartbeatFrame['lon'], 20);
      expect(heartbeatFrame['accuracy_m'], 8);
    });

    test('a heartbeat-driven update frame is NOT gated by positionThrottle '
        '(it must fire every heartbeat tick, independent of the movement throttle)', () async {
      await repository.connect();
      await tick();

      locationProvider.emitPosition(
        GeoPosition(latitude: 10, longitude: 20, timestamp: fakeNow, accuracyMeters: 8),
      );
      await tick();
      expect(transport.sentMessages, hasLength(1));

      // Still well within positionThrottle's window - a movement-triggered
      // update would be dropped here, but a heartbeat tick must still go
      // through.
      heartbeatCallback?.call();
      await tick();
      expect(transport.sentMessages, hasLength(2));

      heartbeatCallback?.call();
      await tick();
      expect(transport.sentMessages, hasLength(3));
    });

    test('a heartbeat-driven update frame carries the pending now_playing, same as a '
        'movement-triggered one', () async {
      await repository.connect();
      await tick();
      repository.updateNowPlaying(trackId: 't1', positionMs: 500);

      locationProvider.emitPosition(
        GeoPosition(latitude: 10, longitude: 20, timestamp: fakeNow, accuracyMeters: 8),
      );
      await tick();

      heartbeatCallback?.call();
      await tick();

      final heartbeatFrame = jsonDecode(transport.sentMessages.last as String) as Map<String, dynamic>;
      expect(heartbeatFrame['now_playing'], {'track_id': 't1', 'position_ms': 500});
    });
  });

  test('uses a default periodicTimerFactory/now when none is provided', () async {
    final defaultRepository = WebSocketProximityFeedRepository(
      socketClient: socketClient,
      locationProvider: locationProvider,
    );
    await defaultRepository.connect();
    await tick();
    await defaultRepository.dispose();
  });

  test('dispose disconnects and closes the exposed streams', () async {
    await repository.connect();
    await tick();
    await repository.dispose();
    expect(repository.watch(), emitsDone);
    expect(repository.connectionState, emitsDone);
  });
}
