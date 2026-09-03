// Real-browser, real-backend E2E validation of the web flow, per
// docs/architecture/frontend-flutter.md section 5.2 ("Integration/E2E ...
// flutter drive/chromedriver para Web ... fluxos críticos ponta a ponta:
// login -> tocar uma faixa -> ...") and frontend/README.md's "Testes E2E
// (Web, browser real)" section for how to run this file.
//
// This test deliberately does NOT call any auth_data/library_data/
// player_data repository directly, and does NOT override any repository
// provider with a fake. It builds the exact same widget tree
// app/smusic_web/lib/main.dart builds (same ApiClient, same Dio, same
// HttpAuthRepository/HttpLibraryRepository/HttpPlaybackSessionRepository,
// same JustAudioNativeEngine using just_audio_web) and only ever touches
// it through widget finders (find.text/find.byKey) and tester.tap/
// tester.enterText - the point is to exercise the real network stack
// (Dio -> HTTP -> the browser's CORS enforcement -> the real Go backend)
// exactly the way an actual user driving the app in Chrome would,
// something no unit/widget test with fakes and no `curl`-only backend
// verification can prove.
//
// Requires, at `flutter drive` time (see frontend/README.md):
// - A real backend reachable at --dart-define=SMUSIC_API_BASE_URL (default
//   http://localhost:8080), with CORS_ALLOWED_ORIGINS covering whatever
//   origin `flutter drive -d chrome --web-port=<port>` serves the app from.
// - The backend's catalog seeded with a track titled "E2E Test Track"
//   (see backend/README.md's CORS section / this repo's E2E validation
//   notes for the exact SQL used) so the search step has something real to
//   find.
import 'package:auth_data/auth_data.dart';
import 'package:auth_domain/auth_domain.dart';
import 'package:core_networking/core_networking.dart';
import 'package:core_platform/core_platform.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:integration_test/integration_test.dart';
import 'package:library_data/library_data.dart';
import 'package:library_domain/library_domain.dart';
import 'package:player_data/player_data.dart';
import 'package:player_domain/player_domain.dart';
import 'package:smusic_app_shared/smusic_app_shared.dart';

const _apiBaseUrl = String.fromEnvironment(
  'SMUSIC_API_BASE_URL',
  defaultValue: 'http://localhost:8080',
);

/// The track seeded directly via SQL against the real backend before this
/// test runs (see the SQL in this repo's E2E validation notes /
/// frontend/README.md) - not created by this test itself, since catalog
/// write endpoints are a separate concern from what this test validates.
const _seededTrackTitle = 'E2E Test Track';

/// Builds a widget tree wired identically to
/// `app/smusic_web/lib/main.dart`'s `main()` - same [ApiClient], same real
/// repositories, same [JustAudioNativeEngine] (which resolves to
/// `just_audio_web`'s `MediaElement` backend on this platform) - pointed at
/// [_apiBaseUrl]. Returns the [TokenStorage] alongside the widget so a test
/// can [TokenStorage.clear] it to simulate a fresh, signed-out browser
/// session between phases without tearing down the whole test binding.
({Widget widget, TokenStorage tokenStorage}) _buildRealApp() {
  final tokenStorage = SecureTokenStorage();
  final tokenSource = AuthTokenSourceAdapter(tokenStorage);
  final apiClient = ApiClient(baseUrl: _apiBaseUrl, tokenSource: tokenSource);

  final authRepository = HttpAuthRepository(apiClient);
  // Breaks the ApiClient <-> AuthRepository constructor cycle - see
  // AuthTokenSourceAdapter's doc comment in auth_data (same pattern as
  // main.dart).
  tokenSource.attachRepository(authRepository);

  final libraryRepository = HttpLibraryRepository(apiClient);

  final playbackSessionRepository = HttpPlaybackSessionRepository(apiClient);
  final audioEngine = JustAudioNativeEngine();
  final playbackController = JustAudioPlaybackAdapter(
    engine: audioEngine,
    sessionRepository: playbackSessionRepository,
    deviceId: 'e2e-web-${DateTime.now().microsecondsSinceEpoch}',
  );

  final widget = ProviderScope(
    overrides: [
      authRepositoryProvider.overrideWithValue(authRepository),
      tokenStorageProvider.overrideWithValue(tokenStorage),
      libraryRepositoryProvider.overrideWithValue(libraryRepository),
      playbackQueueControllerProvider.overrideWithValue(playbackController),
    ],
    child: const SmusicApp(),
  );

  return (widget: widget, tokenStorage: tokenStorage);
}

/// `pumpAndSettle` with a bounded timeout (default is 10 minutes) so a
/// genuine regression - e.g. CORS silently blocking a request, leaving a
/// loading spinner spinning forever - fails this test in seconds instead of
/// hanging the whole `flutter drive` run.
Future<void> _settle(WidgetTester tester) => tester.pumpAndSettle(
      const Duration(milliseconds: 100),
      EnginePhase.sendSemanticsUpdate,
      const Duration(seconds: 45),
    );

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();

  final uniqueSuffix = DateTime.now().millisecondsSinceEpoch;
  final testEmail = 'e2e_web_$uniqueSuffix@smusic.test';
  const testPassword = 'SuperSecret123';
  const testDisplayName = 'E2E Web User';

  testWidgets(
    'sign up creates a new account against the real backend and reaches the library',
    (tester) async {
      final app = _buildRealApp();
      await app.tokenStorage.clear(); // start as a fresh, signed-out browser
      await tester.pumpWidget(app.widget);
      await _settle(tester);

      // Unauthenticated -> go_router's redirect sends us to /login first.
      expect(find.byKey(const Key('login_email_field')), findsOneWidget);

      await tester.tap(find.text("Don't have an account? Sign up"));
      await _settle(tester);
      expect(find.byKey(const Key('signup_display_name_field')), findsOneWidget);

      await tester.enterText(
        find.byKey(const Key('signup_display_name_field')),
        testDisplayName,
      );
      await tester.enterText(find.byKey(const Key('signup_email_field')), testEmail);
      await tester.enterText(
        find.byKey(const Key('signup_password_field')),
        testPassword,
      );
      await _settle(tester);

      // Real network round trip: Dio -> HTTP -> browser CORS preflight/
      // actual request -> the real Go backend -> Postgres -> back.
      await tester.tap(find.text('Sign up'));
      await _settle(tester);

      expect(
        find.byKey(const Key('signup_error_text')),
        findsNothing,
        reason: 'signup against the real backend should not surface an error '
            '(CORS-blocked and other network failures would show up here)',
      );
      expect(find.text('Your Library'), findsOneWidget);
    },
  );

  testWidgets(
    'logging in with the just-created account reaches the library, finds the '
    'real seeded track via search, and starts real playback',
    (tester) async {
      final app = _buildRealApp();
      // Simulate a fresh browser tab/session: clear whatever the signup
      // test above persisted, even though it ran in the same browser
      // process, so /login (not an auto-restored session) is what we hit.
      await app.tokenStorage.clear();
      await tester.pumpWidget(app.widget);
      await _settle(tester);

      expect(find.byKey(const Key('login_email_field')), findsOneWidget);
      await tester.enterText(find.byKey(const Key('login_email_field')), testEmail);
      await tester.enterText(
        find.byKey(const Key('login_password_field')),
        testPassword,
      );
      await _settle(tester);

      // Real network round trip against POST /v1/auth/login.
      await tester.tap(find.text('Log in'));
      await _settle(tester);

      expect(
        find.byKey(const Key('login_error_text')),
        findsNothing,
        reason: 'login against the real backend should not surface an error',
      );
      expect(find.text('Your Library'), findsOneWidget);

      // --- Navigate to the real catalog/search screen ---
      await tester.tap(find.byIcon(Icons.search));
      await _settle(tester);
      expect(find.byKey(const Key('search_field')), findsOneWidget);

      // Debounced (300ms) then hits the real GET /v1/catalog/search.
      await tester.enterText(find.byKey(const Key('search_field')), 'E2E');
      await _settle(tester);

      expect(
        find.text(_seededTrackTitle),
        findsOneWidget,
        reason:
            'the track seeded via SQL directly against the real backend should '
            'come back through a real GET /v1/catalog/search round trip',
      );

      // --- Start playback of the real track via a real widget tap ---
      await tester.tap(find.text(_seededTrackTitle));
      await _settle(tester);

      // Reached the expanded "now playing" screen (real POST
      // /v1/playback/sessions -> real POST .../play -> real signed
      // stream_url -> the browser's <audio> element fetching real bytes
      // cross-origin from the backend's /media handler).
      expect(find.text('Now Playing'), findsOneWidget);
      expect(find.text(_seededTrackTitle), findsOneWidget);
      expect(find.byKey(const Key('player_play_pause_button')), findsOneWidget);
      expect(
        find.text('Playback error. Please try again.'),
        findsNothing,
        reason: 'a CORS/network failure resolving the stream would surface here',
      );
      expect(find.text('Nothing is playing right now.'), findsNothing);

      // Exercise pause/resume through the same real widget control a user
      // would use, proving the interaction round-trips through the real
      // PlaybackQueueController -> real backend session state too.
      await tester.tap(find.byKey(const Key('player_play_pause_button')));
      await _settle(tester);
      await tester.tap(find.byKey(const Key('player_play_pause_button')));
      await _settle(tester);
    },
  );
}
