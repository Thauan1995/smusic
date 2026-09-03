import 'package:social_proximity_domain/social_proximity_domain.dart';
import 'package:test/test.dart';

void main() {
  test('equality/hashCode compare trackTitle and artistName', () {
    // Not `const` - a canonicalized const literal never shows as "hit" by
    // line coverage tooling even though the constructor genuinely ran (see
    // frontend/README.md's methodological note).
    // ignore: prefer_const_constructors
    final a = NowPlayingSnapshot(trackTitle: 'Song', artistName: 'Artist');
    // ignore: prefer_const_constructors
    final b = NowPlayingSnapshot(trackTitle: 'Song', artistName: 'Artist');
    // ignore: prefer_const_constructors
    final c = NowPlayingSnapshot(trackTitle: 'Other');

    expect(a, b);
    expect(a.hashCode, b.hashCode);
    expect(a, isNot(c));
    expect(c.artistName, isNull);
  });
}
