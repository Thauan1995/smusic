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
  testWidgets('renders name/email/password fields and a sign up button', (tester) async {
    await tester.pumpWidget(_wrap(
      const SignUpScreen(),
      repository: FakeAuthRepository(),
      storage: FakeTokenStorage(),
    ));
    await tester.pump();

    expect(find.byKey(const Key('signup_display_name_field')), findsOneWidget);
    expect(find.byKey(const Key('signup_email_field')), findsOneWidget);
    expect(find.byKey(const Key('signup_password_field')), findsOneWidget);
    expect(find.text('Sign up'), findsOneWidget);
  });

  testWidgets('shows validation errors for empty fields on submit', (tester) async {
    await tester.pumpWidget(_wrap(
      const SignUpScreen(),
      repository: FakeAuthRepository(),
      storage: FakeTokenStorage(),
    ));
    await tester.pump();

    await tester.tap(find.text('Sign up'));
    await tester.pump();

    expect(find.text('Enter a name'), findsOneWidget);
    expect(find.text('Enter a valid email'), findsOneWidget);
    expect(find.text('Password must be at least 8 characters'), findsOneWidget);
  });

  testWidgets('submits, shows a spinner while loading, then calls onSignedUp', (tester) async {
    var signedUp = false;
    final repository = FakeAuthRepository()
      ..signUpResult = AuthSession(
        user: const AuthUser(userId: '1', displayName: 'Ana', email: 'a@b.com'),
        tokens: AuthTokens(
          accessToken: 'a',
          refreshToken: 'r',
          accessTokenExpiresAt: DateTime.now().add(const Duration(hours: 1)),
        ),
      );
    await tester.pumpWidget(_wrap(
      SignUpScreen(onSignedUp: () => signedUp = true),
      repository: repository,
      storage: FakeTokenStorage(),
    ));
    await tester.pump();

    await tester.enterText(find.byKey(const Key('signup_display_name_field')), 'Ana');
    await tester.enterText(find.byKey(const Key('signup_email_field')), 'a@b.com');
    await tester.enterText(find.byKey(const Key('signup_password_field')), 'password1');
    await tester.tap(find.text('Sign up'));
    await tester.pump();

    expect(find.byType(CircularProgressIndicator), findsOneWidget);

    await tester.pumpAndSettle();

    expect(signedUp, isTrue);
  });

  testWidgets('shows a friendly error message when the email is taken', (tester) async {
    final repository = FakeAuthRepository()
      ..throwOnSignUp = const AuthException(AuthExceptionKind.emailAlreadyInUse);
    await tester.pumpWidget(_wrap(
      const SignUpScreen(),
      repository: repository,
      storage: FakeTokenStorage(),
    ));
    await tester.pump();

    await tester.enterText(find.byKey(const Key('signup_display_name_field')), 'Ana');
    await tester.enterText(find.byKey(const Key('signup_email_field')), 'a@b.com');
    await tester.enterText(find.byKey(const Key('signup_password_field')), 'password1');
    await tester.tap(find.text('Sign up'));
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('signup_error_text')), findsOneWidget);
    expect(find.text('That email is already in use.'), findsOneWidget);
  });

  testWidgets('tapping the login link invokes onNavigateToLogin', (tester) async {
    var tapped = false;
    await tester.pumpWidget(_wrap(
      SignUpScreen(onNavigateToLogin: () => tapped = true),
      repository: FakeAuthRepository(),
      storage: FakeTokenStorage(),
    ));
    await tester.pump();

    await tester.tap(find.text('Already have an account? Log in'));
    expect(tapped, isTrue);
  });
}
