import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:library_domain/library_domain.dart';
import 'package:library_ui/library_ui.dart';

void main() {
  testWidgets('renders name and public/private subtitle, forwards tap', (tester) async {
    var tapped = false;
    await tester.pumpWidget(MaterialApp(
      home: Scaffold(
        body: PlaylistRow(
          playlist: const Playlist(id: '1', name: 'Chill', isPublic: true),
          onTap: () => tapped = true,
        ),
      ),
    ));

    expect(find.text('Chill'), findsOneWidget);
    expect(find.text('Public playlist'), findsOneWidget);

    await tester.tap(find.byType(PlaylistRow));
    expect(tapped, isTrue);
  });

  testWidgets('shows Private playlist subtitle', (tester) async {
    await tester.pumpWidget(const MaterialApp(
      home: Scaffold(
        body: PlaylistRow(playlist: Playlist(id: '1', name: 'Chill', isPublic: false)),
      ),
    ));
    expect(find.text('Private playlist'), findsOneWidget);
  });
}
