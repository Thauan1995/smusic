import 'package:riverpod/riverpod.dart';
import 'package:social_proximity_domain/social_proximity_domain.dart';
import 'package:test/test.dart';

void main() {
  test('locationProviderProvider throws UnimplementedError by default', () {
    final container = ProviderContainer();
    addTearDown(container.dispose);
    expect(() => container.read(locationProviderProvider), throwsUnimplementedError);
  });

  test('proximityFeedRepositoryProvider throws UnimplementedError by default', () {
    final container = ProviderContainer();
    addTearDown(container.dispose);
    expect(() => container.read(proximityFeedRepositoryProvider), throwsUnimplementedError);
  });

  test('proximityPrivacySettingsRepositoryProvider throws UnimplementedError by default', () {
    final container = ProviderContainer();
    addTearDown(container.dispose);
    expect(
      () => container.read(proximityPrivacySettingsRepositoryProvider),
      throwsUnimplementedError,
    );
  });
}
