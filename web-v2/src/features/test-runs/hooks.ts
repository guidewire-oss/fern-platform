import { useQuery, keepPreviousData } from '@tanstack/react-query';
import { restFetch } from '@/lib/api';
import type { RunHistory, SpecRunHistory, TestRunConnection } from '@/lib/types';
import type { SortDir } from '@/lib/tableSort';

// Server-side sort keys accepted by GET /api/v2/test-runs?sort=. Sorting
// happens in the query (not on the loaded page) because the list is
// keyset-paginated: a client sort would only reorder the visible page.
export type TestRunSortKey =
  | 'started'
  | 'duration'
  | 'status'
  | 'branch'
  | 'project'
  | 'run_id'
  | 'failed';

export interface TestRunsFilter {
  // All optional fields allow explicit undefined so callers can clear
  // a value via spread under TS's exactOptionalPropertyTypes setting.
  project?: string[] | undefined;
  status?: string[] | undefined;
  branch?: string[] | undefined;
  tag?: string[] | undefined;
  q?: string | undefined;
  // ISO timestamps for the date-range filter. Server matches by start_time.
  from?: string | undefined;
  to?: string | undefined;
  // Duration filter in milliseconds.
  durationGte?: number | undefined;
  durationLte?: number | undefined;
  // Ordering. Absent means newest first (start_time DESC).
  sort?: TestRunSortKey | undefined;
  order?: SortDir | undefined;
  // Pagination.
  first?: number | undefined;
  after?: string | undefined;
  // Opt-in expensive facets. The tag facet joins through suite_runs;
  // include it only when the user has expanded the Tag section.
  includeTagFacet?: boolean | undefined;
}

function buildQuery(f: TestRunsFilter): string {
  const params = new URLSearchParams();
  for (const v of f.project ?? []) params.append('project', v);
  for (const v of f.status  ?? []) params.append('status',  v);
  for (const v of f.branch  ?? []) params.append('branch',  v);
  for (const v of f.tag     ?? []) params.append('tag',     v);
  if (f.q)     params.set('q',     f.q);
  if (f.from)  params.set('from',  f.from);
  if (f.to)    params.set('to',    f.to);
  if (f.durationGte != null) params.set('durationGte', String(f.durationGte));
  if (f.durationLte != null) params.set('durationLte', String(f.durationLte));
  if (f.sort)  params.set('sort',  f.sort);
  if (f.sort && f.order) params.set('order', f.order);
  if (f.first) params.set('first', String(f.first));
  if (f.after) params.set('after', f.after);
  // If the user already has any tag-facet visible (selected tags) or
  // explicitly asked for it, request the tag facet from the server.
  if (f.includeTagFacet || (f.tag && f.tag.length > 0)) {
    params.set('facets', 'tag');
  }
  const s = params.toString();
  return s ? `?${s}` : '';
}

export function useTestRuns(filter: TestRunsFilter) {
  return useQuery({
    queryKey: ['test-runs', filter],
    queryFn: () =>
      restFetch<TestRunConnection>(`/api/v2/test-runs${buildQuery(filter)}`),
    staleTime: 15_000,
    placeholderData: keepPreviousData,
  });
}

// useRunHistory loads the history strip data for one run: how each of
// its suites and specs fared in the project's last runs on the same
// branch. `id` is the run's database id (the GraphQL TestRun.id); the
// query waits until it is known. A run's past doesn't change, so the
// result can stay fresh a while.
export function useRunHistory(id: string | undefined) {
  return useQuery({
    queryKey: ['run-history', id],
    queryFn: () =>
      restFetch<RunHistory>(`/api/v2/test-runs/${encodeURIComponent(id ?? '')}/history`),
    enabled: !!id,
    staleTime: 60_000,
  });
}

// useSpecHistory loads one spec's history with error messages for the
// spec history panel. It runs only while the panel is open, so error
// text is never fetched for specs nobody looks at.
export function useSpecHistory(id: string, suiteName: string, specName: string, enabled: boolean) {
  return useQuery({
    queryKey: ['spec-history', id, suiteName, specName],
    queryFn: () => {
      const q = new URLSearchParams({ suite: suiteName, spec: specName });
      return restFetch<SpecRunHistory>(
        `/api/v2/test-runs/${encodeURIComponent(id)}/history/spec?${q.toString()}`,
      );
    },
    enabled,
    staleTime: 60_000,
  });
}
