import 'package:auth_domain/auth_domain.dart';
import 'package:auth_ui/auth_ui.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('maps each AuthExceptionKind to a distinct message', () {
    final messages = {
      for (final kind in AuthExceptionKind.values)
        kind: authErrorMessage(AuthException(kind)),
    };
    expect(messages.values.toSet(), hasLength(AuthExceptionKind.values.length));
  });

  test('falls back to a generic message for a non-AuthException error', () {
    expect(authErrorMessage(Exception('boom')), 'Something went wrong. Please try again.');
  });

  test('falls back to a generic message for null', () {
    expect(authErrorMessage(null), 'Something went wrong. Please try again.');
  });
}
