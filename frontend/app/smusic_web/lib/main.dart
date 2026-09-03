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

/// Web entrypoint. See smusic_mobile/lib/main.dart's doc comment - the
/// wiring below is deliberately source-identical to it in Fatia 1
/// (frontend-flutter.md section 1.3's "100% reuse by construction" holds
/// even at the composition root when the concrete implementations happen
/// to be the same for both platforms).
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
    // TODO(post-Fatia-1): a real per-browser device id (persisted, e.g.
    // via localStorage) - see the equivalent TODO in smusic_mobile.
    deviceId: 'web-${DateTime.now().microsecondsSinceEpoch}',
  );

  runApp(
    ProviderScope(
      overrides: [
        authRepositoryProvider.overrideWithValue(authRepository),
        tokenStorageProvider.overrideWithValue(tokenStorage),
        libraryRepositoryProvider.overrideWithValue(libraryRepository),
        playbackQueueControllerProvider.overrideWithValue(playbackController),
        // OfflineStorage is mobile-only by nature (frontend-flutter.md
        // section 1.3) and has no consuming provider yet in Fatia 1 - see
        // core_platform.NoopOfflineStorage doc comment.
      ],
      child: const SmusicApp(),
    ),
  );
}
