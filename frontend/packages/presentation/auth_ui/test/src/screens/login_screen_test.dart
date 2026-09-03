import 'package:auth_domain/auth_domain.dart';
import 'package:auth_ui/auth_ui.dart';
import 'package:core_design_system/core_design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../support/fakes.dart';

Widget _wrap(Widget child, {required FakeAuthRepository repository, required FakeTokenStorage storage}) {
  return ProviderScope(
    overrides: [
      authRepositoryProvider.overrideWithValue(repository),
      tokenStorageProvider.overrideWithValue(storage),
    ],
    child: MaterialApp(theme: SmusicTheme.light(), home: child),
  );
}

void main() {
  testWidgets('renders email/password fields and a log in button', (tester) async {
    await tester.pumpWidget(_wrap(
      const LoginScreen(),
      repository: FakeAuthRepository(),
      storage: FakeTokenStorage(),
    ));
    await tester.pump();

    expect(find.byKey(const Key('login_email_field')), findsOneWidget);
    expect(find.byKey(const Key('login_password_field')), findsOneWidget);
    expect(find.text('Log in'), findsOneWidget);
  });

  testWidgets('shows validation errors for empty fields on submit', (tester) async {
    await tester.pumpWidget(_wrap(
      const LoginScreen(),
      repository: FakeAuthRepository(),
      storage: FakeTokenStorage(),
    ));
    await tester.pump();

    await tester.tap(find.text('Log in'));
    await tester.pump();

    expect(find.text('Enter a valid email'), findsOneWidget);
    expect(find.text('Password must be at least 8 characters'), findsOneWidget);
  });

  testWidgets('submits, shows a spinner while loading, then calls onLoggedIn', (tester) async {
    var loggedIn = false;
    final repository = FakeAuthRepository()
      ..logInResult = AuthSession(
        user: const AuthUser(userId: '1', displayName: 'Ana', email: 'a@b.com'),
        tokens: AuthTokens(
          accessToken: 'a',
          refreshToken: 'r',
          accessTokenExpiresAt: DateTime.now().add(const Duration(hours: 1)),
        ),
      );
    await tester.pumpWidget(_wrap(
      LoginScreen(onLoggedIn: () => loggedIn = true),
      repository: repository,
      storage: FakeTokenStorage(),
    ));
    await tester.pump();

    await tester.enterText(find.byKey(const Key('login_email_field')), 'a@b.com');
    await tester.enterText(find.byKey(const Key('login_password_field')), 'password1');
    await tester.tap(find.text('Log in'));
    await tester.pump();

    expect(find.byType(CircularProgressIndicator), findsOneWidget);

    await tester.pumpAndSettle();

    expect(loggedIn, isTrue);
  });

  testWidgets('shows a friendly error message on invalid credentials', (tester) async {
    final repository = FakeAuthRepository()
      ..throwOnLogIn = const AuthException(AuthExceptionKind.invalidCredentials);
    await tester.pumpWidget(_wrap(
      const LoginScreen(),
      repository: repository,
      storage: FakeTokenStorage(),
    ));
    await tester.pump();

    await tester.enterText(find.byKey(const Key('login_email_field')), 'a@b.com');
    await tester.enterText(find.byKey(const Key('login_password_field')), 'wrongpass');
    await tester.tap(find.text('Log in'));
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('login_error_text')), findsOneWidget);
    expect(find.text('Incorrect email or password.'), findsOneWidget);
  });

  testWidgets('tapping the sign-up link invokes onNavigateToSignUp', (tester) async {
    var tapped = false;
    await tester.pumpWidget(_wrap(
      LoginScreen(onNavigateToSignUp: () => tapped = true),
      repository: FakeAuthRepository(),
      storage: FakeTokenStorage(),
    ));
    await tester.pump();

    await tester.tap(find.text("Don't have an account? Sign up"));
    expect(tapped, isTrue);
  });
}
