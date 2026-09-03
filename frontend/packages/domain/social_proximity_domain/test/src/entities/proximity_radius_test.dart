import 'package:social_proximity_domain/social_proximity_domain.dart';
import 'package:test/test.dart';

void main() {
  test('meters match security.md 1.3 thresholds', () {
    expect(ProximityRadius.m150.meters, 150);
    expect(ProximityRadius.km1.meters, 1000);
    expect(ProximityRadius.km5.meters, 5000);
    expect(ProximityRadius.km15.meters, 15000);
  });

  test('labels are human readable', () {
    expect(ProximityRadius.m150.label, '150 m');
    expect(ProximityRadius.km1.label, '1 km');
    expect(ProximityRadius.km5.label, '5 km');
    expect(ProximityRadius.km15.label, '15 km');
  });

  test('default is 1km per security.md 1.3', () {
    expect(ProximityRadius.defaultValue, ProximityRadius.km1);
  });
}
