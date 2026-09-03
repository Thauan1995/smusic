/// Generic cursor-paginated page, matching every `{ results[]/plays[],
/// next_cursor }` shape in backend-go.md section 4 (cursor-based, "não
/// offset - offset degrada em catálogos grandes").
class Paginated<T> {
  const Paginated({required this.items, required this.nextCursor});

  const Paginated.empty() : items = const [], nextCursor = null;

  final List<T> items;

  /// `null` means there are no more pages.
  final String? nextCursor;

  bool get hasMore => nextCursor != null;
}
