import '../entities/auth_exception.dart';
import '../entities/auth_session.dart';
import '../repositories/auth_repository.dart';
import '../repositories/token_storage.dart';

/// Run once at app startup (`AuthSessionNotifier`'s initial state build) to
/// restore a previously signed-in session from [TokenStorage], if any.
class RestoreSessionUseCase {
  const RestoreSessionUseCase(this._repository, this._tokenStorage);

  final AuthRepository _repository;
  final TokenStorage _tokenStorage;

  /// Returns `null` when there is no stored session (never signed in, or
  /// previously signed out) - this is a normal, expected outcome, not an
  /// error.
  Future<AuthSession?> call() async {
    final storedTokens = await _tokenStorage.read();
    if (storedTokens == null) return null;

    var tokens = storedTokens;
    if (tokens.isExpired()) {
      try {
        tokens = await _repository.refresh(refreshToken: tokens.refreshToken);
        await _tokenStorage.save(tokens);
      } on AuthException {
        await _tokenStorage.clear();
        return null;
      }
    }

    try {
      final user = await _repository.getCurrentUser();
      return AuthSession(user: user, tokens: tokens);
    } on AuthException {
      await _tokenStorage.clear();
      return null;
    }
  }
}
