import '../repositories/auth_repository.dart';
import '../repositories/token_storage.dart';

class LogOutUseCase {
  const LogOutUseCase(this._repository, this._tokenStorage);

  final AuthRepository _repository;
  final TokenStorage _tokenStorage;

  Future<void> call() async {
    final tokens = await _tokenStorage.read();
    // Best-effort server-side revocation: if we have no stored refresh
    // token, or the server call fails, we still clear local state - a
    // signed-out UI must never depend on network reachability.
    if (tokens != null) {
      try {
        await _repository.logOut(refreshToken: tokens.refreshToken);
      } catch (_) {
        // Intentionally swallowed - see comment above.
      }
    }
    await _tokenStorage.clear();
  }
}
