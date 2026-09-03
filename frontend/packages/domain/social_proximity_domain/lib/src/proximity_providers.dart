import 'package:core_platform/core_platform.dart' show LocationProvider;
import 'package:riverpod/riverpod.dart';

import 'repositories/proximity_feed_repository.dart';
import 'repositories/proximity_privacy_settings_repository.dart';

/// Overridden in `app/*` with `core_platform`'s `GeolocatorLocationProvider`
/// - same composition pattern as `player_domain`'s
/// `playbackQueueControllerProvider` (frontend-flutter.md section 1.3:
/// concrete platform implementations are registered only at the app
/// composition root).
final locationProviderProvider = Provider<LocationProvider>((ref) {
  throw UnimplementedError(
    'locationProviderProvider must be overridden by app/* with a core_platform implementation.',
  );
});

/// Overridden in `app/*` with `social_proximity_data`'s
/// `WebSocketProximityFeedRepository`.
final proximityFeedRepositoryProvider = Provider<ProximityFeedRepository>((ref) {
  throw UnimplementedError(
    'proximityFeedRepositoryProvider must be overridden by app/* with a social_proximity_data implementation.',
  );
});

/// Overridden in `app/*` with `social_proximity_data`'s
/// `HttpProximityPrivacySettingsRepository`.
final proximityPrivacySettingsRepositoryProvider =
    Provider<ProximityPrivacySettingsRepository>((ref) {
  throw UnimplementedError(
    'proximityPrivacySettingsRepositoryProvider must be overridden by app/* with a social_proximity_data implementation.',
  );
});
