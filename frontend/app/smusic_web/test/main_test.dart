import 'package:core_networking/core_networking.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:smusic_web/main.dart';

class _FakeTokenSource implements AuthTokenSource {
  _FakeTokenSource(this.token);
  String? token;

  @override
  Future<String?> currentAccessToken() async => token;

  @override
  Future<String?> refreshAccessToken() async => token;
}

void main() {
  group('buildPresenceUri', () {
    test('maps http -> ws and appends the presence path', () {
      final uri = buildPresenceUri(apiBaseUrl: 'http://localhost:8080');
      expect(uri.scheme, 'ws');
      expect(uri.path, '/v1/presence/connect');
      expect(uri.queryParameters.containsKey('access_token'), isFalse);
    });

    test('maps https -> wss', () {
      final uri = buildPresenceUri(apiBaseUrl: 'https://api.smusic.example');
      expect(uri.scheme, 'wss');
    });

    test('includes access_token when provided (browser-safe auth per backend/internal/presence/ws)', () {
      final uri = buildPresenceUri(apiBaseUrl: 'https://api.smusic.example', accessToken: 'tok123');
      expect(uri.queryParameters['access_token'], 'tok123');
    });
  });

  group('buildPresenceSocketClient', () {
    test('builds a client whose isRunning starts false (not yet started)', () {
      final client = buildPresenceSocketClient(
        apiBaseUrl: 'https://api.smusic.example',
        tokenSource: _FakeTokenSource('tok123'),
      );
      expect(client.isRunning, isFalse);
    });
  });
}
