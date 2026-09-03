import 'dart:async';

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
import 'package:social_proximity_data/social_proximity_data.dart';
import 'package:social_proximity_domain/social_proximity_domain.dart';

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

  const locationProvider = GeolocatorLocationProvider();
  final proximityPrivacySettingsRepository = HttpProximityPrivacySettingsRepository(apiClient);
  final proximityFeedRepository = WebSocketProximityFeedRepository(
    socketClient: buildPresenceSocketClient(apiBaseUrl: _apiBaseUrl, tokenSource: tokenSource),
    locationProvider: locationProvider,
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
        locationProviderProvider.overrideWithValue(locationProvider),
        proximityPrivacySettingsRepositoryProvider
            .overrideWithValue(proximityPrivacySettingsRepository),
        proximityFeedRepositoryProvider.overrideWithValue(proximityFeedRepository),
      ],
      child: const SmusicApp(),
    ),
  );
}

/// Builds the `/v1/presence/connect` `ReconnectingWebSocketClient`
/// (frontend-flutter.md section 4.1/4.3), shared verbatim by
/// `smusic_mobile`/`smusic_web`'s `main.dart` (same "source-identical
/// composition root" pattern as every other wiring block in this file).
///
/// **Auth on the WS handshake - deviation flagged for the backend
/// specialist to confirm**: `backend/internal/presence/ws/handler.go`'s
/// `bearerToken` helper accepts the access token via either the
/// `Authorization` header (native-only - browsers cannot set custom headers
/// on a WebSocket handshake) or an `access_token` query parameter; this
/// composition root always uses the query parameter so the exact same code
/// runs on mobile and Web (frontend-flutter.md section 1.3's 100%-reuse
/// mandate applies to the composition root too, not just features).
/// `ReconnectingWebSocketClient.uriBuilder` is intentionally synchronous
/// (`Uri Function()`, not `Future<Uri> Function()` - see that class's doc
/// comment on why it stays protocol/async-agnostic), while
/// `AuthTokenSource.currentAccessToken()` is async, so there is no way to
/// fetch a guaranteed-fresh token exactly at each (re)connect attempt
/// without changing that shared, already-tested class. This function
/// instead keeps a small in-memory cache refreshed every 30s (comfortably
/// under security.md section 2's 10-15 min access-token TTL, so the token
/// used to open a socket is never more than 30s stale) - a deliberate,
/// bounded-staleness trade-off scoped to this composition root, not a
/// change to any tested package.
@visibleForTesting
ReconnectingWebSocketClient buildPresenceSocketClient({
  required String apiBaseUrl,
  required AuthTokenSource tokenSource,
}) {
  String? cachedToken;
  Future<void> refreshCachedToken() async {
    cachedToken = await tokenSource.currentAccessToken();
  }

  unawaited(refreshCachedToken());
  Timer.periodic(const Duration(seconds: 30), (_) => unawaited(refreshCachedToken()));

  return ReconnectingWebSocketClient(
    uriBuilder: () => buildPresenceUri(apiBaseUrl: apiBaseUrl, accessToken: cachedToken),
  );
}

/// Pure URI-construction half of [buildPresenceSocketClient], split out so
/// it is directly unit-testable without a live `Timer`/`AuthTokenSource`
/// (`http(s)` -> `ws(s)` scheme mapping, path, and the optional
/// `access_token` query parameter - see [buildPresenceSocketClient]'s doc
/// comment for the full rationale of why the token is a plain parameter
/// here rather than fetched inline).
@visibleForTesting
Uri buildPresenceUri({required String apiBaseUrl, String? accessToken}) {
  final httpUri = Uri.parse(apiBaseUrl);
  final wsScheme = httpUri.scheme == 'https' ? 'wss' : 'ws';
  return httpUri.replace(
    scheme: wsScheme,
    path: '/v1/presence/connect',
    queryParameters: {
      if (accessToken != null) 'access_token': accessToken,
    },
  );
}
