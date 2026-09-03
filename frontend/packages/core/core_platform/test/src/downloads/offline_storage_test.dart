import 'package:core_platform/core_platform.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  group('NoopOfflineStorage', () {
    const storage = NoopOfflineStorage();

    test('getLocalFilePath always returns null', () async {
      expect(await storage.getLocalFilePath('track-1'), isNull);
    });

    test('saveTrack throws UnsupportedError', () {
      expect(
        () => storage.saveTrack('track-1', const Stream.empty()),
        throwsUnsupportedError,
      );
    });

    test('deleteTrack completes without throwing', () async {
      await storage.deleteTrack('track-1');
    });

    test('downloadProgress is an empty stream', () async {
      expect(await storage.downloadProgress('track-1').toList(), isEmpty);
    });
  });
}
