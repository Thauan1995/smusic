import 'package:core_design_system/core_design_system.dart';
import 'package:core_platform/testing.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:social_proximity_domain/social_proximity_domain.dart';
import 'package:social_proximity_ui/social_proximity_ui.dart';

import '../../support/fake_proximity_privacy_settings_repository.dart';

Widget _wrap(
  FakeProximityPrivacySettingsRepository repo, {
  VoidCallback? onOptedIn,
  bool isRenewal = false,
}) {
  return ProviderScope(
    overrides: [
      proximityPrivacySettingsRepositoryProvider.overrideWithValue(repo),
      locationProviderProvider.overrideWithValue(FakeLocationProvider()),
    ],
    child: MaterialApp(
      theme: SmusicTheme.light(),
      home: ProximityValueScreen(onOptedIn: onOptedIn, isRenewal: isRenewal),
    ),
  );
}

void main() {
  testWidgets('shows the activation copy and CTA by default', (tester) async {
    final repo = FakeProximityPrivacySettingsRepository();
    await tester.pumpWidget(_wrap(repo));
    await tester.pump();

    expect(find.text('Veja quem está ouvindo música perto de você'), findsOneWidget);
    expect(find.text('Ativar descoberta por proximidade'), findsOneWidget);
  });

  testWidgets('shows renewal copy when isRenewal is true', (tester) async {
    final repo = FakeProximityPrivacySettingsRepository();
    await tester.pumpWidget(_wrap(repo, isRenewal: true));
    await tester.pump();

    expect(
      find.text('Confirme para continuar usando a descoberta por proximidade'),
      findsOneWidget,
    );
    expect(find.text('Confirmar e continuar'), findsOneWidget);
  });

  testWidgets('tapping the CTA calls enableFeature and invokes onOptedIn', (tester) async {
    final repo = FakeProximityPrivacySettingsRepository();
    var optedIn = false;
    await tester.pumpWidget(_wrap(repo, onOptedIn: () => optedIn = true));
    await tester.pump();

    await tester.ensureVisible(find.text('Ativar descoberta por proximidade'));
    await tester.tap(find.text('Ativar descoberta por proximidade'));
    await tester.pump();
    await tester.pump();

    expect(repo.grantConsentCalls, 1);
    expect(repo.settings.enabled, isTrue);
    expect(optedIn, isTrue);
  });
}
