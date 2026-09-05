/// Domain-level auth failure. `auth_data`'s repository implementation
/// translates transport errors (`core_networking`'s `ApiException`) into
/// this type so `auth_domain`/`presentation` never see a networking type -
/// domain must not depend on `core_networking`
/// (docs/architecture/frontend-flutter.md section 1.2).
class AuthException implements Exception {
  const AuthException(this.kind, {this.message});

  final AuthExceptionKind kind;
  final String? message;

  @override
  String toString() => 'AuthException(${kind.name}${message != null ? ': $message' : ''})';
}

enum AuthExceptionKind {
  invalidCredentials,
  emailAlreadyInUse,
  sessionExpired,

  /// `POST /v1/auth/mfa/verify` rejected the code (wrong or expired TOTP
  /// value) - distinct from [invalidCredentials] so the MFA enrollment
  /// screen can show "código inválido, tente novamente" instead of
  /// bouncing the user back to the login screen.
  invalidMfaCode,
  network,
  unknown,
}
