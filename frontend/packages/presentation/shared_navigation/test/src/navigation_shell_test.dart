import 'package:core_design_system/core_design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:player_domain/player_domain.dart';
import 'package:shared_navigation/shared_navigation.dart';

import '../support/fakes.dart';

Widget _harness(FakePlaybackQueueController controller, {String initialLocation = '/library'}) {
  // Real matching paths: NavigationShell's destinations navigate via
  // `context.go('/library')`/`context.go('/search')` internally (hardcoded
  // to match app_router.dart's actual route tree), so the test harness's
  // own routes have to use those same paths for a destination tap to
  // resolve to anything.
  final router = GoRouter(
    initialLocation: initialLocation,
    routes: [
      GoRoute(
        path: '/library',
        builder: (context, state) => const NavigationShell(
          currentLocation: '/library',
          child: Text('Library body'),
        ),
      ),
      GoRoute(
        path: '/search',
        builder: (context, state) => const NavigationShell(
          currentLocation: '/search',
          child: Text('Search body'),
        ),
      ),
    ],
  );
  return ProviderScope(
    overrides: [playbackQueueControllerProvider.overrideWithValue(controller)],
    child: MaterialApp.router(theme: SmusicTheme.light(), routerConfig: router),
  );
}

void main() {
  late FakePlaybackQueueController controller;

  setUp(() => controller = FakePlaybackQueueController());
  tearDown(() => controller.dispose());

  testWidgets('renders a NavigationBar at compact width', (tester) async {
    tester.view.physicalSize = const Size(400, 800);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    await tester.pumpWidget(_harness(controller));
    await tester.pumpAndSettle();

    expect(find.byType(NavigationBar), findsOneWidget);
    expect(find.byType(NavigationRail), findsNothing);
    expect(find.text('Library body'), findsOneWidget);
  });

  testWidgets('renders a NavigationRail at expanded width', (tester) async {
    tester.view.physicalSize = const Size(1400, 900);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    await tester.pumpWidget(_harness(controller));
    await tester.pumpAndSettle();

    expect(find.byType(NavigationRail), findsOneWidget);
    expect(find.byType(NavigationBar), findsNothing);
    final rail = tester.widget<NavigationRail>(find.byType(NavigationRail));
    expect(rail.extended, isTrue);
  });

  testWidgets('renders a non-extended NavigationRail at medium width', (tester) async {
    tester.view.physicalSize = const Size(800, 900);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    await tester.pumpWidget(_harness(controller));
    await tester.pumpAndSettle();

    final rail = tester.widget<NavigationRail>(find.byType(NavigationRail));
    expect(rail.extended, isFalse);
  });

  testWidgets('tapping the Search destination navigates via go_router', (tester) async {
    tester.view.physicalSize = const Size(400, 800);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    await tester.pumpWidget(_harness(controller));
    await tester.pumpAndSettle();

    await tester.tap(find.text('Search'));
    await tester.pumpAndSettle();

    expect(find.text('Search body'), findsOneWidget);
  });

  testWidgets('highlights the Search destination when currentLocation is /search', (tester) async {
    tester.view.physicalSize = const Size(400, 800);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    await tester.pumpWidget(_harness(controller, initialLocation: '/search'));
    await tester.pumpAndSettle();

    final navBar = tester.widget<NavigationBar>(find.byType(NavigationBar));
    expect(navBar.selectedIndex, 1);
  });
}
