import 'dart:async';
import 'dart:typed_data';

import 'package:dio/dio.dart';

/// Copied from `core_networking`'s test-support helper of the same name
/// (each `*_data` package owns its own test doubles, per this monorepo's
/// convention - see e.g. `player_data`/`player_domain` each having their
/// own `fake_playback_session_repository.dart`).
///
/// A minimal [HttpClientAdapter] that replays one canned result per call, in
/// order (the last entry repeats once the list is exhausted) - needed for
/// `RetryInterceptor`-exercising tests, where `http_mock_adapter`'s
/// `onGet`/`onPost` (a single callback run once, at registration time)
/// cannot express "every attempt fails the same connection-level way",
/// which caused GET-network-failure tests to hang consuming real
/// `RetryInterceptor`-driven `Future.delayed` retries instead of a
/// deterministic single failure.
class SequencedAdapter implements HttpClientAdapter {
  SequencedAdapter(this._responders);

  final List<FutureOr<ResponseBody> Function(RequestOptions options)> _responders;
  int callCount = 0;

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    final index = callCount < _responders.length ? callCount : _responders.length - 1;
    callCount++;
    return _responders[index](options);
  }

  @override
  void close({bool force = false}) {}
}
