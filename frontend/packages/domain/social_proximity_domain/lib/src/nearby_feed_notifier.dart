import 'dart:async';

import 'package:riverpod/riverpod.dart';

import 'entities/nearby_feed_state.dart';
import 'entities/nearby_listener.dart';
import 'entities/proximity_connection_state.dart';
import 'entities/proximity_privacy_settings.dart';
import 'proximity_privacy_settings_notifier.dart';
import 'location_permission_notifier.dart';
import 'proximity_providers.dart';

/// frontend-flutter.md section 4.1's combined feed/connection/permission
/// state, wired to security.md section 1's activation rules: the feed only
/// ever connects when [ProximityPrivacySettingsNotifier]'s settings report
/// `isActive` (opted in, consent not lapsed, not paused - security.md 1.1/
/// 1.4) *and* location permission is granted. Any other combination
/// disconnects the repository (if connected) and returns
/// [NearbyFeedState.inactive] - this is the single place that decides
/// "should the socket be open right now", so `social_proximity_ui` never
/// has to reason about that policy itself.
class NearbyFeedNotifier extends AsyncNotifier<NearbyFeedState> {
  StreamSubscription<List<NearbyListener>>? _listenersSubscription;
  StreamSubscription<ProximityConnectionState>? _connectionSubscription;

  @override
  Future<NearbyFeedState> build() async {
    // `ref.watch(x.future)` (not `ref.watch(x)`) so this notifier's own
    // `build()` genuinely awaits each dependency's resolved value - both
    // still register the same reactive dependency (this notifier rebuilds
    // whenever either changes), but reading the plain `AsyncValue`
    // synchronously and treating "still loading" as "settings=null" caused
    // build() to resolve on stale (pre-fetch) data in a way that could
    // leave [nearbyFeedProvider]'s own future never settling in tests that
    // read it before priming its dependencies - see the test suite's
    // comment on this for the concrete symptom.
    ProximityPrivacySettings? settings;
    try {
      settings = await ref.watch(proximityPrivacySettingsProvider.future);
    } catch (_) {
      settings = null;
    }

    var permission = LocationPermissionState.notRequested;
    try {
      permission = await ref.watch(locationPermissionProvider.future);
    } catch (_) {
      // keep notRequested - permission truly unknown while checkPermission
      // itself is failing.
    }

    final repository = ref.read(proximityFeedRepositoryProvider);

    ref.onDispose(() {
      _listenersSubscription?.cancel();
      _connectionSubscription?.cancel();
    });

    final shouldRun = settings != null &&
        settings.isActive &&
        permission == LocationPermissionState.granted;

    if (!shouldRun) {
      await repository.disconnect();
      return NearbyFeedState.inactive(locationPermission: permission);
    }

    await repository.connect();

    _listenersSubscription = repository.watch().listen((listeners) {
      final current = state.valueOrNull;
      if (current == null) return;
      state = AsyncData(current.copyWith(listeners: listeners));
    });
    _connectionSubscription = repository.connectionState.listen((connectionState) {
      final current = state.valueOrNull;
      if (current == null) return;
      state = AsyncData(current.copyWith(connectionState: connectionState));
    });

    // Not `reconnecting` - see `ProximityConnectionState`'s doc comment:
    // the very first connect attempt should not render the "reconnecting…"
    // banner. `repository.connectionState`'s first real emission (typically
    // `connected` almost immediately) replaces this via the subscription
    // above.
    return NearbyFeedState(
      listeners: const [],
      connectionState: ProximityConnectionState.offline,
      locationPermission: permission,
    );
  }
}

final nearbyFeedProvider = AsyncNotifierProvider<NearbyFeedNotifier, NearbyFeedState>(
  NearbyFeedNotifier.new,
);
