import 'package:player_domain/player_domain.dart';
import 'package:test/test.dart';

QueueItem _item() => QueueItem(
      trackId: 't1',
      title: 'Song',
      artistName: 'Artist',
      durationMs: 200000,
    );

void main() {
  test('PlayerState.idle() creates a PlayerIdle', () {
    expect(const PlayerState.idle(), isA<PlayerIdle>());
  });

  test('PlayerIdle instances are equal to each other', () {
    expect(PlayerIdle(), PlayerIdle());
    expect(PlayerIdle().hashCode, PlayerIdle().hashCode);
  });

  test('PlayerState.buffering() creates a PlayerBuffering with current item', () {
    final state = PlayerState.buffering(_item());
    expect(state, isA<PlayerBuffering>());
    expect((state as PlayerBuffering).current, _item());
  });

  test('PlayerBuffering equality compares current', () {
    expect(PlayerBuffering(_item()), PlayerBuffering(_item()));
    expect(PlayerBuffering(_item()).hashCode, PlayerBuffering(_item()).hashCode);
    final other = QueueItem(trackId: 't2', title: 'X', artistName: 'Y', durationMs: 1);
    expect(PlayerBuffering(_item()) == PlayerBuffering(other), isFalse);
  });

  test('PlayerState.playing() creates a PlayerPlaying with position', () {
    final state = PlayerState.playing(_item(), const Duration(seconds: 5));
    expect(state, isA<PlayerPlaying>());
    final playing = state as PlayerPlaying;
    expect(playing.current, _item());
    expect(playing.position, const Duration(seconds: 5));
  });

  test('PlayerPlaying equality compares current and position', () {
    final a = PlayerPlaying(_item(), const Duration(seconds: 1));
    final b = PlayerPlaying(_item(), const Duration(seconds: 1));
    expect(a, b);
    expect(a.hashCode, b.hashCode);
    final c = PlayerPlaying(_item(), const Duration(seconds: 2));
    expect(a == c, isFalse);
  });

  test('PlayerState.paused() creates a PlayerPaused with position', () {
    final state = PlayerState.paused(_item(), const Duration(seconds: 5));
    expect(state, isA<PlayerPaused>());
    final paused = state as PlayerPaused;
    expect(paused.current, _item());
    expect(paused.position, const Duration(seconds: 5));
  });

  test('PlayerPaused equality compares current and position', () {
    final a = PlayerPaused(_item(), const Duration(seconds: 1));
    final b = PlayerPaused(_item(), const Duration(seconds: 1));
    expect(a, b);
    expect(a.hashCode, b.hashCode);
    final c = PlayerPaused(_item(), const Duration(seconds: 9));
    expect(a == c, isFalse);
  });

  test('PlayerState.error() creates a PlayerErrorState', () {
    final error = PlayerError('boom');
    final state = PlayerState.error(error);
    expect(state, isA<PlayerErrorState>());
    expect((state as PlayerErrorState).error, error);
  });

  test('PlayerErrorState equality compares error', () {
    final a = PlayerErrorState(PlayerError('x'));
    final b = PlayerErrorState(PlayerError('x'));
    expect(a, b);
    expect(a.hashCode, b.hashCode);
    final c = PlayerErrorState(PlayerError('y'));
    expect(a == c, isFalse);
  });

  test('exhaustive switch pattern matches every variant', () {
    String describe(PlayerState state) => switch (state) {
          PlayerIdle() => 'idle',
          PlayerBuffering() => 'buffering',
          PlayerPlaying() => 'playing',
          PlayerPaused() => 'paused',
          PlayerErrorState() => 'error',
        };

    expect(describe(const PlayerState.idle()), 'idle');
    expect(describe(PlayerState.buffering(_item())), 'buffering');
    expect(
      describe(PlayerState.playing(_item(), Duration.zero)),
      'playing',
    );
    expect(
      describe(PlayerState.paused(_item(), Duration.zero)),
      'paused',
    );
    expect(describe(PlayerState.error(PlayerError('x'))), 'error');
  });
}
