import 'dart:async';
import 'dart:math';

import 'package:meta/meta.dart';

import 'socket_transport.dart';

/// Generic connection-lifecycle phase - deliberately not the same type as
/// `social_proximity_domain`'s `ProximityConnectionState`: this class lives
/// in `core_networking` and must not depend on any feature package (per
/// frontend-flutter.md section 1.2, "core sem dependência de features").
/// `social_proximity_data`'s repository maps this to the domain-facing enum.
enum SocketConnectionPhase { connecting, connected, reconnecting, disconnected }

/// Pure backoff-delay calculation, extracted so it is unit-testable without
/// timers: base delay doubles per attempt (0 -> [initial], 1 -> 2x, 2 -> 4x,
/// ...), capped at [max], with ±20% multiplicative jitter applied on top
/// (frontend-flutter.md section 4.3: "backoff exponencial (ex.: 1s, 2s, 4s,
/// 8s, cap em 30s) e jitter" - the doc does not specify a jitter algorithm,
/// this is this client's own implementation decision, not a backend
/// contract question). Jitter is renewed on every call (never derivable by
/// an observer from a fixed per-connection seed), matching the same
/// "don't let an attacker calibrate a fixed offset" spirit as security.md
/// section 1.2's spatial jitter (different mechanism, same principle).
@visibleForTesting
Duration computeReconnectBackoff(
  int attempt, {
  required Duration initial,
  required Duration max,
  required Random random,
}) {
  final rawMs = initial.inMilliseconds * pow(2, attempt);
  final cappedMs = min(rawMs.toDouble(), max.inMilliseconds.toDouble());
  final jitterFactor = 0.8 + random.nextDouble() * 0.4; // [0.8, 1.2)
  final jitteredMs = (cappedMs * jitterFactor).round();
  return Duration(milliseconds: jitteredMs.clamp(0, max.inMilliseconds));
}

/// Generic reconnecting WebSocket client (frontend-flutter.md section 4.3):
/// "um `ReconnectingWebSocketClient` genérico em `core_networking`
/// (reusável para qualquer outro stream futuro, não só proximidade)".
///
/// Owns exactly one logical connection at a time. While [start]ed, it keeps
/// (re)connecting via [transportFactory] until [stop] is called, backing off
/// exponentially between attempts (see [computeReconnectBackoff]) and
/// exposing every phase transition on [connectionPhase] so a caller (or the
/// UI, via `social_proximity_domain`'s notifier) can render a "reconnecting…"
/// indicator instead of a silently stale feed - never resolves to the UI as
/// still "connected" while a reconnect attempt is in flight.
///
/// Every message received on the underlying transport is republished as-is
/// (raw, undecoded) on [messages] - JSON decoding/frame-type dispatch is a
/// protocol concern left to the caller (`social_proximity_data`), keeping
/// this class protocol-agnostic and genuinely reusable for a future stream.
class ReconnectingWebSocketClient {
  ReconnectingWebSocketClient({
    required Uri Function() uriBuilder,
    SocketTransport Function(Uri uri)? transportFactory,
    this.initialBackoff = const Duration(seconds: 1),
    this.maxBackoff = const Duration(seconds: 30),
    Future<void> Function(Duration duration)? delay,
    Random? random,
  })  : _uriBuilder = uriBuilder,
        _transportFactory =
            transportFactory ?? WebSocketChannelTransport.connect,
        _delay = delay ?? Future.delayed,
        _random = random ?? Random();

  final Uri Function() _uriBuilder;
  final SocketTransport Function(Uri uri) _transportFactory;
  final Duration initialBackoff;
  final Duration maxBackoff;
  final Future<void> Function(Duration duration) _delay;
  final Random _random;

  final StreamController<dynamic> _messagesController =
      StreamController.broadcast();
  final StreamController<SocketConnectionPhase> _phaseController =
      StreamController.broadcast();

  SocketTransport? _transport;
  StreamSubscription<dynamic>? _subscription;
  int _attempt = 0;
  bool _stopped = true;

  /// Incremented on every [start]/[stop] so a connect loop started by a
  /// previous [start] call notices it has been superseded and stops driving
  /// state, even if its in-flight connect/await hasn't unwound yet.
  int _generation = 0;

  /// Completed by [stop] so a pending backoff [_delay] is interrupted
  /// immediately (`Future.any`) instead of the loop waiting out a stale
  /// timer before noticing [_stopped].
  Completer<void>? _stopSignal;

  /// The in-flight "connection ended" completer for the current
  /// [_attemptConnect] call, if any - [stop] completes it directly so a
  /// connect loop suspended waiting on a live session doesn't hang forever
  /// once its subscription is cancelled out from under it (cancelling a
  /// subscription does not itself fire `onDone`/`onError`).
  Completer<bool>? _activeConnectCompleter;

  Stream<dynamic> get messages => _messagesController.stream;

  Stream<SocketConnectionPhase> get connectionPhase => _phaseController.stream;

  bool get isRunning => !_stopped;

  /// Begins the connect-and-auto-reconnect loop. Safe to call again after
  /// [stop] (or repeatedly - a second call while already running is a
  /// no-op).
  void start() {
    if (!_stopped) return;
    _stopped = false;
    _attempt = 0;
    _stopSignal = Completer<void>();
    final generation = ++_generation;
    unawaited(_connectLoop(generation));
  }

  Future<void> _connectLoop(int generation) async {
    while (!_stopped && generation == _generation) {
      _emitPhase(
        _attempt == 0
            ? SocketConnectionPhase.connecting
            : SocketConnectionPhase.reconnecting,
      );

      final everConnected = await _attemptConnect(generation);
      if (generation != _generation) return;
      if (_stopped) break;

      // Reset the counter after any session that actually connected (even
      // if it dropped immediately) - only consecutive *failed* connection
      // attempts should make the backoff grow; a healthy connection that
      // later disconnects always retries at [initialBackoff] first.
      if (everConnected) _attempt = 0;
      final backoff = computeReconnectBackoff(
        _attempt,
        initial: initialBackoff,
        max: maxBackoff,
        random: _random,
      );
      _attempt++;
      _emitPhase(SocketConnectionPhase.reconnecting);
      await Future.any([_delay(backoff), _stopSignal!.future]);
    }
  }

  /// Returns `true` if a connection was established (even if it then
  /// closed/errored); `false` if the connection attempt itself failed
  /// (backoff should grow) or was superseded by a newer [start]/[stop].
  Future<bool> _attemptConnect(int generation) async {
    late final SocketTransport transport;
    try {
      transport = _transportFactory(_uriBuilder());
    } catch (_) {
      return false;
    }
    if (generation != _generation) {
      await transport.close();
      return false;
    }

    _transport = transport;
    _emitPhase(SocketConnectionPhase.connected);

    final completer = Completer<bool>();
    _activeConnectCompleter = completer;
    _subscription = transport.stream.listen(
      (data) {
        if (!_messagesController.isClosed) _messagesController.add(data);
      },
      onError: (Object _, StackTrace __) {
        if (!completer.isCompleted) completer.complete(true);
      },
      onDone: () {
        if (!completer.isCompleted) completer.complete(true);
      },
      cancelOnError: true,
    );

    return completer.future;
  }

  /// Sends a message on the currently connected transport, if any. Silently
  /// drops the message while disconnected/reconnecting - callers that need
  /// delivery guarantees (e.g. heartbeats) should check [isRunning]/observe
  /// [connectionPhase] rather than relying on [send] to buffer or throw.
  void send(dynamic data) => _transport?.send(data);

  /// Stops the connect loop and closes the current connection (if any).
  /// Does not close [messages]/[connectionPhase] - call [dispose] for that,
  /// typically once, at the owner's teardown.
  Future<void> stop() async {
    _stopped = true;
    _generation++;
    if (_stopSignal case final signal? when !signal.isCompleted) {
      signal.complete();
    }
    if (_activeConnectCompleter case final completer? when !completer.isCompleted) {
      completer.complete(false);
    }
    await _subscription?.cancel();
    _subscription = null;
    await _transport?.close();
    _transport = null;
    _emitPhase(SocketConnectionPhase.disconnected);
  }

  void _emitPhase(SocketConnectionPhase phase) {
    if (!_phaseController.isClosed) _phaseController.add(phase);
  }

  Future<void> dispose() async {
    await stop();
    await _messagesController.close();
    await _phaseController.close();
  }
}
