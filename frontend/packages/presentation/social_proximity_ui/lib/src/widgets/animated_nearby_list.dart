import 'package:flutter/material.dart';
import 'package:flutter_hooks/flutter_hooks.dart';
import 'package:social_proximity_domain/social_proximity_domain.dart';

import 'nearby_listener_card.dart';

/// frontend-flutter.md section 4.2: "Atualizações incrementais da lista
/// (chegada/saída de um ouvinte próximo) são animadas com
/// `AnimatedList`/`implicit animations`, não rebuild completo - reforça a
/// sensação de 'ao vivo'."
///
/// [listeners] is the *complete current snapshot* each time it changes
/// (per `ProximityFeedRepository.watch`'s doc comment - `nearby_update`/
/// `resync_full` both carry a full `users[]` array on the wire, not deltas)
/// - this widget is what turns that snapshot-replace stream into the
/// incremental insert/remove animations `AnimatedList` expects, by diffing
/// against what it last rendered (matched by [NearbyListener.userId]).
/// Reordering within an unchanged set is handled as a plain in-place swap
/// (no animation) rather than an animated move - proximity order changing
/// slightly between ticks is expected and not worth animating specially.
class AnimatedNearbyList extends HookWidget {
  const AnimatedNearbyList({super.key, required this.listeners});

  final List<NearbyListener> listeners;

  @override
  Widget build(BuildContext context) {
    final listKey = useMemoized(() => GlobalKey<AnimatedListState>());
    final current = useRef<List<NearbyListener>>(<NearbyListener>[]);
    final mounted = useRef<bool>(false);

    useEffect(() {
      if (!mounted.value) {
        // First frame: seed synchronously without animating (AnimatedList's
        // `initialItemCount` already covers this render) so we don't
        // double-insert what `initialItemCount` already accounts for.
        current.value = List.of(listeners);
        mounted.value = true;
        return null;
      }
      _applyDiff(current.value, listeners, listKey);
      return null;
    }, [listeners]);

    return AnimatedList(
      key: listKey,
      initialItemCount: listeners.length,
      itemBuilder: (context, index, animation) {
        final source = mounted.value ? current.value : listeners;
        if (index >= source.length) return const SizedBox.shrink();
        return _AnimatedItem(animation: animation, child: NearbyListenerCard(listener: source[index]));
      },
    );
  }

  static void _applyDiff(
    List<NearbyListener> current,
    List<NearbyListener> next,
    GlobalKey<AnimatedListState> listKey,
  ) {
    final nextIds = next.map((l) => l.userId).toSet();

    // Remove, back to front, so earlier indices stay valid.
    for (var i = current.length - 1; i >= 0; i--) {
      if (!nextIds.contains(current[i].userId)) {
        final removed = current.removeAt(i);
        listKey.currentState?.removeItem(
          i,
          (context, animation) => _AnimatedItem(animation: animation, child: NearbyListenerCard(listener: removed)),
          duration: const Duration(milliseconds: 250),
        );
      }
    }

    // Insert/update in target order.
    for (var i = 0; i < next.length; i++) {
      final listener = next[i];
      final existingIndex = current.indexWhere((l) => l.userId == listener.userId);
      if (existingIndex == -1) {
        current.insert(i, listener);
        listKey.currentState?.insertItem(i, duration: const Duration(milliseconds: 250));
      } else {
        current[existingIndex] = listener;
        if (existingIndex != i) {
          final moved = current.removeAt(existingIndex);
          current.insert(i, moved);
        }
      }
    }
  }
}

class _AnimatedItem extends StatelessWidget {
  const _AnimatedItem({required this.animation, required this.child});

  final Animation<double> animation;
  final Widget child;

  @override
  Widget build(BuildContext context) {
    return SizeTransition(
      sizeFactor: animation,
      child: FadeTransition(opacity: animation, child: child),
    );
  }
}
