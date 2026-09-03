import 'dart:async';

import '../websocket/socket_transport.dart';

/// Deterministic, controllable fake of [SocketTransport] for unit tests of
/// [ReconnectingWebSocketClient] (frontend-flutter.md section 5.2:
/// "`FakeReconnectingWebSocketClient` que simula desconexão/reconexão/
/// latência sob controle do teste (`StreamController` manual)" - this class
/// plays that role one layer down, at the transport seam, so the real
/// `ReconnectingWebSocketClient` reconnect/backoff logic itself is exercised
/// unmodified by the test rather than replaced by a bigger fake).
class FakeSocketTransport implements SocketTransport {
  FakeSocketTransport({this.onSend, this.failToClose = false});

  /// Called synchronously whenever [send] is invoked - lets a test assert
  /// what was sent (e.g. the `heartbeat`/`update` JSON frames).
  final void Function(dynamic data)? onSend;

  final bool failToClose;
  bool closed = false;
  final List<dynamic> sentMessages = [];

  final StreamController<dynamic> _controller = StreamController.broadcast();

  @override
  Stream<dynamic> get stream => _controller.stream;

  @override
  void send(dynamic data) {
    sentMessages.add(data);
    onSend?.call(data);
  }

  @override
  Future<void> close() async {
    closed = true;
    if (failToClose) throw StateError('close failed');
    if (!_controller.isClosed) await _controller.close();
  }

  void emitMessage(dynamic data) => _controller.add(data);

  void emitError(Object error) => _controller.addError(error);

  /// Simulates the remote end closing the socket (server drain, network
  /// drop, etc.) - the client's `ReconnectingWebSocketClient` should react
  /// by scheduling a reconnect.
  Future<void> emitDone() => _controller.close();
}
