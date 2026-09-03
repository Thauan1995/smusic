import 'dart:async';

import 'package:auth_domain/auth_domain.dart';
import 'package:library_domain/library_domain.dart';
import 'package:player_domain/player_domain.dart';

class FakeAuthRepository implements AuthRepository {
  AuthSession? signUpResult;
  AuthSession? logInResult;
  AuthUser? currentUserResult;

  @override
  Future<AuthSession> signUp({
    required String email,
    required String password,
    required String displayName,
  }) async => signUpResult!;

  @override
  Future<AuthSession> logIn({required String email, required String password}) async =>
      logInResult!;

  @override
  Future<AuthTokens> refresh({required String refreshToken}) => throw UnimplementedError();

  @override
  Future<AuthUser> getCurrentUser() async => currentUserResult!;

  @override
  Future<void> logOut({required String refreshToken}) async {}
}

class FakeTokenStorage implements TokenStorage {
  AuthTokens? stored;

  @override
  Future<AuthTokens?> read() async => stored;

  @override
  Future<void> save(AuthTokens tokens) async => stored = tokens;

  @override
  Future<void> clear() async => stored = null;
}

class FakeLibraryRepository implements LibraryRepository {
  List<Playlist> playlists = [];
  Paginated<SearchResultItem> searchResult = const Paginated.empty();

  @override
  Future<List<Playlist>> getMyPlaylists() async => playlists;

  @override
  Future<String> createPlaylist({required String name, required bool isPublic}) =>
      throw UnimplementedError();

  @override
  Future<Paginated<SearchResultItem>> search({
    required String query,
    SearchResultType? type,
    int limit = 20,
    String? cursor,
  }) async => searchResult;

  @override
  Future<Track> getTrack(String trackId) => throw UnimplementedError();

  @override
  Future<Album> getAlbum(String albumId) => throw UnimplementedError();

  @override
  Future<void> addTrackToPlaylist({required String playlistId, required String trackId}) async {}

  @override
  Future<void> removeTrackFromPlaylist({required String playlistId, required String trackId}) async {}

  @override
  Future<void> saveTrack(String trackId) async {}
}

class FakePlaybackQueueController implements PlaybackQueueController {
  final StreamController<PlayerState> _stateController = StreamController.broadcast();
  final StreamController<QueueItem?> _nowPlayingController = StreamController.broadcast();

  @override
  Future<void> playFromQueue(List<QueueItem> queue, {required int startIndex}) async {}

  @override
  Future<void> pause() async {}

  @override
  Future<void> resume() async {}

  @override
  Future<void> skipNext() async {}

  @override
  Future<void> skipPrevious() async {}

  @override
  Future<void> seekTo(Duration position) async {}

  @override
  Stream<PlayerState> get stateStream => _stateController.stream;

  @override
  Stream<QueueItem?> get nowPlayingStream => _nowPlayingController.stream;

  Future<void> dispose() async {
    await _stateController.close();
    await _nowPlayingController.close();
  }
}
