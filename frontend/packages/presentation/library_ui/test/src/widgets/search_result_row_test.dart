import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:library_domain/library_domain.dart';
import 'package:library_ui/library_ui.dart';

Future<void> _pump(WidgetTester tester, SearchResultItem item) {
  return tester.pumpWidget(MaterialApp(
    home: Scaffold(body: SearchResultRow(item: item)),
  ));
}

void main() {
  for (final type in SearchResultType.values) {
    testWidgets('renders an icon for $type', (tester) async {
      await _pump(
        tester,
        SearchResultItem(id: '1', type: type, title: 'X', subtitle: 'Y'),
      );
      expect(find.byType(Icon), findsOneWidget);
      expect(find.text('X'), findsOneWidget);
      expect(find.text('Y'), findsOneWidget);
    });
  }

  testWidgets('omits the subtitle widget when subtitle is empty', (tester) async {
    await _pump(
      tester,
      const SearchResultItem(id: '1', type: SearchResultType.track, title: 'X', subtitle: ''),
    );
    expect(find.text('X'), findsOneWidget);
    final tile = tester.widget<ListTile>(find.byType(ListTile));
    expect(tile.subtitle, isNull);
  });

  testWidgets('forwards tap to onTap', (tester) async {
    var tapped = false;
    await tester.pumpWidget(MaterialApp(
      home: Scaffold(
        body: SearchResultRow(
          item: const SearchResultItem(id: '1', type: SearchResultType.track, title: 'X', subtitle: 'Y'),
          onTap: () => tapped = true,
        ),
      ),
    ));
    await tester.tap(find.byType(SearchResultRow));
    expect(tapped, isTrue);
  });
}
