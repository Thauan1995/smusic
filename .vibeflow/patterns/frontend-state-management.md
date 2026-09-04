---
tags: [flutter, dart, riverpod, state-management, notifiers]
modules: [frontend/packages/domain/, frontend/packages/presentation/]
applies_to: [notifiers, providers, screens, hooks]
confidence: inferred
---
# Pattern: Riverpod AsyncNotifier State Management

<!-- vibeflow:auto:start -->
## What
State management is Riverpod (`riverpod` in pure-Dart domain packages,
`flutter_riverpod`/`hooks_riverpod` in Flutter packages), using
`AsyncNotifier`/`Notifier` classes defined in `domain/*` and consumed via
`ConsumerWidget`/`HookConsumerWidget` in `presentation/*`.

## Where
Every `domain/<feature>_domain` package defines its own notifiers and
providers (e.g. `NearbyFeedNotifier`, `ProximityPrivacySettingsNotifier`,
`LocationPermissionNotifier` in `social_proximity_domain`). Screens in
`presentation/*_ui` read them with `ref.watch`/`ref.read`.

## The Pattern
- Cross-cutting/derived state is composed by watching multiple providers'
  `.future` inside another notifier's `build()`, rather than the UI
  combining them.
- Providers for concrete implementations (repositories, platform services)
  are declared in `domain` as bare providers with no default implementation,
  and overridden at the composition root (`main.dart`) via
  `ProviderScope(overrides: [...])` — this is how DI happens, no separate
  service-locator/GetIt.
- Errors from a watched dependency are caught locally and mapped to a safe
  fallback state rather than propagating and crashing the combining
  notifier.

## Rules
- Domain notifiers must stay framework-agnostic (`package:riverpod`, not
  `package:flutter_riverpod`) — see the layered-architecture pattern.
- Use `ref.watch(x.future)` (not `ref.watch(x)`) when a notifier's `build()`
  needs another provider's *resolved* value, not just its current
  `AsyncValue` — reading the raw `AsyncValue` while still loading can leave
  the combining provider's future permanently unsettled (see the
  `NearbyFeedNotifier` example below).
- UI never re-implements policy that a domain notifier already encodes
  (e.g. "should the socket be open right now") — the notifier is the single
  source of truth.

## Examples from this codebase
File: `frontend/packages/domain/social_proximity_domain/lib/src/nearby_feed_notifier.dart`
```dart
class NearbyFeedNotifier extends AsyncNotifier<NearbyFeedState> {
  @override
  Future<NearbyFeedState> build() async {
    ProximityPrivacySettings? settings;
    try {
      settings = await ref.watch(proximityPrivacySettingsProvider.future);
    } catch (_) {
      settings = null;
    }
    var permission = LocationPermissionState.notRequested;
    try {
      permission = await ref.watch(locationPermissionProvider.future);
    } catch (_) {
      // keep notRequested
    }
    final repository = ref.read(proximityFeedRepositoryProvider);
    final shouldRun = settings != null && settings.isActive && ...
  }
}
```

File: `frontend/packages/presentation/social_proximity_ui/lib/src/screens/proximity_permission_gate.dart`
```dart
class ProximityPermissionGate extends ConsumerWidget {
  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return Scaffold(
      body: EmptyState(
        ...
        onAction: () {
          final notifier = ref.read(locationPermissionProvider.notifier);
          if (_isPermanentlyBlocked) {
            notifier.openAppSettings();
          } else {
            notifier.request();
          }
        },
      ),
    );
  }
}
```

File: `frontend/app/smusic_mobile/lib/main.dart` — DI via provider overrides
```dart
runApp(
  ProviderScope(
    overrides: [
      authRepositoryProvider.overrideWithValue(authRepository),
      locationProviderProvider.overrideWithValue(locationProvider),
      proximityFeedRepositoryProvider.overrideWithValue(proximityFeedRepository),
    ],
    child: const SmusicApp(),
  ),
);
```
<!-- vibeflow:auto:end -->
