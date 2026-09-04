---
tags: [testing, coverage, flutter-test, dart-test, mocking]
modules: [frontend/packages/, frontend/app/]
applies_to: [tests]
confidence: inferred
---
# Pattern: Per-Package Coverage via Melos

<!-- vibeflow:auto:start -->
## What
Every package that has a `test/` directory is tested independently
(`flutter test --coverage` for Flutter packages, `dart test --coverage` for
pure-Dart domain packages), run in aggregate via `melos run test`. All 19
packages plus the 2 platform entrypoints have their own `coverage/` output
directory, confirming per-package coverage is actually generated, not just
configured.

## Where
`frontend/melos.yaml` (`test` script), every
`packages/*/*/test/` and `app/*/test/` directory.

## The Pattern
- The stated project-wide policy (`docs/architecture/00-overview.md` §2):
  100% coverage on all hand-written domain/business logic, excluding
  generated code (`*.g.dart`, `*.freezed.dart`), composition-root wiring
  (`main.dart`), and explicitly-justified defensive branches — every
  exclusion must carry an explicit, reviewable justification, never silent.
- `smusic_mobile`/`smusic_web`'s `test/` covers only
  `buildPresenceUri`/`buildPresenceSocketClient` — the one piece of
  `main.dart` with real branching logic worth testing directly (the rest
  is pure wiring, excluded per policy).
- `social_proximity_domain` (16 test files), `social_proximity_ui` (10),
  `social_proximity_data` (4) — the differentiator feature has the densest
  test coverage of the sampled packages.

## Rules
- A coverage exclusion without an explicit code/PR-level justification does
  not count as coverage achieved, per the recorded policy — this is the
  Auditor's evaluation criterion, not a suggestion.
<!-- vibeflow:auto:end -->
