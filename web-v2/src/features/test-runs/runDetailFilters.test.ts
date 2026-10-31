import { describe, it, expect } from 'vitest';
import {
  EMPTY_ROW_FILTER,
  filterRows,
  isRowFilterActive,
  sortSpecs,
  tagLabel,
} from './runDetailFilters';

const tag = (name: string, category: string | null = null, value: string | null = null) => ({
  name,
  category,
  value,
});

const specs = [
  { specName: 'logs in', status: 'passed', duration: 30, errorMessage: null, startTime: '2026-01-01T00:00:02Z', tags: [tag('smoke')] },
  { specName: 'logs out', status: 'failed', duration: 10, errorMessage: 'Timeout waiting', startTime: '2026-01-01T00:00:01Z', tags: [tag('jira:GWCP-1', 'jira', 'GWCP-1')] },
  { specName: 'resets', status: 'skipped', duration: 0, errorMessage: null, startTime: '', tags: [] },
];
const text = (s: (typeof specs)[number]) => [s.specName, s.errorMessage];

describe('tagLabel', () => {
  it('renders a raw category:value name as "category: value"', () => {
    expect(tagLabel(tag('jira:GWCP-1', 'jira', 'GWCP-1'))).toBe('jira: GWCP-1');
  });
  it('appends value to a distinct name', () => {
    expect(tagLabel(tag('team', 'owner', 'platform'))).toBe('team: platform');
  });
  it('is just the name without a value', () => {
    expect(tagLabel(tag('smoke'))).toBe('smoke');
  });
});

describe('filterRows', () => {
  it('passes everything through the empty filter', () => {
    expect(filterRows(specs, EMPTY_ROW_FILTER, text)).toHaveLength(3);
  });

  it('hides skipped', () => {
    const got = filterRows(specs, { ...EMPTY_ROW_FILTER, hideSkipped: true }, text);
    expect(got.map((s) => s.specName)).toEqual(['logs in', 'logs out']);
  });

  it('keeps only selected statuses', () => {
    const got = filterRows(specs, { ...EMPTY_ROW_FILTER, statuses: ['failed'] }, text);
    expect(got.map((s) => s.specName)).toEqual(['logs out']);
  });

  it('matches any selected tag by label', () => {
    const got = filterRows(specs, { ...EMPTY_ROW_FILTER, tags: ['jira: GWCP-1', 'smoke'] }, text);
    expect(got).toHaveLength(2);
  });

  it('ignores selected tags that no row carries', () => {
    const got = filterRows(specs, { ...EMPTY_ROW_FILTER, tags: ['from-another-suite'] }, text);
    expect(got).toHaveLength(3);
  });

  it('searches name and error text case-insensitively', () => {
    const got = filterRows(specs, { ...EMPTY_ROW_FILTER, q: 'TIMEOUT' }, text);
    expect(got.map((s) => s.specName)).toEqual(['logs out']);
  });
});

describe('isRowFilterActive', () => {
  it('is false for the empty filter and true once anything is set', () => {
    expect(isRowFilterActive(EMPTY_ROW_FILTER)).toBe(false);
    expect(isRowFilterActive({ ...EMPTY_ROW_FILTER, q: '  ' })).toBe(false);
    expect(isRowFilterActive({ ...EMPTY_ROW_FILTER, hideSkipped: true })).toBe(true);
  });
});

describe('sortSpecs', () => {
  it('sorts by duration and by start time with missing times last', () => {
    expect(sortSpecs(specs, { key: 'duration', dir: 'asc' }).map((s) => s.duration)).toEqual([0, 10, 30]);
    expect(sortSpecs(specs, { key: 'started', dir: 'asc' }).map((s) => s.specName)).toEqual([
      'logs out',
      'logs in',
      'resets',
    ]);
  });

  it('puts specs with an error first when sorting by error desc', () => {
    expect(sortSpecs(specs, { key: 'error', dir: 'desc' })[0]!.specName).toBe('logs out');
  });
});
