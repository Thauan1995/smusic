import 'package:geolocator/geolocator.dart' as geo;

import 'location_provider.dart';

/// Pure mapping functions, extracted from [GeolocatorLocationProvider] for
/// testability (same rationale as `mapJustAudioProcessingState` in
/// `just_audio_native_audio_engine.dart`) - `geo.LocationPermission` and
/// `geo.LocationAccuracy` are plain enums with no platform-channel
/// dependency to construct, so unlike the audio engine's mapping these are
/// directly unit-testable without any platform channel mock.
///
/// ASSUMPTION flagged for the backend/security specialist (security.md
/// section 7's "confirmar que a feature usa apenas localização em
/// foreground" question is about the *client*, this is a related client-side
/// gap): `geolocator` does not expose a "permission dialog never shown yet"
/// state distinct from "user denied once" on `checkPermission()` - both
/// collapse to `LocationPermission.denied`. [mapCheckPermissionResult] treats
/// that as [LocationPermissionStatus.notRequested] (the optimistic reading -
/// "we don't know a prompt was shown"), while [mapRequestPermissionResult]
/// (called only right after actually invoking the OS prompt) treats the same
/// raw value as [LocationPermissionStatus.deniedOnce]. This means a user who
/// denied once, backgrounds the app, and returns will see `checkPermission()`
/// report `notRequested` again rather than `deniedOnce` - functionally
/// harmless here (frontend-flutter.md section 4.4's flow re-shows the value
/// screen and re-requests either way for anything short of
/// `deniedForever`/`restricted`), but worth the backend/product team knowing
/// this client-side ambiguity exists if analytics ever depend on it.
LocationPermissionStatus mapCheckPermissionResult(
  geo.LocationPermission permission,
) {
  switch (permission) {
    case geo.LocationPermission.denied:
      return LocationPermissionStatus.notRequested;
    case geo.LocationPermission.deniedForever:
      return LocationPermissionStatus.deniedForever;
    case geo.LocationPermission.whileInUse:
    case geo.LocationPermission.always:
      return LocationPermissionStatus.granted;
    case geo.LocationPermission.unableToDetermine:
      return LocationPermissionStatus.restricted;
  }
}

LocationPermissionStatus mapRequestPermissionResult(
  geo.LocationPermission permission,
) {
  switch (permission) {
    case geo.LocationPermission.denied:
      return LocationPermissionStatus.deniedOnce;
    case geo.LocationPermission.deniedForever:
      return LocationPermissionStatus.deniedForever;
    case geo.LocationPermission.whileInUse:
    case geo.LocationPermission.always:
      return LocationPermissionStatus.granted;
    case geo.LocationPermission.unableToDetermine:
      return LocationPermissionStatus.restricted;
  }
}

/// security.md section 7 asks the mobile client to confirm foreground-only
/// use; [LocationAccuracy.precise] is intentionally never requested by
/// `social_proximity_data` (see its repository doc comment) - this mapping
/// exists for completeness/interface symmetry and is exercised by tests with
/// every enum value regardless.
geo.LocationAccuracy mapLocationAccuracy(LocationAccuracy accuracy) {
  switch (accuracy) {
    case LocationAccuracy.city:
      return geo.LocationAccuracy.low;
    case LocationAccuracy.neighborhood:
      return geo.LocationAccuracy.medium;
    case LocationAccuracy.block:
      return geo.LocationAccuracy.high;
    case LocationAccuracy.precise:
      return geo.LocationAccuracy.best;
  }
}

/// `geolocator`-backed [LocationProvider] - same package used on mobile and
/// Web per frontend-flutter.md section 1.3's table ("mesmo pacote, funciona
/// nos dois; fallback de precisão degradada tratado dentro da
/// implementação").
///
/// COVERAGE EXCLUSION (per docs/architecture/00-overview.md section 2, same
/// category as `JustAudioNativeEngine`): the instance methods are thin
/// bindings onto `Geolocator`'s static platform-channel calls, which throw
/// `MissingPluginException` under plain `flutter test`. The real logic
/// (permission/accuracy mapping) is extracted above as pure top-level
/// functions and is fully unit-tested. Verified instead by manual
/// `flutter run` smoke test, per frontend/README.md's exclusion list
/// pattern for Fatia 1.
// coverage:ignore-start
class GeolocatorLocationProvider implements LocationProvider {
  const GeolocatorLocationProvider();

  @override
  Stream<GeoPosition> watchPosition({required LocationAccuracy accuracy}) {
    final settings = geo.LocationSettings(
      accuracy: mapLocationAccuracy(accuracy),
      // A modest client-side distance filter in addition to the
      // repository-level time throttle (social_proximity_data) - reduces
      // pointless stream emissions when the device hasn't moved, per
      // frontend-flutter.md section 4.1 ("distanceFilter do geolocator").
      distanceFilter: 10,
    );
    return geo.Geolocator.getPositionStream(locationSettings: settings).map(
      (position) => GeoPosition(
        latitude: position.latitude,
        longitude: position.longitude,
        timestamp: position.timestamp,
        accuracyMeters: position.accuracy,
      ),
    );
  }

  @override
  Future<LocationPermissionStatus> requestPermission() async {
    final serviceEnabled = await geo.Geolocator.isLocationServiceEnabled();
    if (!serviceEnabled) return LocationPermissionStatus.restricted;
    final permission = await geo.Geolocator.requestPermission();
    return mapRequestPermissionResult(permission);
  }

  @override
  Future<LocationPermissionStatus> checkPermission() async {
    final serviceEnabled = await geo.Geolocator.isLocationServiceEnabled();
    if (!serviceEnabled) return LocationPermissionStatus.restricted;
    final permission = await geo.Geolocator.checkPermission();
    return mapCheckPermissionResult(permission);
  }

  @override
  Future<bool> openAppSettings() => geo.Geolocator.openAppSettings();
}
// coverage:ignore-end
