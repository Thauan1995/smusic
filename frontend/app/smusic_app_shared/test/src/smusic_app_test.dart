import 'package:auth_domain/auth_domain.dart';
import 'package:flutter/widgets.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:library_domain/library_domain.dart';
import 'package:player_domain/player_domain.dart';
import 'package:smusic_app_shared/smusic_app_shared.dart';

import '../support/fakes.dart';

void main() {
  testWidgets('renders LoginScreen when signed out', (tester) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          authRepositoryProvider.overrideWithValue(FakeAuthRepository()),
          tokenStorageProvider.overrideWithValue(FakeTokenStorage()),
          libraryRepositoryProvider.overrideWithValue(FakeLibraryRepository()),
          playbackQueueControllerProvider.overrideWithValue(FakePlaybackQueueController()),
        ],
        // Not const: a const SmusicApp() invocation is canonicalized at
        // compile time and never shows as "hit" by line coverage tooling.
        // ignore: prefer_const_constructors
        child: SmusicApp(),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('smusic'), findsOneWidget);
    expect(find.byKey(const Key('login_email_field')), findsOneWidget);
  });
}
