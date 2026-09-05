/// Domain-level proximity/presence failure. `social_proximity_data`
/// translates transport errors into this type (same rationale as
/// `auth_domain`'s `AuthException`/`library_domain`'s `LibraryException` -
/// domain must not depend on `core_networking`).
class ProximityException implements Exception {
  const ProximityException(this.kind, {this.message});

  final ProximityExceptionKind kind;
  final String? message;

  @override
  String toString() =>
      'ProximityException(${kind.name}${message != null ? ': $message' : ''})';
}

enum ProximityExceptionKind {
  network,
  unauthorized,

  /// `SettingsService.GrantConsent` (backend/internal/presence/
  /// settings_service.go) rejects granting/renewing proximity consent for
  /// an account with no verified TOTP factor (security.md §2) - a
  /// deliberate step-up-auth gate, not an error. The UI must catch this
  /// specifically and route the user to MFA enrollment, never show it as
  /// a generic failure.
  mfaRequired,
  unknown,
}
