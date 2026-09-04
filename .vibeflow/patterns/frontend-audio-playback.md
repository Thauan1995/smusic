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
- `JustAudioNativeAudioEngine.setNextSource` only actually queues the
  prefetched source if the *currently loaded* source is a
  `just_audio.ConcatenatingAudioSource`; otherwise it is a documented
  best-effort no-op ("gapless is an optimization, not a correctness
  requirement for Fatia 1").

## Rules
- Prefetch/resolve logic belongs in `player_data` (adapter level); the
  engine-level `setNextSource` implementation stays a thin just_audio call.

## Examples from this codebase
File: `frontend/packages/core/core_platform/lib/src/audio_engine/just_audio_native_audio_engine.dart`
```dart
@override
Future<void> load(AudioSource source) async {
  await _player.setAudioSource(ja.AudioSource.uri(source.uri, headers: source.headers));
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

## Anti-patterns
**Gapless playback is NOT actually functional end-to-end, despite git
history (`a217e63 fix(frontend): implement real audio prefetch via
setNextSource`) suggesting it was fixed.** `load()` always calls
`ja.AudioSource.uri(...)` — a single, non-concatenating source — and
nothing in this codebase ever constructs a `ConcatenatingAudioSource`.
Therefore the `current is ja.ConcatenatingAudioSource` check in
`setNextSource` can never be true in practice, and `setNextSource` remains
a silent no-op at runtime. This confirms
`docs/architecture/00-overview.md`'s recorded tech debt item #1 is still
accurate: "`JustAudioNativeEngine.setNextSource` é no-op em produção —
falta `ConcatenatingAudioSource`" — the prefetch/resolve half was
implemented, but the engine-level wiring to make gapless transitions
actually happen was not. This is a real gap against the YouTube
Music/Spotify playback-quality bar the project's Auditor is supposed to
enforce.
