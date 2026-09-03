import 'package:auth_domain/auth_domain.dart';

class FakeTokenStorage implements TokenStorage {
  AuthTokens? stored;
  int saveCalls = 0;
  int clearCalls = 0;

  @override
  Future<void> save(AuthTokens tokens) async {
    saveCalls++;
    stored = tokens;
  }

  @override
  Future<AuthTokens?> read() async => stored;

  @override
  Future<void> clear() async {
    clearCalls++;
    stored = null;
  }
}
