import 'package:riverpod/riverpod.dart';

import 'entities/proximity_privacy_settings.dart';
import 'entities/proximity_radius.dart';
import 'entities/proximity_visibility_mode.dart';
import 'entities/reveal_level.dart';
import 'proximity_providers.dart';
import 'repositories/proximity_privacy_settings_repository.dart';

/// security.md section 1's privacy settings, backing the dedicated privacy
/// settings screen (task scope item 2) and the quick "pausar descoberta"
/// toggle (item 5/security.md 1.4).
class ProximityPrivacySettingsNotifier extends AsyncNotifier<ProximityPrivacySettings> {
  @override
  Future<ProximityPrivacySettings> build() {
    return ref.watch(proximityPrivacySettingsRepositoryProvider).fetch();
  }

  Future<void> _mutate(
    Future<ProximityPrivacySettings> Function(ProximityPrivacySettingsRepository repo) op,
  ) async {
    final repo = ref.read(proximityPrivacySettingsRepositoryProvider);
    state = const AsyncLoading<ProximityPrivacySettings>().copyWithPrevious(state);
    state = await AsyncValue.guard(() => op(repo));
  }

  ProximityPrivacySettings get _currentOrDisabled =>
      state.valueOrNull ?? ProximityPrivacySettings.disabled();

  /// security.md 1.1: opt-in, with the value screen having already run in
  /// the UI. Two backend calls, both flagged as a deliberate product
  /// decision (see class doc comment / task report "Desvios da spec"):
  /// [ProximityPrivacySettingsRepository.grantConsent] alone
  /// (`POST /v1/presence/consent`) sets `proximity_consent_enabled` +a
  /// fresh 6-month renewal window, but - per `SettingsService.GrantConsent`
  /// - never changes `paused`/`presence_visibility`, which both start at
  /// their safe defaults (`paused: true`, `presence_visibility: invisible`)
  /// for an account that has never touched presence settings. Left there,
  /// a user who taps this screen's single "Ativar" CTA would see the
  /// feature report `enabled` yet stay invisible/paused - not what the CTA
  /// promises. So this method also unpauses and, **only on a true first
  /// activation** (see [_hadPreviouslySavedConfiguration] below), defaults
  /// visibility to [ProximityVisibilityMode.everyone] (security.md 1.6's
  /// [RevealLevel.level0] - "Alguém por perto está ouvindo *Faixa*", no
  /// name/avatar - is already the *default* [ProximityPrivacySettings.
  /// maxRevealLevel], so "visible to everyone" here still starts fully
  /// anonymous; the user can narrow to friends-only or raise the reveal
  /// ceiling afterwards via the settings screen's existing selectors).
  ///
  /// A **returning** user - someone who granted consent before, chose a
  /// visibility (e.g. [ProximityVisibilityMode.friendsOnly]), then paused
  /// or fully disabled the feature - must NOT have that explicit choice
  /// silently overwritten on reactivation. `SettingsService.RevokeConsent`
  /// (backend) only touches `proximity_consent_enabled`/`paused` server-
  /// side; it never resets `presence_visibility`, so the previously chosen
  /// [ProximityPrivacySettings.visibilityMode] is still sitting in the
  /// fetched settings when this method runs - it is simply preserved
  /// rather than read as "the new default."
  ///
  /// Unlike every other mutator in this notifier, a failure here is
  /// rethrown (in addition to being recorded in [state], same as
  /// [_mutate]) - `SettingsService.GrantConsent`'s
  /// [ProximityExceptionKind.mfaRequired] gate (security.md §2's TOTP
  /// step-up) is a case the calling screen must react to (send the user to
  /// MFA enrollment, then retry), not just display as a generic error.
  Future<void> enableFeature() async {
    // Captured *before* [ProximityPrivacySettingsRepository.grantConsent]
    // runs: that call always stamps a fresh `consentGivenAt` (see its own
    // doc comment / `SettingsService.GrantConsent`), so checking
    // [_hadPreviouslySavedConfiguration] afterwards would see a non-null
    // timestamp unconditionally and misclassify every activation - first or
    // returning - as "returning."
    final isFirstActivation = !_hadPreviouslySavedConfiguration(_currentOrDisabled);
    final previousVisibilityMode = _currentOrDisabled.visibilityMode;
    final repo = ref.read(proximityPrivacySettingsRepositoryProvider);

    state = const AsyncLoading<ProximityPrivacySettings>().copyWithPrevious(state);
    final ProximityPrivacySettings afterConsent;
    try {
      afterConsent = await repo.grantConsent();
    } catch (error, stackTrace) {
      state = AsyncError<ProximityPrivacySettings>(error, stackTrace).copyWithPrevious(state);
      rethrow;
    }
    state = AsyncData<ProximityPrivacySettings>(afterConsent);

    state = await AsyncValue.guard(
      () => repo.update(
        afterConsent.copyWith(
          paused: false,
          visibilityMode:
              isFirstActivation ? ProximityVisibilityMode.everyone : previousVisibilityMode,
        ),
      ),
    );
  }

  /// Distinguishes "this account has never granted proximity consent
  /// before" from "this account granted consent before, then paused or
  /// disabled the feature." [ProximityPrivacySettings.consentGivenAt] is
  /// the right signal for that, not [ProximityPrivacySettings.
  /// visibilityMode] itself: `presence_visibility` defaults to
  /// [ProximityVisibilityMode.invisible] for a brand-new account
  /// (`DefaultPrivacySettings` server-side) *and* is a value a returning
  /// user could have deliberately chosen before pausing, so it can't safely
  /// double as its own "never configured" sentinel. `consentGivenAt`,
  /// however, is only ever set by [ProximityPrivacySettingsRepository.
  /// grantConsent] (`SettingsService.GrantConsent`'s `ProximityConsentTS`)
  /// and is never cleared by [ProximityPrivacySettingsRepository.
  /// revokeConsent] (`SettingsService.RevokeConsent` leaves it untouched) -
  /// so a non-null value here unambiguously means "this account has been
  /// through the consent flow at least once before," independent of which
  /// visibility it last chose.
  bool _hadPreviouslySavedConfiguration(ProximityPrivacySettings settings) =>
      settings.consentGivenAt != null;

  /// security.md 1.1 §5º: revocation must be exactly as easy as granting -
  /// one call, immediate effect, no re-confirmation step here (the "tem
  /// certeza?" friction security.md explicitly forbids belongs in the UI
  /// only if product ever adds it against this doc's guidance - this
  /// notifier never requires it). Delegates to
  /// [ProximityPrivacySettingsRepository.revokeConsent]
  /// (`DELETE /v1/presence/consent`), which also force-pauses server-side.
  Future<void> disableFeature() {
    return _mutate((repo) => repo.revokeConsent());
  }

  /// security.md 1.4: quick "pausar descoberta" toggle.
  Future<void> setPaused(bool paused) {
    return _mutate((repo) => repo.update(_currentOrDisabled.copyWith(paused: paused)));
  }

  Future<void> setVisibilityMode(ProximityVisibilityMode mode) {
    return _mutate((repo) => repo.update(_currentOrDisabled.copyWith(visibilityMode: mode)));
  }

  Future<void> setRadius(ProximityRadius radius) {
    return _mutate((repo) => repo.update(_currentOrDisabled.copyWith(radius: radius)));
  }

  /// security.md 1.6: raising to [RevealLevel.level2] is the "descoberta
  /// aberta" second consent - the caller (UI) is responsible for having
  /// shown that separate confirmation before calling this.
  Future<void> setMaxRevealLevel(RevealLevel level) {
    return _mutate((repo) => repo.update(_currentOrDisabled.copyWith(maxRevealLevel: level)));
  }

  /// security.md 1.1: the 6-month re-confirmation flow - the same
  /// `POST /v1/presence/consent` backend call as [enableFeature]'s first
  /// step (`SettingsService.GrantConsent` "enables (or renews)"), called
  /// alone here since a renewal does not need to also touch visibility/
  /// paused the way a first-time activation does.
  Future<void> renewConsent() {
    return _mutate((repo) => repo.grantConsent());
  }
}

final proximityPrivacySettingsProvider =
    AsyncNotifierProvider<ProximityPrivacySettingsNotifier, ProximityPrivacySettings>(
  ProximityPrivacySettingsNotifier.new,
);
