import 'package:core_platform/testing.dart' show LocationPermissionStatus;
import 'package:social_proximity_domain/social_proximity_domain.dart';
import 'package:test/test.dart';

void main() {
  test('LocationPermissionState is core_platform.LocationPermissionStatus', () {
    const LocationPermissionState value = LocationPermissionStatus.granted;
    expect(value, LocationPermissionStatus.granted);
  });

  test('inactive() starts empty/offline with the given permission', () {
    const state = NearbyFeedState.inactive(
      locationPermission: LocationPermissionStatus.deniedForever,
    );
    expect(state.listeners, isEmpty);
    expect(state.connectionState, ProximityConnectionState.offline);
    expect(state.locationPermission, LocationPermissionStatus.deniedForever);
  });

  test('copyWith only overrides provided fields', () {
    const state = NearbyFeedState.inactive(
      locationPermission: LocationPermissionStatus.notRequested,
    );
    final updated = state.copyWith(connectionState: ProximityConnectionState.connected);
    expect(updated.connectionState, ProximityConnectionState.connected);
    expect(updated.listeners, state.listeners);
    expect(updated.locationPermission, state.locationPermission);
  });
}
