import 'package:auth_domain/auth_domain.dart';
import 'package:test/test.dart';

void main() {
  test('carries user and tokens', () {
    final user = AuthUser(userId: '1', displayName: 'Ana', email: 'a@b.com');
    final tokens = AuthTokens(
      accessToken: 'a',
      refreshToken: 'r',
      accessTokenExpiresAt: DateTime(2026, 1, 1),
    );
    final session = AuthSession(user: user, tokens: tokens);
    expect(session.user, user);
    expect(session.tokens, tokens);
  });
}
