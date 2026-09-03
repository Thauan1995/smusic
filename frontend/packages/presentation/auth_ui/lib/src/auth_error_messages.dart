import 'package:auth_domain/auth_domain.dart';

/// Shared human-readable copy for [AuthException], used by both the login
/// and signup screens.
String authErrorMessage(Object? error) {
  if (error is AuthException) {
    switch (error.kind) {
      case AuthExceptionKind.invalidCredentials:
        return 'Incorrect email or password.';
      case AuthExceptionKind.emailAlreadyInUse:
        return 'That email is already in use.';
      case AuthExceptionKind.sessionExpired:
        return 'Your session expired. Please log in again.';
      case AuthExceptionKind.network:
        return 'Network error. Check your connection and try again.';
      case AuthExceptionKind.unknown:
        return 'Something went wrong. Please try again.';
    }
  }
  return 'Something went wrong. Please try again.';
}
