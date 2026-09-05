import 'package:core_design_system/core_design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:social_proximity_domain/social_proximity_domain.dart';
import 'package:social_proximity_ui/social_proximity_ui.dart';

import '../../support/fake_proximity_privacy_settings_repository.dart';

Widget _wrap(FakeProximityPrivacySettingsRepository repo) {
  return ProviderScope(
    overrides: [proximityPrivacySettingsRepositoryProvider.overrideWithValue(repo)],
    child: MaterialApp(
      theme: SmusicTheme.light(),
      home: const Scaffold(appBar: null, body: PauseDiscoveryToggle()),
    ),
  );
}

void main() {
  testWidgets('renders nothing when the feature is not enabled', (tester) async {
    final repo = FakeProximityPrivacySettingsRepository(
      initial: ProximityPrivacySettings.disabled(),
    );
    await tester.pumpWidget(_wrap(repo));
    await tester.pump();
    expect(find.byKey(const Key('pause_discovery_toggle')), findsNothing);
  });

  testWidgets('shows a pause icon when enabled and not paused; tapping pauses', (tester) async {
    final repo = FakeProximityPrivacySettingsRepository(
      initial: ProximityPrivacySettings.disabled().copyWith(enabled: true, paused: false),
    );
    await tester.pumpWidget(_wrap(repo));
    await tester.pump();

    // Filled: discovery is active/in-progress - see
    // .vibeflow/specs/icon-system-consistency.md.
    expect(find.byIcon(Icons.pause_circle_filled), findsOneWidget);
    await tester.tap(find.byKey(const Key('pause_discovery_toggle')));
    await tester.pump();
    await tester.pump();

    expect(repo.settings.paused, isTrue);
  });

  testWidgets('shows a resume icon when paused; tapping resumes', (tester) async {
    final repo = FakeProximityPrivacySettingsRepository(
      initial: ProximityPrivacySettings.disabled().copyWith(enabled: true, paused: true),
    );
    await tester.pumpWidget(_wrap(repo));
    await tester.pump();

    expect(find.byIcon(Icons.play_circle_outline), findsOneWidget);
    await tester.tap(find.byKey(const Key('pause_discovery_toggle')));
    await tester.pump();
    await tester.pump();

    expect(repo.settings.paused, isFalse);
  });
}
