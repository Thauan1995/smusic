import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';

import 'package:dio/dio.dart';

/// A minimal [HttpClientAdapter] that replays one canned result per call, in
/// order (the last entry repeats once the list is exhausted).
///
/// `http_mock_adapter`'s `onGet`/`onPost` callback runs exactly once, at
/// registration time - it cannot express "fail the first N calls, then
/// succeed", which is exactly the scenario the retry/refresh interceptors
/// need to be tested against. This adapter is a deliberately tiny
/// hand-rolled alternative for just those stateful cases.
class SequencedAdapter implements HttpClientAdapter {
  SequencedAdapter(this._responders);

  final List<FutureOr<ResponseBody> Function(RequestOptions options)>
      _responders;
  int callCount = 0;

  static ResponseBody jsonResponse(int statusCode, Object? data) {
    return ResponseBody.fromString(
      jsonEncode(data),
      statusCode,
      headers: {
        Headers.contentTypeHeader: [Headers.jsonContentType],
      },
    );
  }

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    final index =
        callCount < _responders.length ? callCount : _responders.length - 1;
    callCount++;
    return await _responders[index](options);
  }

  @override
  void close({bool force = false}) {}
}
