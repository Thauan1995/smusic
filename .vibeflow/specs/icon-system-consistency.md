# Spec: Standardize icon style (filled vs outlined) across the app

## Objective
Make icon style follow one consistent, state-driven rule everywhere instead of the current ad hoc mix of filled and outlined variants for what should be the same semantic state.

## Context
The app uses Flutter's built-in Material Icons exclusively (no custom icon font/package in any `pubspec.yaml` — confirmed via repo-wide search) across ~29 distinct icons in the sampled presentation packages (`auth_ui`, `library_ui`, `player_ui`, `social_proximity_ui`, `shared_navigation`). Using a single icon family is a real strength — there is no risk of mixed icon "weights" from different sources. The problem is **inconsistent use of the filled/outlined pair for what should be the same semantic distinction**:
- `Icons.play_circle_filled` (2 uses) vs `Icons.play_circle_outline` (2 uses)
- `Icons.pause_circle_filled` (2 uses) vs `Icons.pause_circle_outline` (2 uses)
- `Icons.person` (2 uses, filled-by-default) vs `Icons.person_outline` (1 use)

Nothing in the codebase documents *when* a filled vs. outlined variant should be chosen — from sampling, it reads as whichever the original author reached for at the time, not a rule (e.g. it is not consistently "outlined = inactive/secondary state, filled = active/primary state," even though that is the obvious, common convention this pairing exists to support — Spotify/YouTube Music both use exactly that convention: an outlined icon for an unselected nav/tab item, filled for the selected one).

## Definition of Done
- [ ] A documented rule exists (in code comments on a new shared icon-selection helper, or in `.vibeflow/patterns/frontend-design-system.md`) for when to use filled vs. outlined: e.g. "outlined for an inactive/available action or unselected state, filled for an active/selected/in-progress state."
- [ ] Every current filled/outlined pair in the app (play/pause circle icons, person icon) is audited against that rule and corrected where it doesn't match — cite each corrected call site's file:line in the PR.
- [ ] `shared_navigation`'s tab/nav icons (if any use filled/outline pairs for selected vs. unselected state — check `navigation_shell.dart`/`app_router.dart`) explicitly follow the rule; if `shared_navigation` doesn't yet distinguish selected/unselected icon state at all, that itself is a finding to fix as part of this spec (a nav bar with no visual "you are here" signal beyond route highlighting is a real usability gap against the Spotify/YouTube Music bar for bottom/side navigation).
- [ ] No new icon usage introduced elsewhere in this same change deviates from the documented rule (a lint-by-review check, not automatable in this pass).
- [ ] No violation of `conventions.md` Don'ts.

## Scope
- An audit + fix pass over the ~10 filled/outlined icon call sites identified above (and any others the fuller fix turns up in `shared_navigation`'s nav icons specifically).
- A short, documented rule (a code comment where the pattern is centralized, or a section in `frontend-design-system.md`) — not a new abstraction/widget unless the audit shows the same 2-3 icon pairs are duplicated across many screens, in which case a tiny `StateIcon`-style helper is justified (see Anti-scope for the line not to cross).

## Anti-scope
- Do NOT introduce a custom icon font or third-party icon package (e.g. Font Awesome, Feather) — Material Icons' built-in filled/outlined pairing already covers every icon currently in use; swapping icon *sources* is a much bigger, unrelated design decision this spec doesn't need to make.
- Do NOT build a general-purpose "icon registry"/theming abstraction beyond what's needed to fix the current ~10 inconsistent call sites — that's speculative infrastructure for a problem this small.
- Do NOT change icon *choice* (which icon represents which concept) — only *style* (filled vs. outlined) for a given already-chosen icon. If an icon choice itself seems wrong, note it, don't silently swap it here.

## Technical Decisions
- **Filled = active/selected/in-progress, outlined = inactive/available/unselected** as the single rule, matching both the reference apps' own convention and Material Design's own guidance for this exact icon pairing — no need to invent a project-specific rule when an industry-standard one already fits and is what a Spotify/YouTube Music-literate user already expects.

## Applicable Patterns
- `frontend-design-system.md` (new, created alongside this audit) — the rule from this spec should be recorded there so it isn't relitigated per-screen in the future.

## Risks
- **Risk**: "fixing" an icon's filled/outlined state without also checking its surrounding widget's actual selected/active logic could just move the inconsistency (e.g. showing the "outlined" variant even when the state, on inspection, actually is active). **Mitigation**: the DoD requires citing each corrected call site with its surrounding logic checked, not a blind icon-name swap.
