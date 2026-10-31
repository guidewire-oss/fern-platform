import type { FilterOption } from '@/components/ui/FilterChips';

// Workflow order for status filter chips: green states first, then
// degraded, then in-flight. Matches the test-runs filter sidebar.
export const STATUS_ORDER = ['passed', 'failed', 'flaky', 'skipped', 'running'] as const;

// optionsWithCounts builds chip options from the values present in a
// set of rows, with how many rows carry each.
export function optionsWithCounts(
  values: readonly string[],
  order: readonly string[] = [],
): FilterOption[] {
  const counts = new Map<string, number>();
  for (const v of values) counts.set(v, (counts.get(v) ?? 0) + 1);
  return orderOptions(
    [...counts.entries()].map(([value, count]) => ({ value, count })),
    order,
  );
}

// orderOptions pins known values (e.g. the workflow status order) ahead
// of the rest, which sort alphabetically. Use it directly when counts
// already come from the server, such as a facet.
export function orderOptions(
  options: readonly FilterOption[],
  order: readonly string[] = [],
): FilterOption[] {
  const rank = (v: string) => {
    const i = order.indexOf(v);
    return i === -1 ? order.length : i;
  };
  return [...options].sort((a, b) => rank(a.value) - rank(b.value) || a.value.localeCompare(b.value));
}
