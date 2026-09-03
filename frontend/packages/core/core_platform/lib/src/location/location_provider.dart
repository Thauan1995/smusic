/// Interface-only for Fatia 1. Not wired up or used by any feature yet -
/// `social_proximity_domain`/`social_proximity_data`/`social_proximity_ui`
/// are out of scope for this slice (see docs/architecture/00-overview.md
/// section 3/4). Kept here so the package boundary described in
/// frontend-flutter.md section 1.3 exists from day one and Fatia 2 does not
/// need to introduce a new package.
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
  });

  final double latitude;
  final double longitude;
  final DateTime timestamp;
}

/// See frontend-flutter.md section 1.3 (table) and section 4.4.
///
/// TODO(Fatia 2): implement `GeolocatorLocationProvider` once
/// `social_proximity_domain`/`data`/`ui` are in scope. No production code
/// depends on this interface in Fatia 1.
abstract interface class LocationProvider {
  Stream<GeoPosition> watchPosition({required LocationAccuracy accuracy});

  Future<LocationPermissionStatus> requestPermission();

  Future<LocationPermissionStatus> checkPermission();
}
