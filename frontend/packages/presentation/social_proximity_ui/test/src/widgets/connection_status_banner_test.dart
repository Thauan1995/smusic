import 'package:core_design_system/core_design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:social_proximity_domain/social_proximity_domain.dart';
import 'package:social_proximity_ui/social_proximity_ui.dart';

Widget _wrap(ProximityConnectionState state, {Duration delay = const Duration(seconds: 5)}) {
  return MaterialApp(
    theme: SmusicTheme.light(),
    home: Scaffold(body: ConnectionStatusBanner(connectionState: state, delay: delay)),
  );
}

void main() {
  testWidgets('renders nothing while connected', (tester) async {
    await tester.pumpWidget(_wrap(ProximityConnectionState.connected));
    await tester.pump();
    expect(find.byKey(const Key('proximity_connection_banner')), findsNothing);
  });

  testWidgets('does not show immediately on disconnect (no flash for a quick blip)', (tester) async {
    await tester.pumpWidget(
      _wrap(ProximityConnectionState.reconnecting, delay: const Duration(seconds: 5)),
    );
    await tester.pump(const Duration(seconds: 1));
    expect(find.byKey(const Key('proximity_connection_banner')), findsNothing);
  });

  testWidgets('shows "Reconectando…" after the delay while reconnecting', (tester) async {
    await tester.pumpWidget(
      _wrap(ProximityConnectionState.reconnecting, delay: const Duration(seconds: 5)),
    );
    await tester.pump(const Duration(seconds: 6));
    expect(find.byKey(const Key('proximity_connection_banner')), findsOneWidget);
    expect(find.text('Reconectando…'), findsOneWidget);
  });

  testWidgets('shows the offline copy after the delay while offline', (tester) async {
    await tester.pumpWidget(
      _wrap(ProximityConnectionState.offline, delay: const Duration(seconds: 5)),
    );
    await tester.pump(const Duration(seconds: 6));
    expect(find.text('Sem conexão com a descoberta por proximidade.'), findsOneWidget);
  });

  testWidgets('reconnecting then connected before the delay hides it again', (tester) async {
    await tester.pumpWidget(_wrap(ProximityConnectionState.reconnecting));
    await tester.pump(const Duration(seconds: 2));

    await tester.pumpWidget(_wrap(ProximityConnectionState.connected));
    await tester.pump(const Duration(seconds: 10));

    expect(find.byKey(const Key('proximity_connection_banner')), findsNothing);
  });
}
