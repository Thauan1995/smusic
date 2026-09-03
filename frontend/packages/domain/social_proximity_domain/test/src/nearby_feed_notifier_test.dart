import 'package:core_platform/testing.dart';
import 'package:riverpod/riverpod.dart';
import 'package:social_proximity_domain/social_proximity_domain.dart';
import 'package:test/test.dart';

import '../support/fake_proximity_feed_repository.dart';
import '../support/fake_proximity_privacy_settings_repository.dart';

void main() {
  late FakeProximityFeedRepository feedRepository;
  late FakeProximityPrivacySettingsRepository settingsRepository;
  late FakeLocationProvider locationProvider;
  late ProviderContainer container;

  setUp(() {
    feedRepository = FakeProximityFeedRepository();
    settingsRepository = FakeProximityPrivacySettingsRepository();
    locationProvider = FakeLocationProvider();
    container = ProviderContainer(
      overrides: [
        proximityFeedRepositoryProvider.overrideWithValue(feedRepository),
        proximityPrivacySettingsRepositoryProvider.overrideWithValue(settingsRepository),
        locationProviderProvider.overrideWithValue(locationProvider),
      ],
    );
    addTearDown(container.dispose);
  });

  Future<void> activateEverything({DateTime? consentGivenAt}) async {
    if (consentGivenAt != null) settingsRepository.now = () => consentGivenAt;
    await container.read(proximityPrivacySettingsProvider.notifier).enableFeature();
    locationProvider.permissionStatus = LocationPermissionStatus.granted;
    await container.read(locationPermissionProvider.notifier).refresh();
  }

  test('starts inactive when neither opted in nor permitted', () async {
    final state = await container.read(nearbyFeedProvider.future);
    expect(state.listeners, isEmpty);
    expect(state.connectionState, ProximityConnectionState.offline);
    expect(state.locationPermission, LocationPermissionStatus.notRequested);
    expect(feedRepository.connectCalls, 0);
  });

  test('stays inactive when opted in but permission not granted', () async {
    await container.read(proximityPrivacySettingsProvider.notifier).enableFeature();
    final state = await container.read(nearbyFeedProvider.future);
    expect(feedRepository.connectCalls, 0);
    expect(state.connectionState, ProximityConnectionState.offline);
  });

  test('stays inactive when permitted but not opted in', () async {
    locationProvider.permissionStatus = LocationPermissionStatus.granted;
    await container.read(locationPermissionProvider.notifier).refresh();
    final state = await container.read(nearbyFeedProvider.future);
    expect(feedRepository.connectCalls, 0);
    expect(state.connectionState, ProximityConnectionState.offline);
  });

  test('stays inactive when consent has lapsed', () async {
    await activateEverything(consentGivenAt: DateTime.now().subtract(const Duration(days: 200)));
    final state = await container.read(nearbyFeedProvider.future);
    expect(feedRepository.connectCalls, 0);
    expect(state.connectionState, ProximityConnectionState.offline);
  });

  test('connects once opted in, consent valid, permission granted', () async {
    await activateEverything();
    final state = await container.read(nearbyFeedProvider.future);
    expect(feedRepository.connectCalls, 1);
    expect(state.locationPermission, LocationPermissionStatus.granted);
  });

  test('disconnects when the user pauses discovery (security.md 1.4)', () async {
    await activateEverything();
    await container.read(nearbyFeedProvider.future);
    expect(feedRepository.connectCalls, 1);

    await container.read(proximityPrivacySettingsProvider.notifier).setPaused(true);
    final state = await container.read(nearbyFeedProvider.future);
    expect(state.connectionState, ProximityConnectionState.offline);
    expect(feedRepository.disconnectCalls, greaterThanOrEqualTo(1));
  });

  test('propagates listener updates from the repository', () async {
    await activateEverything();
    await container.read(nearbyFeedProvider.future);

    final listener = NearbyListener(
      userId: 'u1',
      distanceBucket: DistanceBucket.neighborhood,
      revealLevel: RevealLevel.level1,
      displayName: 'Ana',
    );

    final future = container.read(nearbyFeedProvider.notifier).future;
    feedRepository.emitListeners([listener]);
    await future;
    await Future<void>.delayed(Duration.zero);

    final state = container.read(nearbyFeedProvider).value!;
    expect(state.listeners, [listener]);
  });

  test('propagates connection-state updates from the repository', () async {
    await activateEverything();
    await container.read(nearbyFeedProvider.future);

    feedRepository.emitConnectionState(ProximityConnectionState.connected);
    await Future<void>.delayed(Duration.zero);
    expect(
      container.read(nearbyFeedProvider).value!.connectionState,
      ProximityConnectionState.connected,
    );

    feedRepository.emitConnectionState(ProximityConnectionState.reconnecting);
    await Future<void>.delayed(Duration.zero);
    expect(
      container.read(nearbyFeedProvider).value!.connectionState,
      ProximityConnectionState.reconnecting,
    );
  });
}
