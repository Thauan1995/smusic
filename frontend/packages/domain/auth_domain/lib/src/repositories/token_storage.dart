import '../entities/auth_tokens.dart';

/// Persists the current session's tokens across app restarts. Implemented
/// by `auth_data` using `flutter_secure_storage` (task scope item 3: "token
/// armazenamento seguro"). Kept as a domain-level interface (rather than
/// living in `core_platform`, which is reserved for the location/audio/
/// offline-storage triad named in frontend-flutter.md section 1.3) because
/// it is specific to the auth feature, not a cross-feature platform
/// capability.
abstract interface class TokenStorage {
  Future<void> save(AuthTokens tokens);

  Future<AuthTokens?> read();

  Future<void> clear();
}
