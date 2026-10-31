import { describe, it, expect } from 'vitest';
import { nextSort, sortRows } from './tableSort';

describe('nextSort', () => {
  it('cycles one column asc → desc → unsorted', () => {
    const a = nextSort(null, 'name');
    expect(a).toEqual({ key: 'name', dir: 'asc' });
    const b = nextSort(a, 'name');
    expect(b).toEqual({ key: 'name', dir: 'desc' });
    expect(nextSort(b, 'name')).toBeNull();
  });

  it('starts a different column at asc', () => {
    expect(nextSort({ key: 'name', dir: 'desc' }, 'age')).toEqual({ key: 'age', dir: 'asc' });
  });
});

describe('sortRows', () => {
  const rows = [
    { n: 'b10', v: 3, d: '2026-01-02' },
    { n: 'b2', v: null, d: '' },
    { n: 'a', v: 1, d: '2026-01-03' },
  ];
  const acc = {
    n: (r: (typeof rows)[number]) => r.n,
    v: (r: (typeof rows)[number]) => r.v,
    d: (r: (typeof rows)[number]) => (r.d ? new Date(r.d) : null),
  };

  it('returns the natural order when unsorted', () => {
    expect(sortRows(rows, null, acc).map((r) => r.n)).toEqual(['b10', 'b2', 'a']);
  });

  it('sorts strings numerically-aware', () => {
    expect(sortRows(rows, { key: 'n', dir: 'asc' }, acc).map((r) => r.n)).toEqual(['a', 'b2', 'b10']);
  });

  it('sinks missing values in both directions', () => {
    expect(sortRows(rows, { key: 'v', dir: 'asc' }, acc).map((r) => r.v)).toEqual([1, 3, null]);
    expect(sortRows(rows, { key: 'v', dir: 'desc' }, acc).map((r) => r.v)).toEqual([3, 1, null]);
  });

  it('sorts dates', () => {
    expect(sortRows(rows, { key: 'd', dir: 'desc' }, acc).map((r) => r.n)).toEqual(['a', 'b10', 'b2']);
  });

  it('does not mutate the input', () => {
    const copy = [...rows];
    sortRows(rows, { key: 'n', dir: 'asc' }, acc);
    expect(rows).toEqual(copy);
  });
});
