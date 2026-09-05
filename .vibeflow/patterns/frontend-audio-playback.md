---
tags: [audio, just-audio, playback, gapless, prefetch]
modules: [frontend/packages/core/core_platform/, frontend/packages/data/player_data/]
applies_to: [services, adapters]
confidence: inferred
---
# Pattern: NativeAudioEngine Abstraction over just_audio

<!-- vibeflow:auto:start -->
## What
`NativeAudioEngine` is a `core_platform` interface abstracting the real
audio engine (`just_audio`), consumed via `JustAudioPlaybackAdapter` in
`player_data`. Playback session state (position, queue) is persisted to the
backend, not just held in-memory.

## Where
`frontend/packages/core/core_platform/lib/src/audio_engine/
{native_audio_engine.dart, just_audio_native_audio_engine.dart}` and
`frontend/packages/data/player_data/lib/src/adapters/
just_audio_playback_adapter.dart`.

## The Pattern
- `JustAudioPlaybackAdapter` resolves and prefetches the next track (up to
  3 tracks deep) and pushes it to `NativeAudioEngine.setNextSource` so
  `just_audio` can warm/buffer it ahead of time.
- `JustAudioNativeAudioEngine.load` seeds a `ja.ConcatenatingAudioSource`
  (via the pure, unit-tested `buildInitialAudioSource`) rather than a
  plain source, specifically so `setNextSource`'s `current is
  ja.ConcatenatingAudioSource` check is actually true at runtime — see
  `.vibeflow/specs/gapless-playback-engine.md`. `setNextSource` still
  falls back to a best-effort no-op if the current source somehow isn't
  concatenating (e.g. before the first `load()`), but that's now a
  defensive edge case, not the permanent production state it used to be.

## Rules
- Prefetch/resolve logic belongs in `player_data` (adapter level); the
  engine-level `setNextSource` implementation stays a thin just_audio call.
- Any logic inside `JustAudioNativeAudioEngine` that doesn't strictly
  require a live platform channel (e.g. building a `ConcatenatingAudioSource`
  or other just_audio value object) should be extracted as a pure,
  top-level function — like `mapJustAudioProcessingState` and
  `buildInitialAudioSource` — specifically so it escapes the class's
  `coverage:ignore` exclusion and gets real unit test coverage.

## Examples from this codebase
File: `frontend/packages/core/core_platform/lib/src/audio_engine/just_audio_native_audio_engine.dart`
```dart
ja.ConcatenatingAudioSource buildInitialAudioSource(AudioSource source) {
  return ja.ConcatenatingAudioSource(
    children: [ja.AudioSource.uri(source.uri, headers: source.headers)],
  );
}

@override
Future<void> load(AudioSource source) async {
  await _player.setAudioSource(buildInitialAudioSource(source));
}

@override
Future<void> setNextSource(AudioSource? source) async {
  final current = _player.audioSource;
  if (current is ja.ConcatenatingAudioSource && source != null) {
    await current.add(ja.AudioSource.uri(source.uri, headers: source.headers));
  }
}
```
<!-- vibeflow:auto:end -->

## Anti-patterns (historical — resolved 2026-09-04)
Gapless playback was NOT actually functional end-to-end for a while,
despite earlier git history (`a217e63 fix(frontend): implement real audio
prefetch via setNextSource`) suggesting it was fixed: `load()` always
called `ja.AudioSource.uri(...)` — a single, non-concatenating source —
so `setNextSource`'s `current is ja.ConcatenatingAudioSource` check could
never be true, and `setNextSource` was a silent no-op at runtime despite
compiling and looking wired up. Fixed per
`.vibeflow/specs/gapless-playback-engine.md` — `load()` now seeds a real
`ConcatenatingAudioSource`, proven by a unit test that constructs one via
`buildInitialAudioSource` and calls `.add()` on it directly (no platform
channel involved: `.add()` only touches the real player once one has
actually attached the source via `setAudioSource`, which a hermetic unit
test never does).
