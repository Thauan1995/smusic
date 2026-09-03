/// Test-only fakes for `core_networking` interfaces, mirroring
/// `core_platform`'s `testing.dart` entry point (separate from the main
/// library export so production code never accidentally imports a test
/// double).
library core_networking.testing;

export 'src/testing/fake_socket_transport.dart';
export 'src/websocket/socket_transport.dart';
