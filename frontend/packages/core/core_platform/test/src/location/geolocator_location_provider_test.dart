import 'package:core_platform/src/location/geolocator_location_provider.dart';
import 'package:core_platform/src/location/location_provider.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:geolocator/geolocator.dart' as geo;

void main() {
  group('mapCheckPermissionResult', () {
    final cases = <geo.LocationPermission, LocationPermissionStatus>{
      geo.LocationPermission.denied: LocationPermissionStatus.notRequested,
      geo.LocationPermission.deniedForever:
          LocationPermissionStatus.deniedForever,
      geo.LocationPermission.whileInUse: LocationPermissionStatus.granted,
      geo.LocationPermission.always: LocationPermissionStatus.granted,
      geo.LocationPermission.unableToDetermine:
          LocationPermissionStatus.restricted,
    };

    for (final entry in cases.entries) {
      test('maps ${entry.key} to ${entry.value}', () {
        expect(mapCheckPermissionResult(entry.key), entry.value);
      });
    }
  });

  group('mapRequestPermissionResult', () {
    final cases = <geo.LocationPermission, LocationPermissionStatus>{
      geo.LocationPermission.denied: LocationPermissionStatus.deniedOnce,
      geo.LocationPermission.deniedForever:
          LocationPermissionStatus.deniedForever,
      geo.LocationPermission.whileInUse: LocationPermissionStatus.granted,
      geo.LocationPermission.always: LocationPermissionStatus.granted,
      geo.LocationPermission.unableToDetermine:
          LocationPermissionStatus.restricted,
    };

    for (final entry in cases.entries) {
      test('maps ${entry.key} to ${entry.value}', () {
        expect(mapRequestPermissionResult(entry.key), entry.value);
      });
    }
  });

  group('mapLocationAccuracy', () {
    final cases = <LocationAccuracy, geo.LocationAccuracy>{
      LocationAccuracy.city: geo.LocationAccuracy.low,
      LocationAccuracy.neighborhood: geo.LocationAccuracy.medium,
      LocationAccuracy.block: geo.LocationAccuracy.high,
      LocationAccuracy.precise: geo.LocationAccuracy.best,
    };

    for (final entry in cases.entries) {
      test('maps ${entry.key} to ${entry.value}', () {
        expect(mapLocationAccuracy(entry.key), entry.value);
      });
    }
  });
}
