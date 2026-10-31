import { useState } from 'react';
import { Link, useParams, useRouter } from '@tanstack/react-router';
import { keepPreviousData, useQuery } from '@tanstack/react-query';
import { ArrowLeft } from 'lucide-react';
import { restFetch } from '@/lib/api';
import type { TestRunConnection } from '@/lib/types';
import { Card } from '@/components/ui/Card';
import { Spinner } from '@/components/ui/Spinner';
import { StatusBadge } from '@/components/ui/StatusBadge';
import { SortableTH, Table, TBody, TD, THead, TR } from '@/components/ui/Table';
import { EmptyState } from '@/components/ui/EmptyState';
import { FilterChips } from '@/components/ui/FilterChips';
import { orderOptions, STATUS_ORDER } from '@/lib/tableFilter';
import type { SortState } from '@/lib/tableSort';
import { TestHistoryChart } from './TestHistoryChart';
import { useProject } from './hooks';
import { presetRange } from '../test-runs/dateRange';
import { DateRangeFilter } from '../test-runs/DateRangeFilter';
import type { TestRunSortKey } from '../test-runs/hooks';
import { runDuration } from '../test-runs/runDuration';

const DEFAULT_WINDOW_DAYS = 7;

interface DateBounds {
  from?: string | undefined;
  to?: string | undefined;
}

interface TableOpts {
  sort?: SortState<TestRunSortKey> | null | undefined;
  statuses?: readonly string[] | undefined;
}

// runsQuery builds the query string for the project's run list. Cleared
// bounds mean all time, which must pass allTime=1: without it the v2
// endpoint applies its own 7-day default whenever from/to are absent.
// Sort and status go to the server so they apply to the whole window,
// not just the 20 rows that happen to be loaded.
function runsQuery(projectId: string, range: DateBounds, opts: TableOpts = {}): string {
  const params = new URLSearchParams({ project: projectId, first: '20' });
  if (opts.sort) {
    params.set('sort', opts.sort.key);
    params.set('order', opts.sort.dir);
  }
  for (const s of opts.statuses ?? []) params.append('status', s);
  if (!range.from && !range.to) {
    params.set('allTime', '1');
    return params.toString();
  }
  if (range.from) params.set('from', range.from);
  if (range.to) params.set('to', range.to);
  return params.toString();
}

// useProjectRuns fetches one page of the project's runs. The chart calls
// it with no table options and the table with its sort/status; while
// those are unset the two share a query key, so there is one request.
function useProjectRuns(projectId: string, range: DateBounds, opts: TableOpts = {}) {
  return useQuery({
    queryKey: [
      'project-runs',
      projectId,
      range.from,
      range.to,
      opts.sort?.key ?? null,
      opts.sort?.dir ?? null,
      (opts.statuses ?? []).join(','),
    ],
    queryFn: () =>
      restFetch<TestRunConnection>(
        `/api/v2/test-runs?${runsQuery(projectId, range, opts)}`,
      ),
    staleTime: 30_000,
    placeholderData: keepPreviousData,
  });
}

export default function ProjectDetail() {
  const { projectId } = useParams({ from: '/projects/$projectId' });
  const router = useRouter();
  // The user can land here from Projects, Summaries, or anywhere else.
  // Walk router history back if it's available so they land where they
  // came from; otherwise fall through to the Projects list.
  const canGoBack = router.history.length > 1;
  const goBack = () => router.history.back();

  // Name lookup is separate from the runs query and non-blocking: the
  // page renders on the id alone if it fails or is still in flight.
  const { data: project } = useProject(projectId);
  const projectName = project?.name;

  // Time window for both the history chart and the run list. Defaults
  // to 7 days, which is what the v2 endpoint already clamped to when a
  // client sent no bounds — the window was simply invisible before.
  // `null` means all time, which needs an explicit opt-out of that
  // server-side default.
  const [range, setRange] = useState<DateBounds>(() => presetRange(DEFAULT_WINDOW_DAYS));

  const [sort, setSort] = useState<SortState<TestRunSortKey> | null>(null);
  const [statuses, setStatuses] = useState<string[]>([]);

  // `data` is the unfiltered window: it feeds the history chart, the
  // total, and the status chip counts. `table` is the sorted/filtered
  // page the run table renders.
  const { data, isLoading, error } = useProjectRuns(projectId, range);
  const table = useProjectRuns(projectId, range, { sort, statuses });

  if (isLoading) {
    return (
      <div className="flex items-center gap-2 text-muted">
        <Spinner /> Loading {projectId}…
      </div>
    );
  }
  if (error) {
    return (
      <EmptyState
        title="Couldn't load project"
        description={(error as Error).message}
      />
    );
  }

  const edges = data?.edges ?? [];
  const tableEdges = table.data?.edges ?? [];
  // Offer whatever statuses the window actually has (runs can also be
  // e.g. "completed"), in workflow order.
  const statusOptions = orderOptions(data?.facets.byStatus ?? [], STATUS_ORDER);

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-3 text-sm">
        {canGoBack && (
          <button
            type="button"
            onClick={goBack}
            className="inline-flex items-center gap-1 text-muted hover:text-foreground"
          >
            <ArrowLeft className="h-3 w-3" /> Back
          </button>
        )}
        <Link
          to="/projects"
          className="inline-flex items-center gap-1 text-muted hover:text-foreground"
        >
          All projects
        </Link>
      </div>

      <header className="flex items-baseline justify-between">
        <div>
          <h1 className="text-2xl font-semibold">{projectName || projectId}</h1>
          {projectName && (
            <p className="font-mono text-xs text-muted">{projectId}</p>
          )}
          <p className="text-sm text-muted">Recent test runs</p>
          <div className="mt-3 max-w-md">
            <DateRangeFilter
              from={range.from}
              to={range.to}
              idPrefix="project-range"
              clearLabel="All time"
              showHeading={false}
              onChange={setRange}
            />
          </div>

        </div>
        <div className="flex items-center gap-2">
          <Link
            to="/projects/$projectId/coverage"
            params={{ projectId }}
            className="inline-flex items-center gap-1 rounded-md border border-slate-300 bg-slate-100 px-2.5 py-1 text-xs font-medium text-slate-900 shadow-sm hover:bg-slate-200 dark:border-slate-600 dark:bg-slate-800 dark:text-slate-100 dark:hover:bg-slate-700"
            title="JIRA requirement coverage by release"
          >
            📊 Coverage
          </Link>
          <Link
            to="/projects/$projectId/settings"
            params={{ projectId }}
            className="inline-flex items-center gap-1 rounded-md border border-slate-300 bg-slate-100 px-2.5 py-1 text-xs font-medium text-slate-900 shadow-sm hover:bg-slate-200 dark:border-slate-600 dark:bg-slate-800 dark:text-slate-100 dark:hover:bg-slate-700"
            title="General / Integrations / Team / Notifications"
          >
            ⚙️ Settings
          </Link>
          <span className="ml-2 text-sm text-muted">
            {data?.totalCount.toLocaleString() ?? 0}{data?.totalCountIsEstimate ? ' (est.)' : ''} total
          </span>
        </div>
      </header>

      {edges.length === 0 ? (
        <EmptyState
          title="No test runs for this project"
          description="Seed sample data with `make docker-test-seed`."
        />
      ) : (
        <>
        <TestHistoryChart runs={edges.map((e) => e.node)} />
        <Card>
          <div className="flex flex-wrap items-center gap-3 border-b border-border px-3 py-2">
            <FilterChips
              label="Status"
              options={statusOptions}
              selected={statuses}
              onChange={setStatuses}
            />
            <span className="ml-auto text-xs text-muted">
              {table.isFetching && <Spinner className="mr-1" />}
              Showing {tableEdges.length}
              {table.data ? ` of ${table.data.totalCount.toLocaleString()}` : ''}
            </span>
          </div>
          {tableEdges.length === 0 ? (
            <EmptyState title="No runs match the status filter" />
          ) : (
          <Table>
            <THead>
              <TR>
                <SortableTH sortKey="started" sort={sort} onSortChange={setSort}>Started</SortableTH>
                <SortableTH sortKey="status" sort={sort} onSortChange={setSort}>Status</SortableTH>
                <SortableTH sortKey="branch" sort={sort} onSortChange={setSort}>Branch</SortableTH>
                <SortableTH sortKey="duration" sort={sort} onSortChange={setSort} className="text-right">
                  Duration
                </SortableTH>
                <SortableTH
                  sortKey="failed"
                  sort={sort}
                  onSortChange={setSort}
                  className="text-right"
                  title="Sorts by failed count"
                >
                  Passed / Failed
                </SortableTH>
              </TR>
            </THead>
            <TBody>
              {tableEdges.map(({ node }) => (
                <TR key={node.id}>
                  <TD>
                    <Link
                      to="/test-runs/$runId"
                      params={{ runId: String(node.id) }}
                      className="text-primary hover:underline"
                    >
                      {new Date(node.start_time).toLocaleString()}
                    </Link>
                    <div className="text-xs text-muted">{node.run_id}</div>
                  </TD>
                  <TD><StatusBadge status={node.status} /></TD>
                  <TD className="font-mono text-xs">{node.branch || '—'}</TD>
                  <TD className="text-right tabular-nums">{runDuration(node)}</TD>
                  <TD className="text-right tabular-nums">
                    <span className="text-green-600 dark:text-green-400">{node.passed_tests}</span>
                    {' / '}
                    <span className="text-red-600 dark:text-red-400">{node.failed_tests}</span>
                  </TD>
                </TR>
              ))}
            </TBody>
          </Table>
          )}
        </Card>
        </>
      )}
    </div>
  );
}
