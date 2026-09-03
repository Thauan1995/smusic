import 'package:library_domain/library_domain.dart';
import 'package:test/test.dart';

void main() {
  test('toString includes kind and message when set', () {
    final exception = LibraryException(LibraryExceptionKind.notFound, message: 'no track');
    expect(exception.toString(), contains('notFound'));
    expect(exception.toString(), contains('no track'));
  });

  test('toString omits message suffix when null', () {
    final exception = LibraryException(LibraryExceptionKind.network);
    expect(exception.toString(), 'LibraryException(network)');
  });

  test('all kinds exist', () {
    expect(LibraryExceptionKind.values, hasLength(4));
  });
}
