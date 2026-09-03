import 'package:auth_domain/auth_domain.dart';
import 'package:core_networking/core_networking.dart';

/// Bridges `auth_domain`'s [TokenStorage] to `core_networking`'s
/// [AuthTokenSource] interface, so `ApiClient`'s `AuthInterceptor` can
/// attach/refresh bearer tokens without `core_networking` ever depending on
/// `auth_domain` (see `core_networking.AuthTokenSource`'s own doc comment).
///
/// Construction-order note: [ApiClient] needs an [AuthTokenSource] at
/// construction time, and [AuthRepository] (which knows how to actually
/// call `POST /v1/auth/refresh`) needs an [ApiClient] - a genuine
/// constructor-time cycle. Broken here by making [attachRepository] a
/// post-construction setter: `app/*`'s wiring builds this adapter first
/// (with just [TokenStorage]), builds `ApiClient`/`HttpAuthRepository` from
/// it, then calls [attachRepository] once the repository exists. Calling
/// [refreshAccessToken] before that happens is a wiring bug, not a runtime
/// scenario (no request can fail with 401 before the app has finished
/// composing its dependency graph), so it throws [StateError] rather than
/// silently no-op'ing.
class AuthTokenSourceAdapter implements AuthTokenSource {
  AuthTokenSourceAdapter(this._tokenStorage);

  final TokenStorage _tokenStorage;
  AuthRepository? _repository;

  void attachRepository(AuthRepository repository) {
    _repository = repository;
  }

  @override
  Future<String?> currentAccessToken() async {
    final tokens = await _tokenStorage.read();
    return tokens?.accessToken;
  }

  @override
  Future<String?> refreshAccessToken() async {
    final repository = _repository;
    if (repository == null) {
      throw StateError(
        'AuthTokenSourceAdapter.refreshAccessToken() called before attachRepository() - see class doc comment.',
      );
    }
    final current = await _tokenStorage.read();
    if (current == null) return null;
    try {
      final refreshed = await repository.refresh(refreshToken: current.refreshToken);
      await _tokenStorage.save(refreshed);
      return refreshed.accessToken;
    } on AuthException {
      await _tokenStorage.clear();
      return null;
    }
  }
}
