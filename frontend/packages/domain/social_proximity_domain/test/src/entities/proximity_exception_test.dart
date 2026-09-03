import 'package:social_proximity_domain/social_proximity_domain.dart';
import 'package:test/test.dart';

void main() {
  test('toString includes kind and optional message', () {
    // Not `const` - a canonicalized const literal never shows as "hit" by
    // line coverage tooling even though the constructor genuinely ran (see
    // frontend/README.md's methodological note).
    // ignore: prefer_const_constructors
    final withMessage = ProximityException(ProximityExceptionKind.network, message: 'timeout');
    expect(withMessage.toString(), 'ProximityException(network: timeout)');

    // ignore: prefer_const_constructors
    final withoutMessage = ProximityException(ProximityExceptionKind.unauthorized);
    expect(withoutMessage.toString(), 'ProximityException(unauthorized)');
  });
}
