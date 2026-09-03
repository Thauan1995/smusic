import '../entities/auth_session.dart';
import '../repositories/auth_repository.dart';
import '../repositories/token_storage.dart';

class SignUpUseCase {
  const SignUpUseCase(this._repository, this._tokenStorage);

  final AuthRepository _repository;
  final TokenStorage _tokenStorage;

  Future<AuthSession> call({
    required String email,
    required String password,
    required String displayName,
  }) async {
    final session = await _repository.signUp(
      email: email,
      password: password,
      displayName: displayName,
    );
    await _tokenStorage.save(session.tokens);
    return session;
  }
}
