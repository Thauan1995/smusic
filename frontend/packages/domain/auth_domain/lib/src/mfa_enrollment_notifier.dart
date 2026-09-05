import 'package:riverpod/riverpod.dart';

import 'auth_session_notifier.dart';
import 'entities/mfa_enrollment.dart';

/// Drives the MFA enrollment screen (security.md §2's TOTP step-up,
/// required before `ProximityPrivacySettingsNotifier.enableFeature` can
/// succeed - see `social_proximity_domain`'s `ProximityExceptionKind.
/// mfaRequired`). Two independent async steps rather than one: [enroll]
/// fetches a fresh secret to show the user (idempotent, safe to re-run if
/// the screen is reopened), [verify] submits the code they typed from
/// their authenticator app and completes only once the backend confirms
/// it - the screen awaits [verify] directly rather than reading it off
/// this notifier's state, since a wrong code should stay recoverable
/// in-place (see `MfaSetupScreen`), not blow away the still-valid
/// [MfaEnrollment] this notifier already fetched.
class MfaEnrollmentNotifier extends AsyncNotifier<MfaEnrollment?> {
  @override
  MfaEnrollment? build() => null;

  Future<void> enroll() async {
    state = const AsyncLoading<MfaEnrollment?>().copyWithPrevious(state);
    state = await AsyncValue.guard(
      () => ref.read(authRepositoryProvider).enrollMfa(),
    );
  }

  /// Throws [AuthException]/[AuthExceptionKind.invalidMfaCode] (see
  /// auth_exception.dart) on a wrong/expired code - deliberately not
  /// caught here so the screen can show an inline "código inválido"
  /// message without losing the already-fetched [MfaEnrollment] this
  /// notifier's state holds.
  Future<void> verify(String code) {
    return ref.read(authRepositoryProvider).verifyMfa(code: code);
  }
}

final mfaEnrollmentProvider =
    AsyncNotifierProvider<MfaEnrollmentNotifier, MfaEnrollment?>(MfaEnrollmentNotifier.new);
