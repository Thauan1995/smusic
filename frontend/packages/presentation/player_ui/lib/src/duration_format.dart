/// Formats a [Duration] as `m:ss` (or `h:mm:ss` past an hour), used by both
/// the mini-player and the expanded player screen.
String formatDuration(Duration duration) {
  final isNegative = duration.isNegative;
  final abs = duration.abs();
  final hours = abs.inHours;
  final minutes = abs.inMinutes.remainder(60);
  final seconds = abs.inSeconds.remainder(60);
  final secondsStr = seconds.toString().padLeft(2, '0');
  final sign = isNegative ? '-' : '';
  if (hours > 0) {
    final minutesStr = minutes.toString().padLeft(2, '0');
    return '$sign$hours:$minutesStr:$secondsStr';
  }
  return '$sign$minutes:$secondsStr';
}
