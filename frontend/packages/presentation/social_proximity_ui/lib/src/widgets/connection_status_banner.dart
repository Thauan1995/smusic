import 'dart:async';

import 'package:core_design_system/core_design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter_hooks/flutter_hooks.dart';
import 'package:social_proximity_domain/social_proximity_domain.dart';

/// frontend-flutter.md section 4.3: "um banner sutil indica 'reconectando…'
/// após ~5s desconectado" - never renders immediately on the first
/// disconnect blip (a socket hiccup that recovers in under [delay] would
/// otherwise flash the banner for no reason), and never lets the list look
/// silently stale: the moment [connectionState] is anything other than
/// [ProximityConnectionState.connected] for longer than [delay], this shows.
class ConnectionStatusBanner extends HookWidget {
  const ConnectionStatusBanner({
    super.key,
    required this.connectionState,
    this.delay = const Duration(seconds: 5),
  });

  final ProximityConnectionState connectionState;
  final Duration delay;

  @override
  Widget build(BuildContext context) {
    final visible = useState(false);

    useEffect(() {
      if (connectionState == ProximityConnectionState.connected) {
        visible.value = false;
        return null;
      }
      final timer = Timer(delay, () => visible.value = true);
      return timer.cancel;
    }, [connectionState]);

    if (!visible.value) return const SizedBox.shrink();

    final theme = Theme.of(context);
    final isOffline = connectionState == ProximityConnectionState.offline;

    return Material(
      key: const Key('proximity_connection_banner'),
      color: theme.colorScheme.surfaceContainerHighest,
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: SmusicSpacing.md, vertical: SmusicSpacing.sm),
        child: Row(
          children: [
            if (!isOffline)
              const SizedBox(width: 14, height: 14, child: CircularProgressIndicator(strokeWidth: 2))
            else
              Icon(Icons.cloud_off, size: 16, color: theme.colorScheme.onSurfaceVariant),
            const SizedBox(width: SmusicSpacing.sm),
            Expanded(
              child: Text(
                isOffline
                    ? 'Sem conexão com a descoberta por proximidade.'
                    : 'Reconectando…',
                key: const Key('proximity_connection_banner_text'),
                style: theme.textTheme.bodySmall,
              ),
            ),
          ],
        ),
      ),
    );
  }
}
