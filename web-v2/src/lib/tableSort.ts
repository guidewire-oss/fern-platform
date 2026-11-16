// Shared sort state for clickable table headers.
//
// A header click cycles one column through three states: ascending,
// descending, then back to unsorted (the table's natural order). Clicking
// a different column starts that column at ascending.

export type SortDir = 'asc' | 'desc';

export interface SortState<K extends string = string> {
  key: K;
  dir: SortDir;
}

export function nextSort<K extends string>(
  current: SortState<K> | null,
  key: K,
): SortState<K> | null {
  if (!current || current.key !== key) return { key, dir: 'asc' };
  if (current.dir === 'asc') return { key, dir: 'desc' };
  return null;
}

export type SortValue = string | number | Date | null | undefined;

const collator = new Intl.Collator(undefined, { numeric: true, sensitivity: 'base' });

function compareValues(a: SortValue, b: SortValue): number {
  if (a instanceof Date) a = a.getTime();
  if (b instanceof Date) b = b.getTime();
  if (typeof a === 'number' && typeof b === 'number') return a - b;
  return collator.compare(String(a), String(b));
}

function isMissing(v: SortValue): boolean {
  return v == null || v === '' || (typeof v === 'number' && Number.isNaN(v));
}

// sortRows returns a sorted copy of `rows`. `accessors` maps each sort
// key to the value it orders by. Missing values (null, '', NaN) sink to
// the bottom in both directions so an "—" cell never leads the list.
// Ties keep their original relative order (Array.prototype.sort is
// stable), so the natural order is the tiebreak.
export function sortRows<T, K extends string>(
  rows: readonly T[],
  sort: SortState<K> | null,
  accessors: Record<K, (row: T) => SortValue>,
): T[] {
  if (!sort) return [...rows];
  const get = accessors[sort.key];
  const dir = sort.dir === 'asc' ? 1 : -1;
  return [...rows].sort((ra, rb) => {
    const a = get(ra);
    const b = get(rb);
    const ma = isMissing(a);
    const mb = isMissing(b);
    if (ma || mb) return ma === mb ? 0 : ma ? 1 : -1;
    return compareValues(a, b) * dir;
  });
}
