import 'package:player_domain/player_domain.dart';
import 'package:test/test.dart';

QueueItem _item({String trackId = 't1'}) => QueueItem(
      trackId: trackId,
      title: 'Song',
      artistName: 'Artist',
      durationMs: 200000,
    );

void main() {
  test('duration converts durationMs to Duration', () {
    expect(_item().duration, const Duration(milliseconds: 200000));
  });

  test('equal when all fields match (non-identical instances)', () {
    expect(_item(), _item());
    expect(_item().hashCode, _item().hashCode);
  });

  test('not equal when trackId differs', () {
    expect(_item() == _item(trackId: 't2'), isFalse);
  });
}
