import 'package:core_design_system/core_design_system.dart';
import 'package:core_platform/testing.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:social_proximity_domain/social_proximity_domain.dart';
import 'package:social_proximity_ui/social_proximity_ui.dart';

Widget _wrap(LocationPermissionState permission, FakeLocationProvider provider) {
  return ProviderScope(
    overrides: [locationProviderProvider.overrideWithValue(provider)],
    child: MaterialApp(
      theme: SmusicTheme.light(),
      home: ProximityPermissionGate(permission: permission),
    ),
  );
}

void main() {
  testWidgets('notRequested shows the ask-permission CTA and requests on tap', (tester) async {
    final provider = FakeLocationProvider()
      ..requestPermissionResult = LocationPermissionStatus.granted;
    await tester.pumpWidget(_wrap(LocationPermissionState.notRequested, provider));
    await tester.pump();

    expect(find.text('Permitir localização'), findsOneWidget);
    await tester.tap(find.text('Permitir localização'));
    await tester.pump();
    await tester.pump();

    expect(provider.permissionStatus, LocationPermissionStatus.granted);
  });

  testWidgets('deniedOnce shows the retry CTA, not the settings CTA', (tester) async {
    final provider = FakeLocationProvider();
    await tester.pumpWidget(_wrap(LocationPermissionState.deniedOnce, provider));
    await tester.pump();

    expect(find.text('Permitir localização'), findsOneWidget);
    expect(find.text('Abrir configurações'), findsNothing);
  });

  testWidgets('deniedForever shows the open-settings CTA and calls openAppSettings on tap', (tester) async {
    final provider = FakeLocationProvider();
    await tester.pumpWidget(_wrap(LocationPermissionState.deniedForever, provider));
    await tester.pump();

    expect(find.text('Abrir configurações'), findsOneWidget);
    await tester.tap(find.text('Abrir configurações'));
    await tester.pump();

    expect(provider.openAppSettingsCalled, isTrue);
  });

  testWidgets('restricted is treated the same as permanently blocked', (tester) async {
    final provider = FakeLocationProvider();
    await tester.pumpWidget(_wrap(LocationPermissionState.restricted, provider));
    await tester.pump();

    expect(find.text('Abrir configurações'), findsOneWidget);
  });
}
