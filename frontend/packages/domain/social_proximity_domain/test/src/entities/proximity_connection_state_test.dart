import 'package:social_proximity_domain/social_proximity_domain.dart';
import 'package:test/test.dart';

void main() {
  test('has exactly 3 values (connected/reconnecting/offline)', () {
    expect(ProximityConnectionState.values, hasLength(3));
    expect(ProximityConnectionState.values, contains(ProximityConnectionState.connected));
    expect(ProximityConnectionState.values, contains(ProximityConnectionState.reconnecting));
    expect(ProximityConnectionState.values, contains(ProximityConnectionState.offline));
  });
}
