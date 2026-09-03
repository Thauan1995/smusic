import 'package:auth_domain/auth_domain.dart';
import 'package:test/test.dart';

void main() {
  test('equal when all fields match', () {
    // Not const on both sides: two `const AuthUser(...)` with identical
    // args canonicalize to the *same* instance, which would short-circuit
    // operator== at `identical(this, other)` and never exercise the
    // field-by-field comparison this test means to cover.
    final a = AuthUser(userId: '1', displayName: 'Ana', email: 'a@b.com');
    final b = AuthUser(userId: '1', displayName: 'Ana', email: 'a@b.com');
    expect(a, b);
    expect(a.hashCode, b.hashCode);
  });

  test('not equal when a field differs', () {
    const a = AuthUser(userId: '1', displayName: 'Ana', email: 'a@b.com');
    const b = AuthUser(userId: '2', displayName: 'Ana', email: 'a@b.com');
    expect(a == b, isFalse);
  });

  test('identical instance equals itself', () {
    const a = AuthUser(userId: '1', displayName: 'Ana', email: 'a@b.com');
    expect(a == a, isTrue);
  });

  test('toString includes all fields', () {
    // Not const - see core_platform's coverage note on const canonicalization.
    final a = AuthUser(userId: '1', displayName: 'Ana', email: 'a@b.com');
    final str = a.toString();
    expect(str, contains('1'));
    expect(str, contains('Ana'));
    expect(str, contains('a@b.com'));
  });
}
