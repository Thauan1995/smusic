import 'package:auth_data/auth_data.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  group('tokensFromLoginResponse', () {
    test('uses response refresh_token when present', () {
      final now = DateTime(2026, 1, 1, 12);
      final tokens = AuthDtos.tokensFromLoginResponse(
        {'access_token': 'a', 'refresh_token': 'r'},
        now: now,
      );
      expect(tokens.accessToken, 'a');
      expect(tokens.refreshToken, 'r');
      expect(tokens.accessTokenExpiresAt, now.add(const Duration(minutes: 15)));
    });

    test('falls back to fallbackRefreshToken when response omits it', () {
      final tokens = AuthDtos.tokensFromLoginResponse(
        {'access_token': 'a'},
        fallbackRefreshToken: 'old-refresh',
      );
      expect(tokens.refreshToken, 'old-refresh');
    });

    test('throws FormatException when neither is available', () {
      expect(
        () => AuthDtos.tokensFromLoginResponse({'access_token': 'a'}),
        throwsFormatException,
      );
    });
  });

  group('userFromMeResponse', () {
    test('reads email from response when present', () {
      final user = AuthDtos.userFromMeResponse({
        'user_id': '1',
        'display_name': 'Ana',
        'email': 'ana@example.com',
      });
      expect(user.email, 'ana@example.com');
    });

    test('falls back to fallbackEmail when response omits email', () {
      final user = AuthDtos.userFromMeResponse(
        {'user_id': '1', 'display_name': 'Ana'},
        fallbackEmail: 'fallback@example.com',
      );
      expect(user.email, 'fallback@example.com');
    });

    test('defaults email to empty string when nothing is available', () {
      final user = AuthDtos.userFromMeResponse({'user_id': '1', 'display_name': 'Ana'});
      expect(user.email, '');
    });
  });
}
