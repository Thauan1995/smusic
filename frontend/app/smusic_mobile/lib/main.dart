import 'package:auth_data/auth_data.dart';
import 'package:auth_domain/auth_domain.dart';
import 'package:core_networking/core_networking.dart';
import 'package:core_platform/core_platform.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:library_data/library_data.dart';
import 'package:library_domain/library_domain.dart';
import 'package:player_data/player_data.dart';
import 'package:player_domain/player_domain.dart';
import 'package:smusic_app_shared/smusic_app_shared.dart';

/// Mobile entrypoint. The only concrete-implementation wiring in this file
/// is `core_platform`'s mobile registrations (frontend-flutter.md section
/// 1.3) - everything else (routes, theme, screens) lives in
/// `smusic_app_shared.SmusicApp`, imported unmodified.
///
/// `--dart-define=SMUSIC_API_BASE_URL=...` overrides the backend base URL
/// at build time; defaults to a local dev backend.
const _apiBaseUrl = String.fromEnvironment(
  'SMUSIC_API_BASE_URL',
  defaultValue: 'http://localhost:8080',
);

void main() {
  final tokenStorage = SecureTokenStorage();
  final tokenSource = AuthTokenSourceAdapter(tokenStorage);
  final apiClient = ApiClient(baseUrl: _apiBaseUrl, tokenSource: tokenSource);

  final authRepository = HttpAuthRepository(apiClient);
  // Breaks the ApiClient <-> AuthRepository constructor cycle - see
  // AuthTokenSourceAdapter's doc comment in auth_data.
  tokenSource.attachRepository(authRepository);

  final libraryRepository = HttpLibraryRepository(apiClient);

  final playbackSessionRepository = HttpPlaybackSessionRepository(apiClient);
  final audioEngine = JustAudioNativeEngine();
  final playbackController = JustAudioPlaybackAdapter(
    engine: audioEngine,
    sessionRepository: playbackSessionRepository,
    // TODO(post-Fatia-1): a real per-install device id (persisted, e.g.
    // via device_info_plus + local storage) - a fresh id per cold start is
    // fine for Fatia 1 (cross-device "Connect" sync isn't in scope yet),
    // but would create a new backend playback session on every launch
    // once that scope lands.
    deviceId: 'mobile-${DateTime.now().microsecondsSinceEpoch}',
  );

  runApp(
    ProviderScope(
      overrides: [
        authRepositoryProvider.overrideWithValue(authRepository),
        tokenStorageProvider.overrideWithValue(tokenStorage),
        libraryRepositoryProvider.overrideWithValue(libraryRepository),
        playbackQueueControllerProvider.overrideWithValue(playbackController),
        // OfflineStorage has no consuming provider yet in Fatia 1 (see
        // core_platform.OfflineStorage doc comment) - NoopOfflineStorage
        // here is a placeholder for when FilesystemOfflineStorage lands.
      ],
      child: const SmusicApp(),
    ),
  );
}
