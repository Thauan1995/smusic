import 'package:player_domain/player_domain.dart';
import 'package:test/test.dart';

void main() {
  test('toString includes kind and message when set', () {
    final exception = PlayerException(
      PlayerExceptionKind.sessionNotFound,
      message: 'gone',
    );
    expect(exception.toString(), contains('sessionNotFound'));
    expect(exception.toString(), contains('gone'));
  });

  test('toString omits message suffix when null', () {
    final exception = PlayerException(PlayerExceptionKind.network);
    expect(exception.toString(), 'PlayerException(network)');
  });

  test('all kinds exist', () {
    expect(PlayerExceptionKind.values, hasLength(4));
  });
}
