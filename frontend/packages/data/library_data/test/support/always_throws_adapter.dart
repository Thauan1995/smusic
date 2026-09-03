import 'dart:async';
import 'dart:typed_data';

import 'package:dio/dio.dart';

/// Always throws a connection error - used instead of `http_mock_adapter`'s
/// `DioAdapter` for tests that need `core_networking`'s `RetryInterceptor`
/// to actually retry a GET request: `DioAdapter.fetch()` does not tolerate
/// being re-entered (called again) while resolving an earlier request
/// against the same registered route, which is exactly what a retry does -
/// see core_networking's own `SequencedAdapter` test support for the first
/// occurrence of this issue.
class AlwaysThrowsConnectionErrorAdapter implements HttpClientAdapter {
  int callCount = 0;

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    callCount++;
    throw DioException.connectionError(requestOptions: options, reason: 'offline');
  }

  @override
  void close({bool force = false}) {}
}
