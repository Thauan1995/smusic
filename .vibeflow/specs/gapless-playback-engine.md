# Spec: Wire real gapless playback via ConcatenatingAudioSource

## Objective
Make track-to-track transitions actually gapless in production, closing a known-but-unresolved gap directly against the founding brief's "match/exceed Spotify/YouTube Music" playback bar.

## Context
`00-overview.md` §3 lists this as tracked, non-blocking tech debt from Fatia 1's reauditoria: "`JustAudioNativeEngine.setNextSource` é no-op em produção — falta `ConcatenatingAudioSource` para gapless real funcionar de ponta a ponta (o prefetch já busca/resolve a próxima faixa corretamente, só o engine não usa isso ainda)." Confirmed still true during this analysis: `JustAudioNativeAudioEngine.load()` always builds a plain `AudioSource.uri(...)`; `setNextSource()`'s own guard (`if (current is ConcatenatingAudioSource)`) can never be true given how `load()` always constructs the source, so the method is effectively dead code in production even though it compiles and appears to be part of the implemented flow. The prefetch/resolve half (fetching and resolving the next track's signed URL ahead of time) does work — only the player-engine wiring to actually chain playback without a gap is missing.

## Definition of Done
- [ ] `JustAudioNativeAudioEngine.load()` constructs a `ConcatenatingAudioSource` (or the current `just_audio` API's equivalent gapless-queue primitive) instead of a bare `AudioSource.uri(...)`, seeded with the current track.
- [ ] `setNextSource()` successfully appends/sets the next track's already-resolved source onto the live `ConcatenatingAudioSource` instance — its existing `is ConcatenatingAudioSource` guard now actually evaluates true in the real playback path (delete the guard only if the new design makes it structurally unreachable — the DoD is "the intended gapless behavior actually fires," not "this specific guard exists").
- [ ] A widget/unit test exercises: track A playing → prefetch resolves track B → transition to B has no audible/measurable gap (or, at minimum, verifies `just_audio`'s queue-advance API was actually invoked with the prefetched source, since a full audio-gap measurement isn't practical in a test harness — assert on the engine's interaction with the mocked `just_audio` player, per `frontend-audio-playback.md`'s existing test pattern).
- [ ] Existing tests for `JustAudioNativeAudioEngine` and the prefetch logic in `player_data`/`player_domain` still pass unmodified in spirit (adjust mocks only as required by the new `ConcatenatingAudioSource` usage, don't rewrite unrelated assertions).
- [ ] `frontend-audio-playback.md`'s "Anti-patterns" section (which currently documents this exact gap) is updated to reflect the fix, or the anti-pattern note is removed if fully resolved.
- [ ] No violation of `conventions.md` Don'ts — no platform-conditional logic introduced (this lives entirely in `core_platform`'s existing native-adapter abstraction, per the layering rule).

## Scope
- `frontend/packages/core/core_platform`'s `JustAudioNativeAudioEngine` (or wherever it now lives per `frontend-audio-playback.md`).
- Any interface changes to the `NativeAudioEngine`/`AudioEngine` abstraction needed to expose "queue next source for gapless playback" cleanly to `player_data`/`player_domain` — keep the interface change minimal; the domain/data layers already call `setNextSource()`, this spec should not need to change *who* calls it, only *what happens* inside the engine.

## Anti-scope
- Do NOT change the prefetch/resolve logic in `player_data`/`player_domain` — per the context above, that half already works correctly; this spec is scoped to the engine-level wiring only.
- Do NOT add crossfade, gapless-with-effects, or any playback feature beyond "no audible gap between sequential tracks" — that's explicitly what `backend-go.md`'s "match Spotify/YT Music" bar implies for this specific tech-debt item, nothing more.
- Do NOT touch the backend's `LocalResolver`/signed-URL logic — this is a pure frontend/client engine fix; the backend already correctly resolves and signs the next track's URL.

## Technical Decisions
- **`ConcatenatingAudioSource` over manual gap-hiding tricks** (e.g. pre-buffering and manually splicing playback state): `just_audio`'s own primitive for this exact use case, and the codebase's existing prefetch design already resolves the next source ahead of time specifically to feed this primitive — the architecture was already pointed at this solution, it just wasn't finished.

## Applicable Patterns
- `frontend-audio-playback.md` — this spec directly resolves that doc's documented anti-pattern; the fix must stay inside the same engine abstraction the pattern doc describes, not bypass it.
- `frontend-state-management.md` — if `setNextSource`'s call site in a Riverpod notifier needs any signature adjustment, follow the existing notifier pattern, don't introduce a new state-management approach for this one feature.

## Risks
- **Risk**: `just_audio`'s `ConcatenatingAudioSource` API differs across platform backends (web vs mobile) in subtle ways (a common source of Flutter audio-plugin bugs). **Mitigation**: since `smusic_mobile`/`smusic_web` share 100% of this code per `frontend-layered-architecture.md`'s verified-thin-entrypoint finding, test on both a mobile target and web/Chrome before calling this done — a fix that only works on one platform silently breaks the "100% shared code" guarantee's implicit promise of "same behavior everywhere."
- **Risk**: this was flagged as non-blocking tech debt in the original Fatia 1 reaudit — implementing it now should not be treated as license to also relitigate other tracked-but-deferred items in the same pass (see Anti-scope on the E2E test blocker, which is a separate, unrelated item in the same `00-overview.md` §3 list).
