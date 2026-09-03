import 'dart:convert';

import 'package:auth_domain/auth_domain.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';

/// [TokenStorage] implementation backed by `flutter_secure_storage`
/// (Keychain on iOS/macOS, Keystore-backed EncryptedSharedPreferences on
/// Android, `localStorage` behind a Web Crypto-wrapped key on Web - the
/// same package works unmodified on both platforms, matching the reuse
/// requirement described for `LocationProvider`/`NativeAudioEngine` in
/// frontend-flutter.md section 1.3).
class SecureTokenStorage implements TokenStorage {
  SecureTokenStorage({FlutterSecureStorage? storage})
      : _storage = storage ?? const FlutterSecureStorage();

  static const _key = 'smusic.auth_tokens';

  final FlutterSecureStorage _storage;

  @override
  Future<void> save(AuthTokens tokens) async {
    final json = jsonEncode({
      'access_token': tokens.accessToken,
      'refresh_token': tokens.refreshToken,
      'access_token_expires_at': tokens.accessTokenExpiresAt.toIso8601String(),
    });
    await _storage.write(key: _key, value: json);
  }

  @override
  Future<AuthTokens?> read() async {
    final raw = await _storage.read(key: _key);
    if (raw == null) return null;
    try {
      final json = jsonDecode(raw) as Map<String, dynamic>;
      return AuthTokens(
        accessToken: json['access_token'] as String,
        refreshToken: json['refresh_token'] as String,
        accessTokenExpiresAt:
            DateTime.parse(json['access_token_expires_at'] as String),
      );
    } on FormatException {
      // Corrupt/legacy stored value - treat as "no session" rather than
      // crashing app startup.
      await clear();
      return null;
    }
  }

  @override
  Future<void> clear() async {
    await _storage.delete(key: _key);
  }
}
