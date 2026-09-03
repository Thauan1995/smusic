/// Domain-level library/catalog failure. `library_data` translates
/// transport errors into this type (same rationale as `auth_domain`'s
/// `AuthException` - domain must not depend on `core_networking`).
class LibraryException implements Exception {
  const LibraryException(this.kind, {this.message});

  final LibraryExceptionKind kind;
  final String? message;

  @override
  String toString() =>
      'LibraryException(${kind.name}${message != null ? ': $message' : ''})';
}

enum LibraryExceptionKind { notFound, network, unauthorized, unknown }
