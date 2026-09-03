import 'package:core_networking/core_networking.dart';
import 'package:test/test.dart';

void main() {
  test('isUnauthorized true only for 401', () {
    expect(const ApiException(message: 'x', statusCode: 401).isUnauthorized, isTrue);
    expect(const ApiException(message: 'x', statusCode: 404).isUnauthorized, isFalse);
  });

  test('isNotFound true only for 404', () {
    expect(const ApiException(message: 'x', statusCode: 404).isNotFound, isTrue);
    expect(const ApiException(message: 'x', statusCode: 401).isNotFound, isFalse);
  });

  test('toString contains statusCode and message', () {
    const exception = ApiException(message: 'boom', statusCode: 500);
    expect(exception.toString(), contains('500'));
    expect(exception.toString(), contains('boom'));
  });

  test('defaults isNetworkError to false', () {
    const exception = ApiException(message: 'x');
    expect(exception.isNetworkError, isFalse);
    expect(exception.statusCode, isNull);
  });
}
