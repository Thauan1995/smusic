import 'package:player_domain/player_domain.dart';
import 'package:test/test.dart';

void main() {
  test('toString includes message', () {
    final error = PlayerError('codec error', cause: 'boom');
    expect(error.toString(), 'PlayerError(codec error)');
    expect(error.cause, 'boom');
  });

  test('equal when message and cause match (non-identical instances)', () {
    final a = PlayerError('x', cause: 'c');
    final b = PlayerError('x', cause: 'c');
    expect(a, b);
    expect(a.hashCode, b.hashCode);
  });

  test('not equal when message differs', () {
    final a = PlayerError('x');
    final b = PlayerError('y');
    expect(a == b, isFalse);
  });
}
