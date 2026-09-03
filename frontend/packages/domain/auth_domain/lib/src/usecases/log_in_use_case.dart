import '../entities/auth_session.dart';
import '../repositories/auth_repository.dart';
import '../repositories/token_storage.dart';

class LogInUseCase {
  const LogInUseCase(this._repository, this._tokenStorage);

  final AuthRepository _repository;
  final TokenStorage _tokenStorage;

  Future<AuthSession> call({
    required String email,
    required String password,
  }) async {
    final session = await _repository.logIn(email: email, password: password);
    await _tokenStorage.save(session.tokens);
    return session;
  }
}
