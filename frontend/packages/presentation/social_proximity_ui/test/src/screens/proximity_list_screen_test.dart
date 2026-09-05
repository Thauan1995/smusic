import 'package:core_design_system/core_design_system.dart';
import 'package:core_platform/testing.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:social_proximity_domain/social_proximity_domain.dart';
import 'package:social_proximity_ui/social_proximity_ui.dart';

import '../../support/fake_proximity_feed_repository.dart';
import '../../support/fake_proximity_privacy_settings_repository.dart';

class _Harness {
  _Harness({ProximityPrivacySettings? settings}) {
    settingsRepo = FakeProximityPrivacySettingsRepository(initial: settings);
    feedRepo = FakeProximityFeedRepository();
    locationProvider = FakeLocationProvider();
  }

  late FakeProximityPrivacySettingsRepository settingsRepo;
  late FakeProximityFeedRepository feedRepo;
  late FakeLocationProvider locationProvider;

  Widget wrap({VoidCallback? onOpenSettings}) {
    return ProviderScope(
      overrides: [
        proximityPrivacySettingsRepositoryProvider.overrideWithValue(settingsRepo),
        proximityFeedRepositoryProvider.overrideWithValue(feedRepo),
        locationProviderProvider.overrideWithValue(locationProvider),
      ],
      child: MaterialApp(
        theme: SmusicTheme.light(),
        home: ProximityListScreen(onOpenSettings: onOpenSettings),
      ),
    );
  }
}

void main() {
  testWidgets('not opted in shows the value screen', (tester) async {
    final harness = _Harness();
    await tester.pumpWidget(harness.wrap());
    await tester.pump();

    expect(find.text('Veja quem está ouvindo música perto de você'), findsOneWidget);
  });

  testWidgets('consent lapsed shows the value screen in renewal mode', (tester) async {
    final harness = _Harness(
      settings: ProximityPrivacySettings.disabled().copyWith(
        enabled: true,
        paused: false,
        consentGivenAt: DateTime.now().subtract(const Duration(days: 400)),
        consentRenewalDueAt: DateTime.now().subtract(const Duration(days: 1)),
      ),
    );
    await tester.pumpWidget(harness.wrap());
    await tester.pump();

    expect(
      find.text('Confirme para continuar usando a descoberta por proximidade'),
      findsOneWidget,
    );
  });

  testWidgets('opting in from the value screen requests OS location permission next', (tester) async {
    final harness = _Harness();
    harness.locationProvider.requestPermissionResult = LocationPermissionStatus.deniedOnce;
    await tester.pumpWidget(harness.wrap());
    await tester.pump();

    await tester.ensureVisible(find.text('Ativar descoberta por proximidade'));
    await tester.tap(find.text('Ativar descoberta por proximidade'));
    await tester.pumpAndSettle();

    // enableFeature() completing flips `settings.enabled`, which re-renders
    // past the value screen straight into the permission gate - the
    // onOptedIn callback's `request()` call is what actually invoked the OS
    // prompt via FakeLocationProvider, moving its status away from the
    // initial `notRequested`.
    expect(harness.locationProvider.permissionStatus, LocationPermissionStatus.deniedOnce);
    expect(find.text('Permitir localização'), findsOneWidget);
  });

  testWidgets('opted in but no OS location permission shows the permission gate', (tester) async {
    final harness = _Harness(
      settings: ProximityPrivacySettings.disabled().copyWith(enabled: true, paused: false),
    );
    await tester.pumpWidget(harness.wrap());
    await tester.pump();

    expect(find.text('Permitir localização'), findsOneWidget);
  });

  // .vibeflow/specs/skeleton-loading-player-and-proximity.md: the "who's
  // nearby" list's own loading state (not the earlier settings-fetch
  // gate, which stays a plain spinner - see the comment at this call site
  // in proximity_list_screen.dart) shows a shape-matched skeleton, not a
  // bare spinner.
  testWidgets('shows a nearby-list skeleton while the feed is connecting', (tester) async {
    final harness = _Harness(
      settings: ProximityPrivacySettings.disabled().copyWith(enabled: true, paused: false),
    );
    harness.locationProvider.permissionStatus = LocationPermissionStatus.granted;
    await tester.pumpWidget(harness.wrap());
    // Bounded pumps, never pumpAndSettle: NearbyListSkeleton's shimmer
    // animation repeats forever, so pumpAndSettle would spin until its
    // own timeout instead of reaching a "settled" tree. A couple of
    // pumps is enough to resolve the settings fetch and location
    // permission check and land on the feed provider's own (also never-
    // resolving, since nothing was emitted yet) loading state.
    await tester.pump();
    await tester.pump();

    expect(find.byType(NearbyListSkeleton), findsOneWidget);
    expect(find.byType(CircularProgressIndicator), findsNothing);
  });

  testWidgets('opted in and permitted shows the empty state when nobody is nearby', (tester) async {
    final harness = _Harness(
      settings: ProximityPrivacySettings.disabled().copyWith(enabled: true, paused: false),
    );
    harness.locationProvider.permissionStatus = LocationPermissionStatus.granted;
    await tester.pumpWidget(harness.wrap());
    await tester.pumpAndSettle();

    expect(find.text('Ninguém por perto ouvindo música no momento.'), findsOneWidget);
    expect(harness.feedRepo.connectCalls, 1);
  });

  testWidgets('shows nearby listeners and the pause/settings actions', (tester) async {
    final harness = _Harness(
      settings: ProximityPrivacySettings.disabled().copyWith(enabled: true, paused: false),
    );
    harness.locationProvider.permissionStatus = LocationPermissionStatus.granted;
    var settingsOpened = false;
    await tester.pumpWidget(harness.wrap(onOpenSettings: () => settingsOpened = true));
    await tester.pumpAndSettle();

    harness.feedRepo.emitListeners([
      NearbyListener(userId: 'u1', distanceBucket: DistanceBucket.veryClose, revealLevel: RevealLevel.level0),
    ]);
    await tester.pump();
    await tester.pump();

    expect(find.byType(NearbyListenerCard), findsOneWidget);
    expect(find.byKey(const Key('pause_discovery_toggle')), findsOneWidget);

    await tester.tap(find.byKey(const Key('proximity_open_settings_button')));
    expect(settingsOpened, isTrue);
  });

  testWidgets('reflects connection state via the banner after the delay', (tester) async {
    final harness = _Harness(
      settings: ProximityPrivacySettings.disabled().copyWith(enabled: true, paused: false),
    );
    harness.locationProvider.permissionStatus = LocationPermissionStatus.granted;
    await tester.pumpWidget(harness.wrap());
    await tester.pumpAndSettle();

    harness.feedRepo.emitConnectionState(ProximityConnectionState.reconnecting);
    await tester.pump(const Duration(seconds: 6));

    expect(find.text('Reconectando…'), findsOneWidget);
  });

  testWidgets('shows an error state for the feed and can retry', (tester) async {
    final harness = _Harness(
      settings: ProximityPrivacySettings.disabled().copyWith(enabled: true, paused: false),
    );
    harness.locationProvider.permissionStatus = LocationPermissionStatus.granted;
    harness.feedRepo.connectError = StateError('boom');
    await tester.pumpWidget(harness.wrap());
    await tester.pumpAndSettle();

    expect(find.text('Não foi possível carregar quem está por perto.'), findsOneWidget);

    harness.feedRepo.connectError = null;
    await tester.tap(find.text('Tentar de novo'));
    await tester.pumpAndSettle();

    expect(find.text('Ninguém por perto ouvindo música no momento.'), findsOneWidget);
  });

  testWidgets('shows an error state for the settings fetch and can retry', (tester) async {
    final harness = _Harness();
    harness.settingsRepo.fetchError = StateError('boom');
    await tester.pumpWidget(harness.wrap());
    await tester.pumpAndSettle();

    expect(
      find.text('Não foi possível carregar suas configurações de privacidade.'),
      findsOneWidget,
    );

    harness.settingsRepo.fetchError = null;
    await tester.tap(find.text('Tentar de novo'));
    await tester.pumpAndSettle();

    expect(find.text('Veja quem está ouvindo música perto de você'), findsOneWidget);
  });
}
