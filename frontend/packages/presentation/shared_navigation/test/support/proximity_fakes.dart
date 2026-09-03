import 'dart:async';

import 'package:social_proximity_domain/social_proximity_domain.dart';

export 'package:core_platform/testing.dart' show FakeLocationProvider;

/// Minimal local fakes so `app_router_test.dart` can exercise the `/nearby`
/// route tree end to end (value screen -> `onOptedIn` -> permission gate ->
/// settings screen) without the real `social_proximity_data` repositories -
/// same rationale as `social_proximity_ui`'s own local fakes (a package's
/// `test/support/` isn't importable from another package).
class FakeProximityPrivacySettingsRepository implements ProximityPrivacySettingsRepository {
  FakeProximityPrivacySettingsRepository({ProximityPrivacySettings? initial})
      : settings = initial ?? ProximityPrivacySettings.disabled();

  ProximityPrivacySettings settings;

  @override
  Future<ProximityPrivacySettings> fetch() async => settings;

  @override
  Future<ProximityPrivacySettings> update(ProximityPrivacySettings newSettings) async {
    settings = newSettings;
    return settings;
  }

  @override
  Future<ProximityPrivacySettings> grantConsent() async {
    settings = settings.copyWith(enabled: true, paused: false);
    return settings;
  }

  @override
  Future<ProximityPrivacySettings> revokeConsent() async {
    settings = settings.copyWith(enabled: false, paused: true);
    return settings;
  }
}

class FakeProximityFeedRepository implements ProximityFeedRepository {
  final StreamController<List<NearbyListener>> _listenersController =
      StreamController.broadcast();
  final StreamController<ProximityConnectionState> _connectionController =
      StreamController.broadcast();

  @override
  Future<void> connect() async {}

  @override
  Future<void> disconnect() async {}

  @override
  Stream<List<NearbyListener>> watch() => _listenersController.stream;

  @override
  Stream<ProximityConnectionState> get connectionState => _connectionController.stream;

  @override
  void setVisibility(ProximityVisibilityMode mode) {}

  @override
  void updateNowPlaying({String? trackId, int positionMs = 0}) {}
}
