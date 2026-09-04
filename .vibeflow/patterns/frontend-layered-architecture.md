---
tags: [flutter, dart, melos, monorepo, layering, architecture]
modules: [frontend/packages/, frontend/app/, frontend/tool/]
applies_to: [packages, modules]
confidence: inferred
---
# Pattern: Frontend Layered Monorepo (Melos)

<!-- vibeflow:auto:start -->
## What
A Melos-managed Dart/Flutter monorepo with 4 strict layers — `core/`,
`domain/`, `data/`, `presentation/` — plus a thin `app/` composition layer.
Enforced by a grep-based script, not just convention.

## Where
`frontend/packages/{core,domain,data,presentation}/*` (19 packages total)
and `frontend/app/{smusic_app_shared,smusic_mobile,smusic_web}`.

## The Pattern
- `core/*` (core_platform, core_networking, core_design_system): no
  dependency on any feature package. Houses genuinely cross-feature
  primitives (e.g. `ReconnectingWebSocketClient`, `ApiClient`,
  `NativeAudioEngine` interface).
- `domain/<feature>_domain`: pure Dart, framework-agnostic business logic
  (entities, repository interfaces, Riverpod notifiers). MUST NOT import
  `package:flutter/*` or any `*_data`/`*_ui` package.
- `data/<feature>_data`: concrete repository implementations (HTTP/WS)
  implementing the domain's repository interfaces.
- `presentation/<feature>_ui`: Flutter widgets/screens, consumes domain
  providers only — MUST NOT import `*_data` directly.
- `app/smusic_app_shared`: the single `SmusicApp` widget (root + theme +
  router), imported unmodified by both platform entrypoints.
- `app/smusic_mobile` and `app/smusic_web`: thin entrypoints. Each `main.dart`
  only builds concrete implementations (`core_platform` overrides) and calls
  `runApp(ProviderScope(overrides: [...], child: const SmusicApp()))`.

**Verified 100% reuse claim**: `smusic_mobile/lib/main.dart` (140 lines) and
`smusic_web/lib/main.dart` (115 lines) are structurally identical — same
imports, same wiring, same `buildPresenceSocketClient`/`buildPresenceUri`
helper functions duplicated verbatim between the two files. The only
difference is a literal string prefix (`'mobile-'` vs `'web-'`) in the
device-id fallback. `smusic_app_shared/lib/` is only 84 lines total across 3
files (`smusic_app_shared.dart`, `app_router_provider.dart`,
`smusic_app.dart`) — genuinely minimal, all real UI lives in `presentation/*`
packages. No platform-specific screens or business logic forks were found.

## Rules
- Enforced by `frontend/tool/check_layer_deps.sh` (run via `melos run
  check-layers`): domain layer forbidden from importing flutter or
  data/ui/mobile/web packages; presentation layer forbidden from importing
  data packages directly.
- Not wired into CI yet — documented TODO in `frontend/README.md`. This is a
  grep-based stand-in for a real `dart_dependency_validator`/custom-lint
  rule.
- New platform-specific code (mobile vs web) should live in `core_platform`
  concrete implementations selected at the composition root (`main.dart`),
  never as a fork of `smusic_app_shared` or any presentation package.

## Examples from this codebase
File: `frontend/tool/check_layer_deps.sh`
```bash
check_forbidden "packages/domain/*/lib" \
  "flutter/" "flutter_test/" \
  "auth_data/" "player_data/" "library_data/" "social_proximity_data/" \
  "auth_ui/" "player_ui/" "library_ui/" "social_proximity_ui/" "shared_navigation/"

check_forbidden "packages/presentation/*/lib" \
  "auth_data/" "player_data/" "library_data/" "social_proximity_data/"
```

File: `frontend/app/smusic_mobile/lib/main.dart` (identical shape in
`smusic_web/lib/main.dart`)
```dart
void main() {
  final tokenStorage = SecureTokenStorage();
  ...
  runApp(
    ProviderScope(
      overrides: [ /* platform-specific concrete implementations */ ],
      child: const SmusicApp(),
    ),
  );
}
```
<!-- vibeflow:auto:end -->

## Anti-patterns
None found — the layering rule is consistently followed in the sampled
packages. The main structural risk is that enforcement is grep-based and
not run in CI (self-documented TODO), so a violation could land undetected
until someone runs `melos run check-layers` manually.
