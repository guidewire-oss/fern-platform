// Sort and filter logic for the run-detail suites and specs tables.
//
// Both tables render data the run query has already loaded in full, so
// sorting and filtering happen client-side. Kept out of the component
// file so it can be unit-tested and fast refresh keeps working.

import { sortRows, type SortState, type SortValue } from '@/lib/tableSort';

export interface TagLike {
  name: string;
  category: string | null;
  value: string | null;
}

// tagLabel is how a tag reads in a chip. `name` is sometimes the raw,
// unprocessed "category:value" string (e.g. Ginkgo labels reported
// as-is) and sometimes a distinct human label with category/value
// tracked separately (e.g. "team" / owner / platform). Only substitute
// in "category: value" when name is exactly that raw compound —
// otherwise appending value to name would double it up.
export function tagLabel(tag: TagLike): string {
  if (!tag.value) return tag.name;
  if (tag.name === `${tag.category}:${tag.value}`) return `${tag.category}: ${tag.value}`;
  return `${tag.name}: ${tag.value}`;
}

export interface RowFilter {
  q: string;
  statuses: string[];
  // Tag labels; a row matches when it carries any of them.
  tags: string[];
  hideSkipped: boolean;
}

export const EMPTY_ROW_FILTER: RowFilter = { q: '', statuses: [], tags: [], hideSkipped: false };

interface FilterableRow {
  status: string;
  tags: TagLike[];
}

// filterRows applies a RowFilter. `text` returns the strings the search
// box matches against (case-insensitive substring). Selected tags that
// no row carries are ignored, so a tag picked on one suite doesn't
// empty the table when the user moves to a suite without it.
export function filterRows<T extends FilterableRow>(
  rows: readonly T[],
  f: RowFilter,
  text: (row: T) => (string | null | undefined)[],
): T[] {
  const q = f.q.trim().toLowerCase();
  const present = new Set(rows.flatMap((r) => r.tags.map(tagLabel)));
  const tags = f.tags.filter((t) => present.has(t));
  return rows.filter((r) => {
    if (f.hideSkipped && r.status === 'skipped') return false;
    if (f.statuses.length > 0 && !f.statuses.includes(r.status)) return false;
    if (tags.length > 0 && !r.tags.some((t) => tags.includes(tagLabel(t)))) return false;
    if (q && !text(r).some((s) => s?.toLowerCase().includes(q))) return false;
    return true;
  });
}

export function isRowFilterActive(f: RowFilter): boolean {
  return f.q.trim() !== '' || f.statuses.length > 0 || f.tags.length > 0 || f.hideSkipped;
}

// --- Suites ---------------------------------------------------------

export type SuiteSortKey = 'name' | 'failed' | 'status' | 'duration';

export interface SuiteRowLike extends FilterableRow {
  suiteName: string;
  failedSpecs: number;
  duration: number;
}

const SUITE_ACCESSORS: Record<SuiteSortKey, (s: SuiteRowLike) => SortValue> = {
  name: (s) => s.suiteName,
  failed: (s) => s.failedSpecs,
  status: (s) => s.status,
  duration: (s) => s.duration,
};

export function sortSuites<T extends SuiteRowLike>(rows: readonly T[], sort: SortState<SuiteSortKey> | null): T[] {
  return sortRows(rows, sort, SUITE_ACCESSORS);
}

// --- Specs ----------------------------------------------------------

export type SpecSortKey = 'name' | 'status' | 'duration' | 'error' | 'started';

export interface SpecRowLike extends FilterableRow {
  specName: string;
  duration: number;
  errorMessage: string | null;
  startTime: string;
}

const SPEC_ACCESSORS: Record<SpecSortKey, (s: SpecRowLike) => SortValue> = {
  name: (s) => s.specName,
  status: (s) => s.status,
  duration: (s) => s.duration,
  error: (s) => s.errorMessage,
  started: (s) => (s.startTime ? new Date(s.startTime) : null),
};

export function sortSpecs<T extends SpecRowLike>(rows: readonly T[], sort: SortState<SpecSortKey> | null): T[] {
  return sortRows(rows, sort, SPEC_ACCESSORS);
}
