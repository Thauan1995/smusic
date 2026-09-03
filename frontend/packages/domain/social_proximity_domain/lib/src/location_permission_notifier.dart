import 'package:core_platform/core_platform.dart' show LocationPermissionStatus;
import 'package:riverpod/riverpod.dart';

import 'proximity_providers.dart';

/// frontend-flutter.md section 4.4's permission state machine, surfaced to
/// the UI as an `AsyncNotifier` so `social_proximity_ui` renders
/// loading/data/error the same way every other feature does (section 1.1's
/// `AsyncValue`-driven rule).
///
/// [request] is only ever meant to be called *after* the UI's own value
/// screen (frontend-flutter.md 4.4: "tela de explicação de valor antes do
/// prompt de permissão do SO") - this class does not enforce that ordering
/// itself (it is a UI-flow concern, not a state-machine concern), it simply
/// never calls [request] on its own from [build].
class LocationPermissionNotifier extends AsyncNotifier<LocationPermissionStatus> {
  @override
  Future<LocationPermissionStatus> build() {
    return ref.watch(locationProviderProvider).checkPermission();
  }

  /// Triggers the OS permission prompt. Per frontend-flutter.md 4.4 ("Se
  /// negado ... nunca insiste com prompt repetido"), callers must gate this
  /// behind the value screen and must not call it again automatically after
  /// a [LocationPermissionStatus.deniedOnce]/[LocationPermissionStatus.deniedForever]
  /// result - that policy lives in `social_proximity_ui`'s permission flow
  /// screen, not here.
  Future<void> request() async {
    final provider = ref.read(locationProviderProvider);
    state = const AsyncLoading<LocationPermissionStatus>().copyWithPrevious(state);
    state = await AsyncValue.guard(provider.requestPermission);
  }

  /// Re-checks the OS permission without prompting - used when the app
  /// resumes from background (the user may have changed the permission in
  /// OS settings after using the [openAppSettings] CTA).
  Future<void> refresh() async {
    final provider = ref.read(locationProviderProvider);
    state = const AsyncLoading<LocationPermissionStatus>().copyWithPrevious(state);
    state = await AsyncValue.guard(provider.checkPermission);
  }

  /// frontend-flutter.md 4.4: CTA for [LocationPermissionStatus.deniedForever].
  Future<bool> openAppSettings() {
    return ref.read(locationProviderProvider).openAppSettings();
  }
}

final locationPermissionProvider =
    AsyncNotifierProvider<LocationPermissionNotifier, LocationPermissionStatus>(
  LocationPermissionNotifier.new,
);
