import 'package:core_platform/testing.dart';
import 'package:riverpod/riverpod.dart';
import 'package:social_proximity_domain/social_proximity_domain.dart';
import 'package:test/test.dart';

void main() {
  late FakeLocationProvider locationProvider;
  late ProviderContainer container;

  setUp(() {
    locationProvider = FakeLocationProvider();
    container = ProviderContainer(
      overrides: [locationProviderProvider.overrideWithValue(locationProvider)],
    );
    addTearDown(container.dispose);
  });

  test('build() checks (not requests) permission', () async {
    locationProvider.permissionStatus = LocationPermissionStatus.deniedOnce;
    final status = await container.read(locationPermissionProvider.future);
    expect(status, LocationPermissionStatus.deniedOnce);
  });

  test('request() calls requestPermission and updates state', () async {
    await container.read(locationPermissionProvider.future);
    locationProvider.requestPermissionResult = LocationPermissionStatus.granted;
    await container.read(locationPermissionProvider.notifier).request();
    expect(container.read(locationPermissionProvider).value, LocationPermissionStatus.granted);
  });

  test('request() surfaces an error via AsyncValue.guard', () async {
    await container.read(locationPermissionProvider.future);
    final failingProvider = _ThrowingLocationProvider();
    final failingContainer = ProviderContainer(
      overrides: [locationProviderProvider.overrideWithValue(failingProvider)],
    );
    addTearDown(failingContainer.dispose);
    await failingContainer.read(locationPermissionProvider.future).catchError((_) => LocationPermissionStatus.notRequested);
    await failingContainer.read(locationPermissionProvider.notifier).request();
    expect(failingContainer.read(locationPermissionProvider).hasError, isTrue);
  });

  test('refresh() re-checks permission', () async {
    await container.read(locationPermissionProvider.future);
    locationProvider.permissionStatus = LocationPermissionStatus.deniedForever;
    await container.read(locationPermissionProvider.notifier).refresh();
    expect(container.read(locationPermissionProvider).value, LocationPermissionStatus.deniedForever);
  });

  test('openAppSettings delegates to the LocationProvider', () async {
    locationProvider.openAppSettingsReturnValue = true;
    final result = await container.read(locationPermissionProvider.notifier).openAppSettings();
    expect(result, isTrue);
    expect(locationProvider.openAppSettingsCalled, isTrue);
  });
}

class _ThrowingLocationProvider implements LocationProvider {
  @override
  Future<LocationPermissionStatus> checkPermission() async =>
      throw StateError('platform channel unavailable');

  @override
  Future<LocationPermissionStatus> requestPermission() async =>
      throw StateError('platform channel unavailable');

  @override
  Future<bool> openAppSettings() async => false;

  @override
  Stream<GeoPosition> watchPosition({required LocationAccuracy accuracy}) => const Stream.empty();
}
