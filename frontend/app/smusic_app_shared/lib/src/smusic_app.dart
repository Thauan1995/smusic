import 'package:core_design_system/core_design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'app_router_provider.dart';

/// The app root, shared verbatim by `smusic_mobile` and `smusic_web`
/// (frontend-flutter.md section 1.3). Both entrypoints' `main.dart` do
/// nothing but build a `ProviderScope` with platform-specific overrides
/// (`core_platform` implementations) and mount this widget - see
/// frontend/README.md for the full wiring.
class SmusicApp extends ConsumerWidget {
  const SmusicApp({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final router = ref.watch(appRouterProvider);
    return MaterialApp.router(
      title: 'smusic',
      debugShowCheckedModeBanner: false,
      theme: SmusicTheme.light(),
      darkTheme: SmusicTheme.dark(),
      routerConfig: router,
    );
  }
}
