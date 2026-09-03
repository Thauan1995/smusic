import 'package:core_design_system/core_design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter_hooks/flutter_hooks.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:library_domain/library_domain.dart';

import '../widgets/search_result_row.dart';

/// Catalog search (task scope item 4), debounced per
/// frontend-flutter.md section 3.3. The "instant local results" half of
/// that section is out of scope for Fatia 1 - see `search_notifier.dart`'s
/// doc comment in `library_domain` and frontend/README.md.
class SearchScreen extends HookConsumerWidget {
  const SearchScreen({super.key, this.onResultTap});

  final void Function(SearchResultItem item)? onResultTap;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final controller = useTextEditingController();
    final searchAsync = ref.watch(searchProvider);

    return Scaffold(
      appBar: AppBar(
        title: TextField(
          key: const Key('search_field'),
          controller: controller,
          autofocus: true,
          decoration: const InputDecoration(
            hintText: 'Search tracks, albums, artists, playlists',
            border: InputBorder.none,
          ),
          onChanged: (value) =>
              ref.read(searchProvider.notifier).onQueryChanged(value),
        ),
      ),
      body: searchAsync.when(
        // Riverpod's AsyncValue.when() defaults to skipLoadingOnRefresh:
        // true (show stale data instead of a loading UI while
        // "refreshing"). SearchNotifier's state always has previous data
        // attached after the first build (copyWithPrevious), so every
        // subsequent debounced search counts as a "refresh" - without this
        // override the skeleton would never actually show past the very
        // first query.
        skipLoadingOnRefresh: false,
        loading: () => const TrackListSkeleton(),
        error: (error, stackTrace) => EmptyState(
          message: 'Search failed. Please try again.',
          icon: Icons.error_outline,
          actionLabel: 'Retry',
          onAction: () =>
              ref.read(searchProvider.notifier).onQueryChanged(controller.text, debounce: Duration.zero),
        ),
        data: (state) {
          if (state.query.trim().isEmpty) {
            return const EmptyState(
              message: 'Search for a track, album, artist or playlist.',
              icon: Icons.search,
            );
          }
          if (state.result.items.isEmpty) {
            return EmptyState(message: 'No results for "${state.query}".', icon: Icons.search_off);
          }
          return ListView.builder(
            itemCount: state.result.items.length + (state.result.hasMore ? 1 : 0),
            itemBuilder: (context, index) {
              if (index == state.result.items.length) {
                return _LoadMoreRow(isLoading: state.isLoadingMore);
              }
              final item = state.result.items[index];
              return SearchResultRow(
                key: ValueKey('${item.type.name}-${item.id}'),
                item: item,
                onTap: onResultTap == null ? null : () => onResultTap!(item),
              );
            },
          );
        },
      ),
    );
  }
}

class _LoadMoreRow extends HookConsumerWidget {
  const _LoadMoreRow({required this.isLoading});

  final bool isLoading;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    if (isLoading) {
      return const Padding(
        padding: EdgeInsets.all(SmusicSpacing.md),
        child: Center(child: CircularProgressIndicator(strokeWidth: 2)),
      );
    }
    return Padding(
      padding: const EdgeInsets.all(SmusicSpacing.md),
      child: Center(
        child: TextButton(
          key: const Key('search_load_more_button'),
          onPressed: () => ref.read(searchProvider.notifier).loadMore(),
          child: const Text('Load more'),
        ),
      ),
    );
  }
}
