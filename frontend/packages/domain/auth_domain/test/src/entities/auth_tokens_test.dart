import 'package:auth_domain/auth_domain.dart';
import 'package:test/test.dart';

void main() {
  group('AuthTokens.isExpired', () {
    test('false when well before expiry', () {
      final tokens = AuthTokens(
        accessToken: 'a',
        refreshToken: 'r',
        accessTokenExpiresAt: DateTime(2026, 1, 1, 12),
      );
      expect(tokens.isExpired(now: DateTime(2026, 1, 1, 11)), isFalse);
    });

    test('true once within the skew window', () {
      final tokens = AuthTokens(
        accessToken: 'a',
        refreshToken: 'r',
        accessTokenExpiresAt: DateTime(2026, 1, 1, 12, 0, 0),
      );
      expect(
        tokens.isExpired(now: DateTime(2026, 1, 1, 11, 59, 45)),
        isTrue,
      );
    });

    test('true after expiry', () {
      final tokens = AuthTokens(
        accessToken: 'a',
        refreshToken: 'r',
        accessTokenExpiresAt: DateTime(2026, 1, 1, 12),
      );
      expect(tokens.isExpired(now: DateTime(2026, 1, 1, 13)), isTrue);
    });

    test('defaults now to DateTime.now()', () {
      final tokens = AuthTokens(
        accessToken: 'a',
        refreshToken: 'r',
        accessTokenExpiresAt: DateTime.now().add(const Duration(hours: 1)),
      );
      expect(tokens.isExpired(), isFalse);
    });
  });

  group('AuthTokens equality', () {
    test('equal when all fields match', () {
      final expiresAt = DateTime(2026, 1, 1);
      final a = AuthTokens(accessToken: 'x', refreshToken: 'y', accessTokenExpiresAt: expiresAt);
      final b = AuthTokens(accessToken: 'x', refreshToken: 'y', accessTokenExpiresAt: expiresAt);
      expect(a, b);
      expect(a.hashCode, b.hashCode);
    });

    test('not equal when a field differs', () {
      final expiresAt = DateTime(2026, 1, 1);
      final a = AuthTokens(accessToken: 'x', refreshToken: 'y', accessTokenExpiresAt: expiresAt);
      final b = AuthTokens(accessToken: 'z', refreshToken: 'y', accessTokenExpiresAt: expiresAt);
      expect(a == b, isFalse);
    });
  });
}
