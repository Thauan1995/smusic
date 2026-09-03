import 'package:core_design_system/core_design_system.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  group('Breakpoints.classify', () {
    test('below compactMax is compact', () {
      expect(Breakpoints.classify(320), WindowSizeClass.compact);
      expect(Breakpoints.classify(599), WindowSizeClass.compact);
    });

    test('between compactMax and mediumMax is medium', () {
      expect(Breakpoints.classify(600), WindowSizeClass.medium);
      expect(Breakpoints.classify(1023), WindowSizeClass.medium);
    });

    test('at or above mediumMax is expanded', () {
      expect(Breakpoints.classify(1024), WindowSizeClass.expanded);
      expect(Breakpoints.classify(1920), WindowSizeClass.expanded);
    });
  });
}
