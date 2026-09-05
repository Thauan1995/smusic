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
    child: MaterialApp(theme: SmusicTheme.light(), home: child),
  );
}

void main() {
  late FakePlaybackQueueController controller;

  setUp(() => controller = FakePlaybackQueueController());
  tearDown(() => controller.dispose());

  testWidgets('shows a spinner before the first state arrives', (tester) async {
    await tester.pumpWidget(_wrap(const PlayerScreen(), controller));
    // No pump() beyond the first frame - stream provider starts in loading.
    expect(find.byType(CircularProgressIndicator), findsOneWidget);
  });

  testWidgets('shows an idle empty state', (tester) async {
    await tester.pumpWidget(_wrap(const PlayerScreen(), controller));
    controller.emitState(const PlayerState.idle());
    await tester.pump();

    expect(find.text('Nothing is playing right now.'), findsOneWidget);
  });

  testWidgets('shows a generic error state when the underlying stream itself fails', (tester) async {
    await tester.pumpWidget(_wrap(const PlayerScreen(), controller));
    controller.emitStreamError(Exception('boom'));
    await tester.pump();

    expect(find.text('Playback error. Please try again.'), findsOneWidget);
  });

  testWidgets('shows an error state', (tester) async {
    await tester.pumpWidget(_wrap(const PlayerScreen(), controller));
    controller.emitState(const PlayerState.error(PlayerError('Codec error')));
    await tester.pump();

    expect(find.text('Codec error'), findsOneWidget);
  });

  testWidgets('shows track info and a pause button while playing', (tester) async {
    await tester.pumpWidget(_wrap(const PlayerScreen(), controller));
    controller.emitState(PlayerState.playing(_item(), const Duration(seconds: 30)));
    await tester.pump();

    expect(find.text('Song'), findsOneWidget);
    expect(find.text('Artist'), findsOneWidget);
    expect(find.text('0:30'), findsOneWidget);
    expect(find.byIcon(Icons.pause_circle_filled), findsOneWidget);
  });

  testWidgets('shows a play button while paused', (tester) async {
    await tester.pumpWidget(_wrap(const PlayerScreen(), controller));
    controller.emitState(PlayerState.paused(_item(), const Duration(seconds: 10)));
    await tester.pump();

    // Outlined: paused is the "available action" state, not in-progress -
    // see .vibeflow/specs/icon-system-consistency.md.
    expect(find.byIcon(Icons.play_circle_outline), findsOneWidget);
  });

  testWidgets('shows a spinner and disables controls while buffering', (tester) async {
    await tester.pumpWidget(_wrap(const PlayerScreen(), controller));
    controller.emitState(PlayerState.buffering(_item()));
    await tester.pump();

    expect(find.byKey(const Key('player_play_pause_button')), findsNothing);
    final nextButton = tester.widget<IconButton>(find.byKey(const Key('player_next_button')));
    expect(nextButton.onPressed, isNull);
  });

  testWidgets('pause/resume buttons call the controller', (tester) async {
    await tester.pumpWidget(_wrap(const PlayerScreen(), controller));
    controller.emitState(PlayerState.playing(_item(), Duration.zero));
    await tester.pump();

    await tester.tap(find.byKey(const Key('player_play_pause_button')));
    expect(controller.pauseCalls, 1);

    controller.emitState(PlayerState.paused(_item(), Duration.zero));
    await tester.pump();
    await tester.tap(find.byKey(const Key('player_play_pause_button')));
    expect(controller.resumeCalls, 1);
  });

  testWidgets('next/previous buttons call the controller', (tester) async {
    await tester.pumpWidget(_wrap(const PlayerScreen(), controller));
    controller.emitState(PlayerState.playing(_item(), Duration.zero));
    await tester.pump();

    await tester.tap(find.byKey(const Key('player_next_button')));
    await tester.tap(find.byKey(const Key('player_previous_button')));

    expect(controller.skipNextCalls, 1);
    expect(controller.skipPreviousCalls, 1);
  });

  testWidgets('dragging the seek slider calls seekTo', (tester) async {
    await tester.pumpWidget(_wrap(const PlayerScreen(), controller));
    controller.emitState(PlayerState.playing(_item(), const Duration(seconds: 50)));
    await tester.pump();

    await tester.drag(find.byKey(const Key('player_seek_slider')), const Offset(-50, 0));
    expect(controller.lastSeekPosition, isNotNull);
  });

  testWidgets('close button invokes onClose', (tester) async {
    var closed = false;
    await tester.pumpWidget(_wrap(PlayerScreen(onClose: () => closed = true), controller));
    controller.emitState(const PlayerState.idle());
    await tester.pump();

    await tester.tap(find.byIcon(Icons.expand_more));
    expect(closed, isTrue);
  });

  testWidgets('handles a zero-duration track without dividing by zero', (tester) async {
    const zeroItem = QueueItem(trackId: 't2', title: 'Live', artistName: 'Radio', durationMs: 0);
    await tester.pumpWidget(_wrap(const PlayerScreen(), controller));
    controller.emitState(const PlayerState.playing(zeroItem, Duration.zero));
    await tester.pump();

    expect(find.text('Live'), findsOneWidget);
  });
}
