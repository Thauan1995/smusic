import 'package:core_platform/testing.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  late FakeLocationProvider provider;

  setUp(() {
    provider = FakeLocationProvider();
  });

  test('defaults to notRequested', () async {
    expect(await provider.checkPermission(), LocationPermissionStatus.notRequested);
  });

  test('requestPermission returns and stores permissionStatus by default', () async {
    provider.permissionStatus = LocationPermissionStatus.granted;
    final result = await provider.requestPermission();
    expect(result, LocationPermissionStatus.granted);
    expect(await provider.checkPermission(), LocationPermissionStatus.granted);
  });

  test('requestPermissionResult overrides the returned/stored status', () async {
    provider.permissionStatus = LocationPermissionStatus.notRequested;
    provider.requestPermissionResult = LocationPermissionStatus.deniedOnce;
    final result = await provider.requestPermission();
    expect(result, LocationPermissionStatus.deniedOnce);
    expect(await provider.checkPermission(), LocationPermissionStatus.deniedOnce);
  });

  test('openAppSettings records the call and returns configured value', () async {
    provider.openAppSettingsReturnValue = false;
    final result = await provider.openAppSettings();
    expect(result, isFalse);
    expect(provider.openAppSettingsCalled, isTrue);
  });

  test('watchPosition records requested accuracy and forwards emitted positions', () async {
    final positions = <GeoPosition>[];
    provider.watchPosition(accuracy: LocationAccuracy.block).listen(positions.add);
    expect(provider.watchPositionCalls, [LocationAccuracy.block]);

    final position = GeoPosition(latitude: 1, longitude: 2, timestamp: DateTime(2026));
    provider.emitPosition(position);
    await Future<void>.delayed(Duration.zero);
    expect(positions, [position]);
  });

  test('emitPositionError forwards an error through watchPosition', () async {
    final errors = <Object>[];
    provider.watchPosition(accuracy: LocationAccuracy.city).listen(
          (_) {},
          onError: errors.add,
        );
    provider.emitPositionError('gps unavailable');
    await Future<void>.delayed(Duration.zero);
    expect(errors, ['gps unavailable']);
  });

  test('dispose closes the position stream', () async {
    await provider.dispose();
    expect(provider.watchPosition(accuracy: LocationAccuracy.city), emitsDone);
  });
}
