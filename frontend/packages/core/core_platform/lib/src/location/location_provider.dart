/// Implemented (Fatia 2) by [GeolocatorLocationProvider] - see
/// `geolocator_location_provider.dart`. Consumed by
/// `social_proximity_domain`'s `LocationPermissionNotifier` and by
/// `social_proximity_data`'s `WebSocketProximityFeedRepository` (which
/// throttles [watchPosition]'s stream before forwarding updates to the
/// backend - see that class's doc comment).
library;

enum LocationAccuracy { city, neighborhood, block, precise }

enum LocationPermissionStatus {
  notRequested,
  granted,
  deniedOnce,
  deniedForever,
  restricted,
}

class GeoPosition {
  const GeoPosition({
    required this.latitude,
    required this.longitude,
    required this.timestamp,
    this.accuracyMeters = 0,
  });

  final double latitude;
  final double longitude;
  final DateTime timestamp;

  /// Horizontal accuracy in meters (`Position.accuracy` on `geolocator`) -
  /// added on top of the section 1.3 illustrative snippet (additive, same
  /// pattern as [LocationProvider.openAppSettings]) because backend-go.md
  /// section 4's `update` WS frame requires an `accuracy_m` field and there
  /// was previously nowhere on this type to source it from. Defaults to `0`
  /// rather than being nullable so existing call sites/tests that predate
  /// this field keep compiling unchanged.
  final double accuracyMeters;
}

/// See frontend-flutter.md section 1.3 (table) and section 4.4.
abstract interface class LocationProvider {
  Stream<GeoPosition> watchPosition({required LocationAccuracy accuracy});

  Future<LocationPermissionStatus> requestPermission();

  Future<LocationPermissionStatus> checkPermission();

  /// Opens the OS app-settings screen (`geolocator.openAppSettings()`) so
  /// the user can flip location permission back on after
  /// [LocationPermissionStatus.deniedForever] - frontend-flutter.md section
  /// 4.4: "CTA para abrir configurações do SO, nunca insiste com prompt
  /// repetido". Not in the section 1.3 illustrative snippet (additive, same
  /// pattern as `PlaybackQueueController`'s pause/resume in Fatia 1's
  /// README "Desvios da spec").
  ///
  /// Returns whether the settings screen could be opened (mirrors
  /// `geolocator`'s own return value); the caller should not treat `false`
  /// as an error worth surfacing beyond leaving the CTA visible.
  Future<bool> openAppSettings();
}
