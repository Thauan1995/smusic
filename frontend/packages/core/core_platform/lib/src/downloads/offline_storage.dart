/// Interface only for Fatia 1; crossfade/offline downloads are explicitly
/// out of scope (see docs/architecture/frontend-flutter.md section 2.5 and
/// the implementation task's stated deviations). `NoopOfflineStorage` is
/// provided so `app/*` has *something* concrete to wire up today without
/// pretending downloads work - every method reports "not supported".
///
/// TODO(post-Fatia-1): `FilesystemOfflineStorage` (path_provider + an
/// Isar/SQLite index, per frontend-flutter.md section 2.5) for mobile.
library;

/// Opaque track identifier, matching `player_domain`'s `TrackId`.
typedef TrackId = String;

abstract interface class OfflineStorage {
  Future<void> saveTrack(TrackId id, Stream<List<int>> bytes);

  /// Returns the local file path if [id] is downloaded, or `null` if not
  /// downloaded (or if this platform doesn't support downloads at all - see
  /// `NoopOfflineStorage`).
  Future<String?> getLocalFilePath(TrackId id);

  Future<void> deleteTrack(TrackId id);

  Stream<double> downloadProgress(TrackId id);
}

/// Always-unsupported implementation. Used on Web (frontend-flutter.md
/// section 1.3 table) and, for Fatia 1, on mobile too until the real
/// filesystem-backed implementation lands (see TODO above) - `player_ui`
/// reacts to "unsupported" the same way on both platforms today, so no
/// feature branches on platform.
class NoopOfflineStorage implements OfflineStorage {
  const NoopOfflineStorage();

  @override
  Future<void> saveTrack(TrackId id, Stream<List<int>> bytes) async {
    throw UnsupportedError('Offline downloads are not supported yet.');
  }

  @override
  Future<String?> getLocalFilePath(TrackId id) async => null;

  @override
  Future<void> deleteTrack(TrackId id) async {}

  @override
  Stream<double> downloadProgress(TrackId id) => const Stream.empty();
}
