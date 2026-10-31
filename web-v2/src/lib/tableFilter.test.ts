import { describe, it, expect } from 'vitest';
import { optionsWithCounts, orderOptions, STATUS_ORDER } from './tableFilter';

describe('optionsWithCounts', () => {
  it('counts values and pins the given order ahead of the rest', () => {
    const got = optionsWithCounts(['zeta', 'failed', 'passed', 'failed', 'alpha'], STATUS_ORDER);
    expect(got).toEqual([
      { value: 'passed', count: 1 },
      { value: 'failed', count: 2 },
      { value: 'alpha', count: 1 },
      { value: 'zeta', count: 1 },
    ]);
  });

  it('is empty for no rows', () => {
    expect(optionsWithCounts([])).toEqual([]);
  });
});

describe('orderOptions', () => {
  it('keeps unknown values like "completed" after the pinned ones', () => {
    const got = orderOptions(
      [{ value: 'completed', count: 4 }, { value: 'failed', count: 1 }],
      STATUS_ORDER,
    );
    expect(got.map((o) => o.value)).toEqual(['failed', 'completed']);
  });
});
