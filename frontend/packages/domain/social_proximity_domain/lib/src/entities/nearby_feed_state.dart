import 'package:core_platform/core_platform.dart' show LocationPermissionStatus;

import 'nearby_listener.dart';
import 'proximity_connection_state.dart';

/// [LocationPermissionState] is the task's requested name for exactly
/// `core_platform`'s existing `LocationPermissionStatus` enum
/// (`notRequested | granted | deniedOnce | deniedForever | restricted`) -
/// reused verbatim as a `typedef` rather than duplicated, so there is one
/// definition of the permission state machine in the whole monorepo (the
/// one `core_platform.LocationProvider` already speaks) instead of two
/// enums a mapping function would have to keep in sync. Documented as a
/// deliberate deviation from the task's literal "create a
/// LocationPermissionState" wording, same spirit as Fatia 1's "Desvios da
/// spec" entries.
typedef LocationPermissionState = LocationPermissionStatus;

/// frontend-flutter.md section 4.1: "Estado combinado num
/// `AsyncNotifier<NearbyFeedState>` que funde: lista de ouvintes próximos +
/// status de conexão do socket + status de permissão de localização - a UI
/// reage a um único estado, não a três fontes cruas."
class NearbyFeedState {
  const NearbyFeedState({
    required this.listeners,
    required this.connectionState,
    required this.locationPermission,
  });

  /// Nothing opted-in/permitted/connected yet - the state
  /// `NearbyFeedNotifier` starts from and falls back to whenever the feed
  /// should not be running (feature disabled, paused, consent lapsed, or
  /// location permission not granted).
  const NearbyFeedState.inactive({required this.locationPermission})
      : listeners = const [],
        connectionState = ProximityConnectionState.offline;

  final List<NearbyListener> listeners;
  final ProximityConnectionState connectionState;
  final LocationPermissionState locationPermission;

  NearbyFeedState copyWith({
    List<NearbyListener>? listeners,
    ProximityConnectionState? connectionState,
    LocationPermissionState? locationPermission,
  }) {
    return NearbyFeedState(
      listeners: listeners ?? this.listeners,
      connectionState: connectionState ?? this.connectionState,
      locationPermission: locationPermission ?? this.locationPermission,
    );
  }
}
