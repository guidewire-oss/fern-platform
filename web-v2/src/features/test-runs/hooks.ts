import { useQuery, keepPreviousData } from '@tanstack/react-query';
import { restFetch } from '@/lib/api';
import type { TestRunConnection } from '@/lib/types';

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
