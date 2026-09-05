# Spec: Replace the placeholder color seed with a red/black/white brand system

> **Superseded 2026-09-05**: this spec's "Technical Decisions" section chose black-as-dominant-surface with red as the *accent* (i.e. `colorScheme.secondary`-shaped role). The user later gave explicit, direct product direction reversing the role priority: **black is the primary brand color** (`colorScheme.primary` — buttons, the FAB, brand mark), **red and white are secondary**. The DoD below (WCAG contrast, brandRed/error distinguishability, no hardcoded colors outside the design system) still holds and was re-verified against the new role mapping; only the specific "which color plays which Material role" decision changed. See `.vibeflow/decisions.md`'s 2026-09-05 entry and `frontend-design-system.md`'s updated pattern for the current, correct mapping and two real rendering bugs the change surfaced.

## Objective
Replace `core_design_system`'s current placeholder brand color (which is, concretely, Spotify's own brand green) with a deliberate red/black/white identity — accent, not background — so smusic stops visually reading as a Spotify reskin and instead has its own identity, without falling into red's real contrast/connotation traps.

## Context
`frontend/packages/core/core_design_system/lib/src/tokens/colors.dart` defines exactly two colors:
```dart
static const Color brandSeed = Color(0xFF1ED760);
static const Color error = Color(0xFFE74C3C);
```
`0xFF1ED760` is not a coincidental green — it is Spotify's exact brand green (their published brand color). The file's own doc comment already flags this: *"Placeholder palette for Fatia 1 - not a final visual identity decision, just enough to build a coherent, theme-aware UI."* `SmusicTheme._build` (`smusic_theme.dart`) feeds this single seed into Material 3's `ColorScheme.fromSeed(...)`, which algorithmically derives the entire light AND dark palette from it — so today, every screen in the app is implicitly tinted with a color that is literally a competitor's brand mark. This was a reasonable placeholder to unblock Fatia 1 UI work, but shipping it as-is is both a genuine brand-identity risk and directly contradicts the explicit decision (this session, 2026-09-04) that smusic's primary colors are **red, black, and white**.

Two real pitfalls the naive version of "red, black, white" runs into, both worth deciding explicitly rather than discovering after the fact:
1. **Red-as-dominant-background fatigues the eye over long sessions** and reads as an alert/error state almost everywhere in UI convention (destructive actions, validation errors, "recording"/live indicators) — exactly the vocabulary a music app's normal, calm, long-lived-listening-session UI must NOT speak by default. Spotify itself doesn't use green as a background — it uses near-black (`#121212`) as the dominant surface and green only as a sparing accent (play button, active state, brand mark). The same logic should apply here: **black/near-black as the dominant surface, red as a disciplined accent, white for text/contrast on dark surfaces** — not red-as-wallpaper.
2. **Red already has a reserved meaning in this exact codebase**: `SmusicColors.error = Color(0xFFE74C3C)` — a red already means "error" in the design system's own vocabulary (see `EmptyState`'s icon-color usage and any validation messaging in `auth_ui`). If the brand accent color and the error color are both "red," they need to be *visually distinct enough* (different hue/tone, not just different opacity) that a user can never confuse "this button is red because it's the primary action" with "this is red because something is wrong." This must be a deliberate design decision, documented, not an accident of picking one shared "red" for both.

## Definition of Done
- [ ] `SmusicColors` (or its replacement) defines an explicit, named palette — at minimum: `brandRed` (accent, e.g. a saturated red distinct in hue/tone from `error`), `surfaceBlack`/`surfaceDark` (dominant dark-mode background), `pureWhite`/`onSurfaceLight` (text/contrast), and keeps `error` as its own, visually distinguishable red-adjacent tone (per Context point 2 — could differ in saturation, or shift error toward an orange-red while brand stays a truer red, but the two must be tested as distinguishable, not just asserted to be).
- [ ] `SmusicTheme.dark()` uses a genuinely near-black surface (not Material 3's default mid-gray-derived-from-seed dark surface) as `scaffoldBackgroundColor`/`colorScheme.surface` — dark mode must look authentically dark/black, not "dark gray that happens to have a red tint."
- [ ] `SmusicTheme.light()` still exists and is coherent (white/near-white surface, black text, red accent) — this spec does not remove light mode, even if dark is the primary/default (confirm/set the default in whichever entrypoint currently decides light vs dark vs system, e.g. `smusic_app_shared`).
- [ ] A contrast check is actually run (not eyeballed) for at least: white text on the red accent's most common button context, and the error red against the dark surface — both must clear **WCAG AA (4.5:1 for normal text, 3:1 for large text/UI components)**. Record the actual computed ratios in the PR/commit, not just "looks fine."
- [ ] Every existing usage of `SmusicColors.brandSeed` across the design system and presentation packages is updated to the new token names (no leftover references to the old green seed or its old name).
- [ ] No violation of `conventions.md` Don'ts — in particular, no new hardcoded `Color(0x...)`/`Colors.*` is introduced in any `presentation/*` screen or widget outside `core_design_system`'s own token files (this codebase's existing 100%-token-adherence for color is a real, verified strength — see this fork's audit — don't regress it while making this change).

## Scope
- `frontend/packages/core/core_design_system/lib/src/tokens/colors.dart` — new palette.
- `frontend/packages/core/core_design_system/lib/src/theme/smusic_theme.dart` — dark surface override, light/dark construction.
- Any call site referencing the old `brandSeed` name (grep for it — at minimum `smusic_theme.dart` itself; check presentation packages too in case any screen references design-system colors directly rather than via `Theme.of(context)`).
- The contrast-ratio verification itself (can be a quick script/calculation, doesn't need new tooling).

## Anti-scope
- Do NOT redesign individual screens/widgets in this spec — this is a token/theme-level change; Material 3's `ColorScheme.fromSeed` (or an explicit `ColorScheme(...)` construction, if the seed-based derivation doesn't give good-enough red-on-black results — that choice is this spec's to make and document) propagates the new palette to every screen automatically, since 100% of sampled UI code already goes through `Theme.of(context)` rather than hardcoded colors.
- Do NOT change `SmusicSpacing` or any non-color token in this pass — unrelated to the color-system decision.
- Do NOT design a marketing/logo identity (wordmark, app icon asset) — this spec is the in-app UI color system only.
- Do NOT silently repurpose the existing `error` red as the brand accent to "save a color" — Context point 2 above is exactly why that's the wrong shortcut.

## Technical Decisions
- **Black as the dominant surface, red as accent, white for contrast** (not red-as-background) — matches the reference apps' own actual pattern (dark/near-black dominant surface + one disciplined accent color) rather than a literal "red backgrounds everywhere" reading of "red, black and white são as cores principais." "Principais" is satisfied by these three colors defining 100% of the palette's *roles* (surface, accent, contrast) — it does not require each to appear in equal visual proportion, and equal proportion is precisely what would make the UI fatiguing and error-coded.
- **Keep `ColorScheme.fromSeed` if it produces adequate contrast, else hand-author a `ColorScheme`**: `fromSeed` is convenient (one seed → full tonal palette, light+dark) and is what's already in place; only abandon it for a fully hand-authored `ColorScheme` if the seed-derived dark surface can't be forced dark/black enough or the derived accent-on-surface contrast fails the WCAG check above — don't discard working infrastructure preemptively.

## Applicable Patterns
- No existing `.vibeflow/patterns/` doc covers the design system yet — see `frontend-design-system.md` (created alongside this spec by the same audit) for the token/theme pattern this change must follow.
- `frontend-layered-architecture.md` — this change lives entirely in `core/core_design_system`, the correct layer for a cross-cutting visual-identity change (no feature package should hardcode color).

## Risks
- **Risk**: a seed-derived dark theme may not get dark/black enough automatically, or the derived "primary" tone from a red seed may compute to a shade that fails contrast against black. **Mitigation**: this spec's DoD explicitly requires measuring, not assuming — check computed values before calling this done, and fall back to a hand-authored `ColorScheme` if `fromSeed` can't be tuned into range.
- **Risk**: red-as-accent everywhere a button/CTA appears could still visually clash with the existing `error` red in specific screens (e.g. a form with both a red "Sign up" button and a red validation error message on screen at once). **Mitigation**: the DoD's "visually distinguishable" requirement for brand-vs-error red is meant to catch exactly this before it ships, not after.
