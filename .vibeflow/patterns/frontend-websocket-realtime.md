---
tags: [websocket, realtime, reconnect, backoff, presence]
modules: [frontend/packages/core/core_networking/, frontend/packages/data/social_proximity_data/]
applies_to: [services, repositories]
confidence: inferred
---
# Pattern: Generic Reconnecting WebSocket Client

<!-- vibeflow:auto:start -->
## What
A protocol-agnostic `ReconnectingWebSocketClient` in `core_networking`
(not tied to the presence feature), used today by
`WebSocketProximityFeedRepository` and designed to be reused by any future
realtime stream.

## Where
`frontend/packages/core/core_networking/lib/src/websocket/
reconnecting_websocket_client.dart` (client + backoff calculation) and
`frontend/packages/data/social_proximity_data/lib/src/repositories/
web_socket_proximity_feed_repository.dart` (protocol-specific consumer).

## The Pattern
- Exponential backoff with jitter: `computeReconnectBackoff` doubles the
  delay per failed attempt, caps at `maxBackoff` (default 30s), and applies
  ±20% multiplicative jitter — extracted as a pure, independently
  unit-testable function (no `Random`/timer coupling needed for tests that
  just check the math).
- The attempt counter resets to 0 after *any* successful connection (even
  one that drops immediately) — only consecutive failed *connection
  attempts* grow the backoff, not disconnects of an established session.
- A `_generation` counter invalidates in-flight connect loops from a
  previous `start()`/`stop()` cycle, so a stale loop can't resurrect state
  after being superseded.
- Raw messages are republished undecoded on `messages` — JSON
  decoding/frame dispatch is left entirely to the caller, keeping this
  class protocol-agnostic.
- `send()` silently drops while disconnected — callers needing delivery
  guarantees must check `isRunning`/observe `connectionPhase` themselves.

## Rules
- Do not add protocol-specific logic (frame types, feature entities) to
  `ReconnectingWebSocketClient` — that belongs in the feature's `*_data`
  repository, which maps `SocketConnectionPhase` to its own domain-facing
  connection-state enum.
- `uriBuilder` stays synchronous (`Uri Function()`) by design — see
  `smusic_mobile/main.dart`'s `buildPresenceSocketClient` for how a caller
  bridges an async token source (30s-refreshed in-memory cache) into that
  constraint without changing this shared, tested class.

## Examples from this codebase
File: `frontend/packages/core/core_networking/lib/src/websocket/reconnecting_websocket_client.dart`
```dart
Duration computeReconnectBackoff(int attempt, {required Duration initial, required Duration max, required Random random}) {
  final rawMs = initial.inMilliseconds * pow(2, attempt);
  final cappedMs = min(rawMs.toDouble(), max.inMilliseconds.toDouble());
  final jitterFactor = 0.8 + random.nextDouble() * 0.4; // [0.8, 1.2)
  return Duration(milliseconds: (cappedMs * jitterFactor).round().clamp(0, max.inMilliseconds));
}
```

File: `frontend/app/smusic_mobile/lib/main.dart` — bridging an async token source
```dart
ReconnectingWebSocketClient buildPresenceSocketClient({required String apiBaseUrl, required AuthTokenSource tokenSource}) {
  String? cachedToken;
  Future<void> refreshCachedToken() async { cachedToken = await tokenSource.currentAccessToken(); }
  unawaited(refreshCachedToken());
  Timer.periodic(const Duration(seconds: 30), (_) => unawaited(refreshCachedToken()));
  return ReconnectingWebSocketClient(uriBuilder: () => buildPresenceUri(apiBaseUrl: apiBaseUrl, accessToken: cachedToken));
}
```
<!-- vibeflow:auto:end -->
