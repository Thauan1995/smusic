import 'dart:async';

import '../location/location_provider.dart';

/// Deterministic, controllable fake of [LocationProvider] for unit tests
/// (frontend-flutter.md section 5.2 "fakes determinísticos de
/// infraestrutura"), mirroring [FakeNativeAudioEngine]'s shape. Nothing here
/// touches a real GPS/platform channel - every emission is driven manually
/// by the test via the `emit*`/fields below.
class FakeLocationProvider implements LocationProvider {
  LocationPermissionStatus permissionStatus =
      LocationPermissionStatus.notRequested;

  /// If set, [requestPermission] returns this instead of [permissionStatus]
  /// (and also updates [permissionStatus] to match) - lets a test express
  /// "the OS prompt result differs from what `checkPermission` would say".
  LocationPermissionStatus? requestPermissionResult;

  bool openAppSettingsCalled = false;
  bool openAppSettingsReturnValue = true;

  final List<LocationAccuracy> watchPositionCalls = [];

  final StreamController<GeoPosition> _positionController =
      StreamController.broadcast();

  @override
  Stream<GeoPosition> watchPosition({required LocationAccuracy accuracy}) {
    watchPositionCalls.add(accuracy);
    return _positionController.stream;
  }

  @override
  Future<LocationPermissionStatus> requestPermission() async {
    final result = requestPermissionResult ?? permissionStatus;
    permissionStatus = result;
    return result;
  }

  @override
  Future<LocationPermissionStatus> checkPermission() async =>
      permissionStatus;

  @override
  Future<bool> openAppSettings() async {
    openAppSettingsCalled = true;
    return openAppSettingsReturnValue;
  }

  void emitPosition(GeoPosition position) => _positionController.add(position);

  void emitPositionError(Object error) =>
      _positionController.addError(error);

  Future<void> dispose() => _positionController.close();
}
