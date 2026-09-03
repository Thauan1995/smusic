import 'package:core_design_system/core_design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:player_domain/player_domain.dart';
import 'package:player_ui/player_ui.dart';

import '../../support/fake_playback_queue_controller.dart';

QueueItem _item() => const QueueItem(
      trackId: 't1',
      title: 'Song',
      artistName: 'Artist',
      durationMs: 200000,
    );

Widget _wrap(Widget child, FakePlaybackQueueController controller) {
  return ProviderScope(
    overrides: [playbackQueueControllerProvider.overrideWithValue(controller)],
    child: MaterialApp(theme: SmusicTheme.light(), home: Scaffold(bottomNavigationBar: child)),
  );
}

void main() {
  late FakePlaybackQueueController controller;

  setUp(() => controller = FakePlaybackQueueController());
  tearDown(() => controller.dispose());

  testWidgets('renders nothing when there is no now-playing item', (tester) async {
    await tester.pumpWidget(_wrap(const MiniPlayerBar(), controller));
    await tester.pump();

    expect(find.byType(MiniPlayerBar), findsOneWidget);
    expect(find.text('Song'), findsNothing);
  });

  testWidgets('renders nothing while state is idle even if nowPlaying briefly has a value', (tester) async {
    await tester.pumpWidget(_wrap(const MiniPlayerBar(), controller));
    await tester.pump();

    controller.emitNowPlaying(_item());
    controller.emitState(const PlayerState.idle());
    await tester.pump();

    expect(find.text('Song'), findsNothing);
  });

  testWidgets('shows title/artist and a pause button while playing', (tester) async {
    await tester.pumpWidget(_wrap(MiniPlayerBar(onExpand: () {}), controller));
    await tester.pump();

    controller.emitNowPlaying(_item());
    controller.emitState(PlayerState.playing(_item(), const Duration(seconds: 5)));
    await tester.pump();
    await tester.pump();

    expect(find.text('Song'), findsOneWidget);
    expect(find.text('Artist'), findsOneWidget);
    expect(find.byIcon(Icons.pause), findsOneWidget);
  });

  testWidgets('shows a play button while paused, and pressing it resumes', (tester) async {
    await tester.pumpWidget(_wrap(const MiniPlayerBar(), controller));
    await tester.pump();

    controller.emitNowPlaying(_item());
    controller.emitState(PlayerState.paused(_item(), const Duration(seconds: 5)));
    await tester.pump();
    await tester.pump();

    expect(find.byIcon(Icons.play_arrow), findsOneWidget);
    await tester.tap(find.byKey(const Key('mini_player_play_pause_button')));
    expect(controller.resumeCalls, 1);
  });

  testWidgets('pressing pause while playing calls pause()', (tester) async {
    await tester.pumpWidget(_wrap(const MiniPlayerBar(), controller));
    await tester.pump();

    controller.emitNowPlaying(_item());
    controller.emitState(PlayerState.playing(_item(), Duration.zero));
    await tester.pump();
    await tester.pump();

    await tester.tap(find.byKey(const Key('mini_player_play_pause_button')));
    expect(controller.pauseCalls, 1);
  });

  testWidgets('shows a spinner instead of a play/pause button while buffering', (tester) async {
    await tester.pumpWidget(_wrap(const MiniPlayerBar(), controller));
    await tester.pump();

    controller.emitNowPlaying(_item());
    controller.emitState(PlayerState.buffering(_item()));
    await tester.pump();
    await tester.pump();

    expect(find.byType(CircularProgressIndicator), findsOneWidget);
    expect(find.byKey(const Key('mini_player_play_pause_button')), findsNothing);
  });

  testWidgets('the next button calls skipNext', (tester) async {
    await tester.pumpWidget(_wrap(const MiniPlayerBar(), controller));
    await tester.pump();

    controller.emitNowPlaying(_item());
    controller.emitState(PlayerState.playing(_item(), Duration.zero));
    await tester.pump();
    await tester.pump();

    await tester.tap(find.byKey(const Key('mini_player_next_button')));
    expect(controller.skipNextCalls, 1);
  });

  testWidgets('tapping the bar invokes onExpand', (tester) async {
    var expanded = false;
    await tester.pumpWidget(_wrap(MiniPlayerBar(onExpand: () => expanded = true), controller));
    await tester.pump();

    controller.emitNowPlaying(_item());
    controller.emitState(PlayerState.playing(_item(), Duration.zero));
    await tester.pump();
    await tester.pump();

    await tester.tap(find.text('Song'));
    expect(expanded, isTrue);
  });
}
