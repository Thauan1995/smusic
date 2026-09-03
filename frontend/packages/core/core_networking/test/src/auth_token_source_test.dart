import 'package:core_networking/core_networking.dart';
import 'package:test/test.dart';

void main() {
  test('NoAuthTokenSource returns null for current and refreshed token', () async {
    const source = NoAuthTokenSource();
    expect(await source.currentAccessToken(), isNull);
    expect(await source.refreshAccessToken(), isNull);
  });
}
