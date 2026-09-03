import 'package:flutter_test/flutter_test.dart';
import 'package:player_ui/player_ui.dart';

void main() {
  test('formats seconds under a minute', () {
    expect(formatDuration(const Duration(seconds: 5)), '0:05');
  });

  test('formats minutes and seconds', () {
    expect(formatDuration(const Duration(minutes: 3, seconds: 27)), '3:27');
  });

  test('formats hours when present', () {
    expect(formatDuration(const Duration(hours: 1, minutes: 2, seconds: 3)), '1:02:03');
  });

  test('formats a negative duration with a leading minus sign', () {
    expect(formatDuration(const Duration(seconds: -5)), '-0:05');
  });
}
