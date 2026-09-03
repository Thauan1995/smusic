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

enum ProximityExceptionKind { network, unauthorized, unknown }
