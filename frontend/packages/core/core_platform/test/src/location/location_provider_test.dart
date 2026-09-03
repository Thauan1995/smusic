import 'package:core_platform/core_platform.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('GeoPosition carries lat/lng/timestamp', () {
    final now = DateTime(2026, 1, 1);
    final position = GeoPosition(latitude: 1.5, longitude: 2.5, timestamp: now);
    expect(position.latitude, 1.5);
    expect(position.longitude, 2.5);
    expect(position.timestamp, now);
  });

  test('LocationPermissionStatus enum has expected values', () {
    expect(LocationPermissionStatus.values, hasLength(5));
  });

  test('LocationAccuracy enum has expected values', () {
    expect(LocationAccuracy.values, hasLength(4));
  });
}
