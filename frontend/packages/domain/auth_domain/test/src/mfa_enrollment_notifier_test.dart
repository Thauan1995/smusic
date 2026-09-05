import 'package:auth_domain/auth_domain.dart';
import 'package:riverpod/riverpod.dart';
import 'package:test/test.dart';

import '../support/fake_auth_repository.dart';

void main() {
  late FakeAuthRepository repository;
  late ProviderContainer container;

  setUp(() {
    repository = FakeAuthRepository();
    container = ProviderContainer(
      overrides: [authRepositoryProvider.overrideWithValue(repository)],
    );
    addTearDown(container.dispose);
  });

  test('build starts with no enrollment', () {
    expect(container.read(mfaEnrollmentProvider).value, isNull);
  });

  test('enroll populates state from the repository', () async {
    repository.enrollMfaResult = const MfaEnrollment(
      secret: 'JBSWY3DPEHPK3PXP',
      otpauthUrl: 'otpauth://totp/smusic:a@b.com?secret=JBSWY3DPEHPK3PXP&issuer=smusic',
    );

    await container.read(mfaEnrollmentProvider.notifier).enroll();

    final state = container.read(mfaEnrollmentProvider);
    expect(state.value?.secret, 'JBSWY3DPEHPK3PXP');
    expect(repository.enrollMfaCalls, 1);
  });

  test('enroll surfaces a repository failure as AsyncError', () async {
    repository.throwOnEnrollMfa = const AuthException(AuthExceptionKind.network);

    await container.read(mfaEnrollmentProvider.notifier).enroll();

    expect(container.read(mfaEnrollmentProvider).hasError, isTrue);
  });

  test('verify forwards the code to the repository and completes on success', () async {
    await container.read(mfaEnrollmentProvider.notifier).verify('123456');
    expect(repository.lastVerifyMfaCode, '123456');
  });

  test('verify rethrows AuthException on an invalid code, without touching state', () async {
    repository.enrollMfaResult = const MfaEnrollment(secret: 's', otpauthUrl: 'o');
    await container.read(mfaEnrollmentProvider.notifier).enroll();

    repository.throwOnVerifyMfa =
        const AuthException(AuthExceptionKind.invalidMfaCode);

    await expectLater(
      () => container.read(mfaEnrollmentProvider.notifier).verify('000000'),
      throwsA(
        isA<AuthException>().having((e) => e.kind, 'kind', AuthExceptionKind.invalidMfaCode),
      ),
    );
    // The already-fetched enrollment must survive a failed verify attempt.
    expect(container.read(mfaEnrollmentProvider).value?.secret, 's');
  });
}
