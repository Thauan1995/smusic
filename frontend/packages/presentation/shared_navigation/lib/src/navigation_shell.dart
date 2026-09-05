import 'package:core_design_system/core_design_system.dart';
import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:player_ui/player_ui.dart';
import 'package:social_proximity_ui/social_proximity_ui.dart';

class _NavDestination {
  const _NavDestination({
    required this.location,
    required this.icon,
    required this.selectedIcon,
    required this.label,
  });
  final String location;
  // Outlined (unselected) / filled (selected) - the same filled/outlined
  // rule as the play/pause controls, applied to nav destinations per
  // .vibeflow/specs/icon-system-consistency.md: this shell previously had
  // no selected-vs-unselected icon distinction at all (a single `icon`
  // reused for both NavigationDestination/NavigationRailDestination
  // states), a real usability gap against Spotify/YouTube Music's own
  // nav bars, which always signal "you are here" via icon style, not
  // just via highlight color.
  final IconData icon;
  final IconData selectedIcon;
  final String label;
}

const _destinations = [
  _NavDestination(
    location: '/library',
    icon: Icons.library_music_outlined,
    selectedIcon: Icons.library_music,
    label: 'Library',
  ),
  _NavDestination(
    location: '/search',
    icon: Icons.search_outlined,
    selectedIcon: Icons.search,
    label: 'Search',
  ),
  // frontend-flutter.md section 4.2 / task scope item 5: proximity gets its
  // own shell tab, not a submenu entry - the feature is meant to feel as
  // "always there" as library/search, per security.md 1.4's "acesso
  // rápido" spirit (even though the *toggle* itself lives in the corner
  // overlay below, the tab is what gets a user to the live list at all).
  _NavDestination(
    location: '/nearby',
    icon: Icons.wifi_tethering_outlined,
    selectedIcon: Icons.wifi_tethering,
    label: 'Perto',
  ),
];

/// Same `ShellRoute` body rendered as a bottom `NavigationBar` (compact
/// width) or a side `NavigationRail` (medium/expanded width), per
/// frontend-flutter.md section 3.5 - breakpoint-driven, never
/// platform-driven (section 1.3). Docks `MiniPlayerBar` above the nav
/// chrome, matching the common "always-visible playback bar" pattern.
///
/// Also docks `PauseDiscoveryToggle` (security.md 1.4: "toggle único e de
/// acesso rápido... não enterrado em configurações") as a small overlay in
/// the corner of every shell screen - task scope item 5's "acesso rápido a
/// 'pausar descoberta'" from the shell itself, not just from within
/// `ProximityListScreen`'s own app bar. It renders nothing
/// (`SizedBox.shrink()`) whenever the feature isn't enabled, so it is safe
/// to place unconditionally here without a separate visibility check.
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
        Expanded(
          // `PauseDiscoveryToggle` floats bottom-right of the content area
          // (a corner deliberately never used by this app's AppBars/FABs) -
          // *not* the top-right, which would sit directly over
          // `ProximityListScreen`'s own AppBar actions (including its
          // "open settings" button) and every other screen's AppBar
          // actions too.
          child: Stack(
            children: [
              child,
              Positioned(
                bottom: SmusicSpacing.sm,
                right: SmusicSpacing.sm,
                child: Material(
                  color: Theme.of(context).colorScheme.surfaceContainerHighest,
                  shape: const CircleBorder(),
                  elevation: 2,
                  child: const PauseDiscoveryToggle(),
                ),
              ),
            ],
          ),
        ),
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
              NavigationDestination(icon: Icon(d.icon), selectedIcon: Icon(d.selectedIcon), label: d.label),
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
                NavigationRailDestination(icon: Icon(d.icon), selectedIcon: Icon(d.selectedIcon), label: Text(d.label)),
            ],
          ),
          const VerticalDivider(width: 1),
          Expanded(child: body),
        ],
      ),
    );
  }
}
