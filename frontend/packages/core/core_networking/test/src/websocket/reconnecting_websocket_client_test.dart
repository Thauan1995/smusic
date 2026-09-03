import 'dart:async';
import 'dart:math';

import 'package:core_networking/core_networking.dart';
import 'package:core_networking/testing.dart';
import 'package:test/test.dart';

class _FixedRandom implements Random {
  @override
  bool nextBool() => false;

  @override
  double nextDouble() => 0.5; // jitter factor exactly 1.0 (no drift)

  @override
  int nextInt(int max) => 0;
}

void main() {
  group('computeReconnectBackoff', () {
    test('doubles per attempt up to the cap when jitter factor is 1.0', () {
      final random = _FixedRandom();
      const initial = Duration(seconds: 1);
      const max = Duration(seconds: 30);
      expect(
        computeReconnectBackoff(0, initial: initial, max: max, random: random),
        const Duration(seconds: 1),
      );
      expect(
        computeReconnectBackoff(1, initial: initial, max: max, random: random),
        const Duration(seconds: 2),
      );
      expect(
        computeReconnectBackoff(2, initial: initial, max: max, random: random),
        const Duration(seconds: 4),
      );
      expect(
        computeReconnectBackoff(3, initial: initial, max: max, random: random),
        const Duration(seconds: 8),
      );
      expect(
        computeReconnectBackoff(4, initial: initial, max: max, random: random),
        const Duration(seconds: 16),
      );
      // 2^5 * 1s = 32s, capped to the 30s max.
      expect(
        computeReconnectBackoff(5, initial: initial, max: max, random: random),
        const Duration(seconds: 30),
      );
      expect(
        computeReconnectBackoff(10, initial: initial, max: max, random: random),
        const Duration(seconds: 30),
      );
    });

    test('applies jitter within +/-20% of the capped base', () {
      final random = Random(42);
      const initial = Duration(seconds: 1);
      const max = Duration(seconds: 30);
      for (var attempt = 0; attempt < 6; attempt++) {
        final delay = computeReconnectBackoff(
          attempt,
          initial: initial,
          max: max,
          random: random,
        );
        final base = min(1000 * pow(2, attempt).toDouble(), 30000);
        expect(delay.inMilliseconds, greaterThanOrEqualTo((base * 0.8).floor()));
        expect(delay.inMilliseconds, lessThanOrEqualTo((base * 1.2).ceil()));
      }
    });
  });

  group('ReconnectingWebSocketClient', () {
    late List<FakeSocketTransport> transports;
    late List<Duration> delays;
    late ReconnectingWebSocketClient client;
    late bool nextConnectThrows;
    late List<Completer<void>> delayGates;

    // A single `await Future.delayed(Duration.zero)` flushes the *entire*
    // microtask queue - with an instantly-resolving `delay`, the whole
    // reconnect loop (possibly several iterations) would drain in one such
    // tick, making it impossible to assert intermediate states ("has it
    // reconnected yet?"). `delayGates` makes each backoff wait explicit and
    // test-controlled: `delay` records the requested duration and returns a
    // `Completer` the test releases one at a time via [releaseNextDelay].
    Future<void> tick() => Future<void>.delayed(Duration.zero);

    Future<void> releaseNextDelay() async {
      delayGates.removeAt(0).complete();
      await tick();
    }

    setUp(() {
      transports = [];
      delays = [];
      delayGates = [];
      nextConnectThrows = false;
      client = ReconnectingWebSocketClient(
        uriBuilder: () => Uri.parse('wss://example.test/v1/presence/connect'),
        transportFactory: (uri) {
          if (nextConnectThrows) {
            nextConnectThrows = false;
            throw StateError('connect failed');
          }
          final transport = FakeSocketTransport();
          transports.add(transport);
          return transport;
        },
        random: _FixedRandom(),
        delay: (d) {
          delays.add(d);
          final gate = Completer<void>();
          delayGates.add(gate);
          return gate.future;
        },
      );
    });

    tearDown(() async {
      await client.dispose();
    });

    test('start connects and forwards messages/phases', () async {
      final phases = <SocketConnectionPhase>[];
      final messages = <dynamic>[];
      client.connectionPhase.listen(phases.add);
      client.messages.listen(messages.add);

      client.start();
      await tick();

      expect(transports, hasLength(1));
      expect(phases, [
        SocketConnectionPhase.connecting,
        SocketConnectionPhase.connected,
      ]);

      transports.first.emitMessage('{"type":"nearby_update"}');
      await tick();
      expect(messages, ['{"type":"nearby_update"}']);
    });

    test('send forwards to the current transport', () async {
      client.start();
      await tick();
      client.send('{"type":"heartbeat"}');
      expect(transports.single.sentMessages, ['{"type":"heartbeat"}']);
    });

    test('send is a no-op before any connection exists', () {
      expect(() => client.send('x'), returnsNormally);
    });

    test(
      'reconnects at initialBackoff each time a healthy session drops',
      () async {
        client.start();
        await tick();
        expect(transports, hasLength(1));

        await transports[0].emitDone();
        await tick();
        expect(delays, [const Duration(seconds: 1)]);
        expect(transports, hasLength(1)); // reconnect gated, not attempted yet

        await releaseNextDelay();
        expect(transports, hasLength(2));

        await transports[1].emitDone();
        await tick();
        // A second healthy-then-dropped session also resets to
        // initialBackoff - growth only happens on consecutive raw connect
        // failures, not on repeated successful-but-short sessions.
        expect(delays, [const Duration(seconds: 1), const Duration(seconds: 1)]);

        await releaseNextDelay();
        expect(transports, hasLength(3));
      },
    );

    test('grows backoff across repeated connect failures, resets after success', () async {
      final phases = <SocketConnectionPhase>[];
      client.connectionPhase.listen(phases.add);

      nextConnectThrows = true;
      client.start();
      await tick();
      expect(transports, isEmpty);
      expect(delays, [const Duration(seconds: 1)]);

      nextConnectThrows = true;
      await releaseNextDelay();
      expect(delays, [const Duration(seconds: 1), const Duration(seconds: 2)]);
      expect(transports, isEmpty);

      // Third attempt succeeds.
      await releaseNextDelay();
      expect(transports, hasLength(1));
      expect(phases, contains(SocketConnectionPhase.connected));

      // A subsequent drop of this healthy session resets to initialBackoff.
      await transports.first.emitDone();
      await tick();
      expect(delays.last, const Duration(seconds: 1));
    });

    test('an error on the transport stream also triggers reconnect', () async {
      client.start();
      await tick();
      transports.first.emitError('boom');
      await tick();
      expect(transports, hasLength(1)); // reconnect gated, not attempted yet
      await releaseNextDelay();
      expect(transports, hasLength(2));
    });

    test('stop halts reconnection and emits disconnected', () async {
      final phases = <SocketConnectionPhase>[];
      client.connectionPhase.listen(phases.add);
      client.start();
      await tick();

      await client.stop();
      await tick(); // let the broadcast phase stream deliver `disconnected`
      expect(phases.last, SocketConnectionPhase.disconnected);
      // `stop` itself closes the transport (unlike a remote-initiated
      // drop/error) - a further tick with no gate release confirms the
      // connect loop really exited rather than merely being paused.
      expect(transports.first.closed, isTrue);
      await tick();
      expect(transports, hasLength(1)); // no reconnect attempted
      expect(client.isRunning, isFalse);
    });

    test('stop interrupts a pending backoff delay immediately', () async {
      // A delay implementation that never completes on its own - if `stop`
      // did not race it via `_stopSignal`, this test would hang forever
      // (the test framework's own timeout would eventually fail it).
      final blockingTransports = <FakeSocketTransport>[];
      final blockingClient = ReconnectingWebSocketClient(
        uriBuilder: () => Uri.parse('wss://example.test/connect'),
        transportFactory: (uri) {
          final t = FakeSocketTransport();
          blockingTransports.add(t);
          return t;
        },
        random: _FixedRandom(),
        delay: (_) => Completer<void>().future,
      );
      blockingClient.start();
      await Future<void>.delayed(Duration.zero);
      expect(blockingTransports, hasLength(1));

      // Drop the healthy session - the client now schedules a reconnect and
      // is suspended inside `await Future.any([_delay(...), _stopSignal])`
      // with a `_delay` that never resolves on its own.
      await blockingTransports.first.emitDone();
      await Future<void>.delayed(Duration.zero);

      await blockingClient.stop();
      expect(blockingClient.isRunning, isFalse);
      await blockingClient.dispose();
    });

    test(
      'a generation change landing between building the URI and connecting '
      'closes the now-stale transport instead of adopting it',
      () async {
        late ReconnectingWebSocketClient raceClient;
        var uriCalls = 0;
        final raceTransports = <FakeSocketTransport>[];
        raceClient = ReconnectingWebSocketClient(
          uriBuilder: () {
            uriCalls++;
            if (uriCalls == 1) {
              // Simulates `stop()` racing an in-flight connect attempt
              // between `uriBuilder()` resolving and `transportFactory`
              // handing back a transport - both happen synchronously with
              // no `await` in between in production code, so the only way
              // to land in that window from a test is a side effect inside
              // `uriBuilder` itself.
              unawaited(raceClient.stop());
            }
            return Uri.parse('wss://example.test/connect');
          },
          transportFactory: (uri) {
            final transport = FakeSocketTransport();
            raceTransports.add(transport);
            return transport;
          },
          random: _FixedRandom(),
          delay: (d) => Future<void>.value(),
        );

        raceClient.start();
        await tick();

        expect(raceTransports, hasLength(1));
        expect(raceTransports.first.closed, isTrue);
        expect(raceClient.isRunning, isFalse);
        await raceClient.dispose();
      },
    );

    test('uses a default Random/delay/transport factory when none is provided', () async {
      final defaultClient = ReconnectingWebSocketClient(
        uriBuilder: () => Uri.parse('wss://example.test/connect'),
      );
      expect(defaultClient.isRunning, isFalse);
      await defaultClient.dispose();
    });

    test('calling start twice is a no-op while already running', () async {
      client.start();
      await Future<void>.delayed(Duration.zero);
      client.start();
      await Future<void>.delayed(Duration.zero);
      expect(transports, hasLength(1));
    });

    test('isRunning reflects start/stop', () async {
      expect(client.isRunning, isFalse);
      client.start();
      expect(client.isRunning, isTrue);
      await client.stop();
      expect(client.isRunning, isFalse);
    });

    test('dispose closes messages and connectionPhase streams', () async {
      client.start();
      await Future<void>.delayed(Duration.zero);
      await client.dispose();
      expect(client.messages, emitsDone);
      expect(client.connectionPhase, emitsDone);
    });
  });
}
