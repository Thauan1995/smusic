---
tags: [dio, http, interceptors, retry, auth, api-client]
modules: [frontend/packages/core/core_networking/]
applies_to: [services, interceptors]
confidence: inferred
---
# Pattern: Shared ApiClient with Auth/Retry Interceptors

<!-- vibeflow:auto:start -->
## What
A single `ApiClient` (wrapping `dio`) shared by every `*_data` package,
with `AuthInterceptor` (bearer token attach + 401 refresh-and-retry-once)
and `RetryInterceptor` (idempotent-GET retry) composed on top.

## Where
`frontend/packages/core/core_networking/lib/src/api_client.dart` and
`interceptors/*.dart`. Consumed by every `*_data` repository
(`HttpAuthRepository`, `HttpLibraryRepository`,
`HttpProximityPrivacySettingsRepository`, etc.) via constructor injection.

## The Pattern
- `ApiClient` exposes typed `get/post/put/delete` methods returning
  `Map<String, dynamic>`, translating any `DioException` into a domain-level
  `ApiException` (network vs. server-message-carrying).
- Auth is opt-out per request via `options.extra['skipAuth'] = true`
  (used only for `/v1/auth/signup` and `/v1/auth/login`), not opt-in —
  every other call is authenticated by default.
- On a 401, `AuthInterceptor` calls `tokenSource.refreshAccessToken()` and
  retries the original request exactly once (guarded by an
  `authRetried` flag in `RequestOptions.extra` to prevent infinite loops).
  The retried request is a *copy* of the original `RequestOptions`
  (`copyWith`), never a mutate-in-place reuse.

## Rules
- Never call `_dio` directly from a `*_data` repository — go through
  `ApiClient`'s typed methods so auth/retry/error-mapping stays uniform.
- Any request that must skip auth needs an explicit `skipAuth: true` — this
  is a conscious opt-out, not a default.
- A 401 retry must set `authRetried` in `extra` before resending, or it
  risks an infinite retry loop against a still-invalid token.

## Examples from this codebase
File: `frontend/packages/core/core_networking/lib/src/api_client.dart`
```dart
class ApiClient {
  ApiClient({required String baseUrl, AuthTokenSource tokenSource = const NoAuthTokenSource(), Dio? dio})
      : _dio = dio ?? Dio(BaseOptions(baseUrl: baseUrl, connectTimeout: const Duration(seconds: 10), ...)) {
    _dio.interceptors.addAll([AuthInterceptor(tokenSource, _dio), RetryInterceptor(_dio)]);
  }
  Future<Map<String, dynamic>> get(String path, {..., bool skipAuth = false}) async { ... }
}
```

File: `frontend/packages/core/core_networking/lib/src/interceptors/auth_interceptor.dart`
```dart
if (response?.statusCode == 401 && !alreadyRetried && !skipAuth) {
  final newToken = await _tokenSource.refreshAccessToken();
  if (newToken != null) {
    final retryOptions = err.requestOptions.copyWith(
      headers: {...err.requestOptions.headers, 'Authorization': 'Bearer $newToken'},
      extra: {...err.requestOptions.extra, 'authRetried': true},
    );
    final retryResponse = await _dio.fetch<dynamic>(retryOptions);
    handler.resolve(retryResponse);
    return;
  }
}
```
<!-- vibeflow:auto:end -->
