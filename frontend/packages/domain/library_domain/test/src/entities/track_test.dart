import 'package:library_domain/library_domain.dart';
import 'package:test/test.dart';

Track _track({String id = '1'}) => Track(
      id: id,
      title: 'Song',
      artistName: 'Artist',
      albumName: 'Album',
      durationMs: 210000,
    );

void main() {
  test('duration converts durationMs to a Duration', () {
    expect(_track().duration, const Duration(milliseconds: 210000));
  });

  test('equal when all fields match (non-identical instances)', () {
    expect(_track(), _track());
    expect(_track().hashCode, _track().hashCode);
  });

  test('not equal when a field differs', () {
    expect(_track() == _track(id: '2'), isFalse);
  });
}
