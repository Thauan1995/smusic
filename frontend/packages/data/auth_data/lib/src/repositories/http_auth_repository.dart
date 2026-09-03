import 'package:auth_domain/auth_domain.dart';
import 'package:core_networking/core_networking.dart';

import '../dto/auth_dtos.dart';

/// [AuthRepository] implementation against backend-go.md section 4's auth
/// endpoints.
///
/// ASSUMPTION (backend contract gap): the spec's illustrative
/// `POST /v1/auth/signup` request shape (`{ email, password | oauth_token
/// }`) doesn't list `display_name`, yet `GET /v1/auth/me`'s response
/// includes one - a display name has to originate somewhere. This
/// repository sends `display_name` in the signup body; if the backend
/// ignores unknown fields (Go decoders using `json.Decoder` without
/// `DisallowUnknownFields`, or explicit strict allow-list) this is a no-op
/// on the server. Flagged for the backend specialist per
/// frontend/README.md.
class HttpAuthRepository implements AuthRepository {
  HttpAuthRepository(this._client);

  final ApiClient _client;

  @override
  Future<AuthSession> signUp({
    required String email,
    required String password,
    required String displayName,
  }) async {
    return _wrap(() async {
      final response = await _client.post(
        '/v1/auth/signup',
        data: {
          'email': email,
          'password': password,
          'display_name': displayName,
        },
        skipAuth: true,
      );
      final tokens = AuthDtos.tokensFromLoginResponse(response);
      final user = AuthUser(
        userId: response['user_id'] as String,
        displayName: displayName,
        email: email,
      );
      return AuthSession(user: user, tokens: tokens);
    });
  }

  @override
  Future<AuthSession> logIn({
    required String email,
    required String password,
  }) async {
    return _wrap(() async {
      final response = await _client.post(
        '/v1/auth/login',
        data: {'email': email, 'password': password},
        skipAuth: true,
      );
      final tokens = AuthDtos.tokensFromLoginResponse(response);
      final me = await _client.get('/v1/auth/me');
      final user = AuthDtos.userFromMeResponse(me, fallbackEmail: email);
      return AuthSession(user: user, tokens: tokens);
    });
  }

  @override
  Future<AuthTokens> refresh({required String refreshToken}) async {
    return _wrap(() async {
      final response = await _client.post(
        '/v1/auth/refresh',
        data: {'refresh_token': refreshToken},
        skipAuth: true,
      );
      return AuthDtos.tokensFromLoginResponse(
        response,
        fallbackRefreshToken: refreshToken,
      );
    });
  }

  @override
  Future<AuthUser> getCurrentUser() async {
    return _wrap(() async {
      final response = await _client.get('/v1/auth/me');
      return AuthDtos.userFromMeResponse(response);
    });
  }

  @override
  Future<void> logOut({required String refreshToken}) async {
    return _wrap(() async {
      await _client.post('/v1/auth/logout', data: {'refresh_token': refreshToken});
    });
  }

  Future<T> _wrap<T>(Future<T> Function() body) async {
    try {
      return await body();
    } on ApiException catch (e) {
      throw AuthException(_kindFor(e), message: e.message);
    }
  }

  AuthExceptionKind _kindFor(ApiException e) {
    if (e.isUnauthorized) return AuthExceptionKind.invalidCredentials;
    if (e.statusCode == 409) return AuthExceptionKind.emailAlreadyInUse;
    if (e.isNetworkError) return AuthExceptionKind.network;
    return AuthExceptionKind.unknown;
  }
}
