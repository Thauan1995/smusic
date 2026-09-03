import 'package:auth_data/auth_data.dart';
import 'package:auth_domain/auth_domain.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_secure_storage_platform_interface/flutter_secure_storage_platform_interface.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../support/fake_secure_storage_platform.dart';

void main() {
  late FakeSecureStoragePlatform fakePlatform;
  late SecureTokenStorage storage;

  setUp(() {
    fakePlatform = FakeSecureStoragePlatform();
    FlutterSecureStoragePlatform.instance = fakePlatform;
    storage = SecureTokenStorage(storage: const FlutterSecureStorage());
  });

  test('read returns null when nothing was saved', () async {
    expect(await storage.read(), isNull);
  });

  test('save then read round-trips the tokens', () async {
    final tokens = AuthTokens(
      accessToken: 'a',
      refreshToken: 'r',
      accessTokenExpiresAt: DateTime(2026, 1, 1, 12),
    );
    await storage.save(tokens);

    final read = await storage.read();
    expect(read, isNotNull);
    expect(read!.accessToken, 'a');
    expect(read.refreshToken, 'r');
    expect(read.accessTokenExpiresAt, DateTime(2026, 1, 1, 12));
  });

  test('clear removes the stored value', () async {
    await storage.save(AuthTokens(
      accessToken: 'a',
      refreshToken: 'r',
      accessTokenExpiresAt: DateTime.now(),
    ));
    await storage.clear();
    expect(await storage.read(), isNull);
  });

  test('read treats corrupt stored JSON as no session and clears it', () async {
    await fakePlatform.write(
      key: 'smusic.auth_tokens',
      value: 'not json',
      options: const {},
    );

    expect(await storage.read(), isNull);
    expect(await fakePlatform.read(key: 'smusic.auth_tokens', options: const {}), isNull);
  });
}
