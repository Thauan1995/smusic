import 'package:auth_domain/auth_domain.dart';
import 'package:auth_ui/auth_ui.dart';
import 'package:core_design_system/core_design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../support/fakes.dart';

Widget _wrap(Widget child, {required FakeAuthRepository repository}) {
  return ProviderScope(
    overrides: [authRepositoryProvider.overrideWithValue(repository)],
    child: MaterialApp(theme: SmusicTheme.light(), home: child),
  );
}

void main() {
  testWidgets('fetches enrollment on load and shows the secret', (tester) async {
    final repository = FakeAuthRepository()
      ..enrollMfaResult = const MfaEnrollment(
        secret: 'JBSWY3DPEHPK3PXP',
        otpauthUrl: 'otpauth://totp/smusic:a@b.com?secret=JBSWY3DPEHPK3PXP&issuer=smusic',
      );

    await tester.pumpWidget(_wrap(const MfaSetupScreen(), repository: repository));
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('mfa_secret_text')), findsOneWidget);
    expect(find.text('JBSWY3DPEHPK3PXP'), findsOneWidget);
  });

  testWidgets('validates the code field on submit', (tester) async {
    final repository = FakeAuthRepository()
      ..enrollMfaResult = const MfaEnrollment(secret: 's', otpauthUrl: 'o');

    await tester.pumpWidget(_wrap(const MfaSetupScreen(), repository: repository));
    await tester.pumpAndSettle();

    await tester.tap(find.text('Verify and enable'));
    await tester.pump();

    expect(find.text('Enter the 6-digit code from your authenticator app'), findsOneWidget);
  });

  testWidgets('submits the code and calls onVerified on success', (tester) async {
    var verified = false;
    final repository = FakeAuthRepository()
      ..enrollMfaResult = const MfaEnrollment(secret: 's', otpauthUrl: 'o');

    await tester.pumpWidget(_wrap(
      MfaSetupScreen(onVerified: () => verified = true),
      repository: repository,
    ));
    await tester.pumpAndSettle();

    await tester.enterText(find.byKey(const Key('mfa_code_field')), '123456');
    await tester.tap(find.text('Verify and enable'));
    await tester.pump();

    expect(find.byType(CircularProgressIndicator), findsOneWidget);

    await tester.pumpAndSettle();

    expect(repository.lastVerifyMfaCode, '123456');
    expect(verified, isTrue);
  });

  testWidgets('shows a friendly error message on an invalid code', (tester) async {
    final repository = FakeAuthRepository()
      ..enrollMfaResult = const MfaEnrollment(secret: 's', otpauthUrl: 'o')
      ..throwOnVerifyMfa = const AuthException(AuthExceptionKind.invalidMfaCode);

    await tester.pumpWidget(_wrap(const MfaSetupScreen(), repository: repository));
    await tester.pumpAndSettle();

    await tester.enterText(find.byKey(const Key('mfa_code_field')), '000000');
    await tester.tap(find.text('Verify and enable'));
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('mfa_verify_error_text')), findsOneWidget);
    expect(find.text('Invalid or expired code. Please try again.'), findsOneWidget);
  });
}
