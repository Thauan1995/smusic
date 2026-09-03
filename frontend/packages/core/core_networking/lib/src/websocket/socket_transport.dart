import 'package:web_socket_channel/web_socket_channel.dart';

/// Thin, test-friendly seam over a live socket connection. Deliberately
/// narrower than `WebSocketChannel` (just `stream`/`send`/`close`) so a test
/// double can implement it directly with plain `StreamController`s, without
/// needing to fake `web_socket_channel`'s own internals (which are not
/// designed to be faked - see `WebSocketChannelTransport`'s doc comment).
abstract interface class SocketTransport {
  Stream<dynamic> get stream;

  void send(dynamic data);

  Future<void> close();
}

/// `web_socket_channel`-backed [SocketTransport] - the one class in
/// `core_networking` that imports `package:web_socket_channel`. Works
/// unmodified on mobile and Web because `web_socket_channel` itself ships
/// conditional-import platform backends behind one Dart API (same pattern
/// `just_audio` uses for `core_platform`'s `NativeAudioEngine`, per
/// frontend-flutter.md section 1.3/4.1: "funciona em mobile e web sobre a
/// mesma API").
///
/// COVERAGE EXCLUSION (per docs/architecture/00-overview.md section 2, same
/// category as `JustAudioNativeEngine`/`GeolocatorLocationProvider`): this
/// is a thin binding with no branching logic - `WebSocketChannel.connect`
/// opens a real (or attempted) network connection, which is not something a
/// deterministic unit test should depend on. `ReconnectingWebSocketClient`
/// (the class with the actual reconnect/backoff logic) is fully unit-tested
/// against a hand-written fake [SocketTransport] instead, per
/// `FakeSocketTransport` in `core_networking`'s `testing.dart`.
// coverage:ignore-start
class WebSocketChannelTransport implements SocketTransport {
  WebSocketChannelTransport(this._channel);

  factory WebSocketChannelTransport.connect(Uri uri) =>
      WebSocketChannelTransport(WebSocketChannel.connect(uri));

  final WebSocketChannel _channel;

  @override
  Stream<dynamic> get stream => _channel.stream;

  @override
  void send(dynamic data) => _channel.sink.add(data);

  @override
  Future<void> close() => _channel.sink.close();
}
// coverage:ignore-end
