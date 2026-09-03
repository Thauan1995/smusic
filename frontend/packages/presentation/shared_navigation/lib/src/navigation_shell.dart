import 'package:core_design_system/core_design_system.dart';
import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:player_ui/player_ui.dart';

class _NavDestination {
  const _NavDestination({required this.location, required this.icon, required this.label});
  final String location;
  final IconData icon;
  final String label;
}

const _destinations = [
  _NavDestination(location: '/library', icon: Icons.library_music, label: 'Library'),
  _NavDestination(location: '/search', icon: Icons.search, label: 'Search'),
];

/// Same `ShellRoute` body rendered as a bottom `NavigationBar` (compact
/// width) or a side `NavigationRail` (medium/expanded width), per
/// frontend-flutter.md section 3.5 - breakpoint-driven, never
/// platform-driven (section 1.3). Docks `MiniPlayerBar` above the nav
/// chrome, matching the common "always-visible playback bar" pattern.
class NavigationShell extends StatelessWidget {
  const NavigationShell({super.key, required this.child, required this.currentLocation});

  final Widget child;
  final String currentLocation;

  int get _selectedIndex =>
      _destinations.indexWhere((d) => currentLocation.startsWith(d.location)).clamp(0, _destinations.length - 1);

  void _onDestinationSelected(BuildContext context, int index) {
    context.go(_destinations[index].location);
  }

  @override
  Widget build(BuildContext context) {
    final width = MediaQuery.sizeOf(context).width;
    final sizeClass = Breakpoints.classify(width);

    final body = Column(
      children: [
        Expanded(child: child),
        MiniPlayerBar(onExpand: () => context.push('/player')),
      ],
    );

    if (sizeClass == WindowSizeClass.compact) {
      return Scaffold(
        body: body,
        bottomNavigationBar: NavigationBar(
          selectedIndex: _selectedIndex,
          onDestinationSelected: (index) => _onDestinationSelected(context, index),
          destinations: [
            for (final d in _destinations)
              NavigationDestination(icon: Icon(d.icon), label: d.label),
          ],
        ),
      );
    }

    return Scaffold(
      body: Row(
        children: [
          NavigationRail(
            extended: sizeClass == WindowSizeClass.expanded,
            selectedIndex: _selectedIndex,
            onDestinationSelected: (index) => _onDestinationSelected(context, index),
            destinations: [
              for (final d in _destinations)
                NavigationRailDestination(icon: Icon(d.icon), label: Text(d.label)),
            ],
          ),
          const VerticalDivider(width: 1),
          Expanded(child: body),
        ],
      ),
    );
  }
}
