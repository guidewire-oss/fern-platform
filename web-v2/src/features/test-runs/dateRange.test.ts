import { describe, it, expect } from 'vitest';
import { customRangeToQuery, queryToRangeInputs } from './dateRange';

describe('customRangeToQuery', () => {
  it('maps both dates to UTC day-start / day-end bounds', () => {
    expect(customRangeToQuery('2026-07-01', '2026-07-14')).toEqual({
      from: '2026-07-01T00:00:00.000Z',
      to: '2026-07-14T23:59:59.999Z',
    });
  });

  it('includes the whole day for a single-day range', () => {
    const q = customRangeToQuery('2026-07-14', '2026-07-14');
    expect(q.from).toBe('2026-07-14T00:00:00.000Z');
    expect(q.to).toBe('2026-07-14T23:59:59.999Z');
  });

  it('omits an unset bound rather than emitting undefined-ish strings', () => {
    expect(customRangeToQuery('2026-07-01', '')).toEqual({ from: '2026-07-01T00:00:00.000Z' });
    expect(customRangeToQuery('', '2026-07-14')).toEqual({ to: '2026-07-14T23:59:59.999Z' });
    expect(customRangeToQuery('', '')).toEqual({});
  });
});

describe('queryToRangeInputs', () => {
  it('recovers YYYY-MM-DD input values from ISO bounds', () => {
    expect(queryToRangeInputs('2026-07-01T00:00:00.000Z', '2026-07-14T23:59:59.999Z')).toEqual({
      fromDate: '2026-07-01',
      toDate: '2026-07-14',
    });
  });

  it('yields empty strings for unset bounds', () => {
    expect(queryToRangeInputs(undefined, undefined)).toEqual({ fromDate: '', toDate: '' });
  });

  it('round-trips with customRangeToQuery', () => {
    const q = customRangeToQuery('2026-01-05', '2026-02-10');
    expect(queryToRangeInputs(q.from, q.to)).toEqual({ fromDate: '2026-01-05', toDate: '2026-02-10' });
  });
});
