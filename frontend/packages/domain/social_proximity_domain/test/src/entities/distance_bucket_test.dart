import 'package:social_proximity_domain/social_proximity_domain.dart';
import 'package:test/test.dart';

void main() {
  test('has exactly the 4 security.md 1.2 buckets with their Portuguese labels', () {
    expect(DistanceBucket.values, hasLength(4));
    expect(DistanceBucket.veryClose.label, 'Bem pertinho');
    expect(DistanceBucket.neighborhood.label, 'No seu bairro');
    expect(DistanceBucket.region.label, 'Na sua região');
    expect(DistanceBucket.city.label, 'Na sua cidade');
  });

  test('no bucket label contains a digit (never a numeric distance)', () {
    for (final bucket in DistanceBucket.values) {
      expect(RegExp(r'\d').hasMatch(bucket.label), isFalse, reason: bucket.label);
    }
  });
}
