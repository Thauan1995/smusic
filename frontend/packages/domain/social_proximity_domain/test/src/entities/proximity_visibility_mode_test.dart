import 'package:social_proximity_domain/social_proximity_domain.dart';
import 'package:test/test.dart';

void main() {
  test('has exactly 3 values (invisible/friendsOnly/everyone)', () {
    expect(ProximityVisibilityMode.values, hasLength(3));
  });
}
