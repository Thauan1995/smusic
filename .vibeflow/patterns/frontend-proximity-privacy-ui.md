---
tags: [privacy, location, permissions, opt-in, social-proximity]
modules: [frontend/packages/domain/social_proximity_domain/, frontend/packages/data/social_proximity_data/, frontend/packages/presentation/social_proximity_ui/]
applies_to: [notifiers, screens, repositories]
confidence: inferred
---
# Pattern: Opt-in Proximity Discovery with Privacy Gating

<!-- vibeflow:auto:start -->
## What
The frontend implementation of the "social proximity discovery" feature —
the product's stated competitive differentiator — built as a strict
opt-in flow gated by both consent settings and OS location permission,
never auto-prompting.

## Where
`frontend/packages/domain/social_proximity_domain/` (state/policy),
`frontend/packages/data/social_proximity_data/` (HTTP + WS repositories),
`frontend/packages/presentation/social_proximity_ui/` (value screen,
permission gate, privacy settings screen, nearby list).

## The Pattern
- `NearbyFeedNotifier` is the single policy point deciding "should the
  socket be open right now": it only runs when
  `ProximityPrivacySettings.isActive` (opted in, consent not lapsed, not
  paused) **and** location permission is granted — the UI never
  independently re-implements this check.
- The permission flow is gated behind a value screen
  (`proximity_value_screen.dart`) shown *before* the OS permission prompt,
  matching the backend's opt-in-first privacy model.
- On denial, `ProximityPermissionGate` shows an explanatory empty state
  with a manual retry CTA (or "open settings" if permanently denied) — the
  OS prompt is **never** re-triggered automatically by re-rendering.
- Distance is only ever consumed as a bucketed enum
  (`DistanceBucket`/`RevealLevel`) from the wire — no raw coordinates or
  geohash reach the client, mirroring the backend's data model.

## Rules
- Any new proximity UI must read activation state from
  `NearbyFeedNotifier`/`ProximityPrivacySettingsNotifier`, never derive it
  locally from permission or settings state directly.
- Never build UI that re-prompts the OS location dialog automatically —
  only an explicit user tap (`request()`) may trigger it.
- Never expose raw location/geohash in any widget or DTO — only the
  bucketed types already defined in `social_proximity_domain/entities/`.

## Examples from this codebase
File: `frontend/packages/presentation/social_proximity_ui/lib/src/screens/proximity_permission_gate.dart`
```dart
class ProximityPermissionGate extends ConsumerWidget {
  bool get _isPermanentlyBlocked =>
      permission == LocationPermissionState.deniedForever ||
      permission == LocationPermissionState.restricted;

  Widget build(BuildContext context, WidgetRef ref) {
    return Scaffold(
      body: EmptyState(
        actionLabel: _isPermanentlyBlocked ? 'Abrir configurações' : 'Permitir localização',
        onAction: () {
          final notifier = ref.read(locationPermissionProvider.notifier);
          if (_isPermanentlyBlocked) { notifier.openAppSettings(); } else { notifier.request(); }
        },
      ),
    );
  }
}
```

File: `frontend/packages/domain/social_proximity_domain/lib/src/nearby_feed_notifier.dart`
```dart
final shouldRun = settings != null && settings.isActive &&
    permission == LocationPermissionState.granted;
```
<!-- vibeflow:auto:end -->
