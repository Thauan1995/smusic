import 'package:auth_domain/auth_domain.dart';
import 'package:test/test.dart';

void main() {
  test('toString includes kind and message when message is set', () {
    // Not const - see core_platform's coverage note on const canonicalization.
    final exception = AuthException(
      AuthExceptionKind.invalidCredentials,
      message: 'bad creds',
    );
    expect(exception.toString(), contains('invalidCredentials'));
    expect(exception.toString(), contains('bad creds'));
  });

  test('toString omits message suffix when message is null', () {
    final exception = AuthException(AuthExceptionKind.network);
    expect(exception.toString(), 'AuthException(network)');
  });

  test('all AuthExceptionKind values exist', () {
    expect(AuthExceptionKind.values, hasLength(5));
  });
}
