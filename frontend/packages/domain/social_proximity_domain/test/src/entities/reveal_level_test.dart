import 'package:social_proximity_domain/social_proximity_domain.dart';
import 'package:test/test.dart';

void main() {
  test('has exactly 3 values (level0/level1/level2)', () {
    expect(RevealLevel.values, hasLength(3));
  });
}
