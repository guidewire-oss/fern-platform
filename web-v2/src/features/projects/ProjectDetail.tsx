import { useState } from 'react';
import { Link, useParams, useRouter } from '@tanstack/react-router';
import { useQuery } from '@tanstack/react-query';
import { ArrowLeft } from 'lucide-react';
import { restFetch } from '@/lib/api';
import type { TestRunConnection } from '@/lib/types';
import { Card } from '@/components/ui/Card';
import { Spinner } from '@/components/ui/Spinner';
import { StatusBadge } from '@/components/ui/StatusBadge';
import { Table, TBody, TD, TH, THead, TR } from '@/components/ui/Table';
import { EmptyState } from '@/components/ui/EmptyState';
import { formatDuration } from '@/lib/duration';
import { TestHistoryChart } from './TestHistoryChart';
import { useProject } from './hooks';
import { presetRange } from '../test-runs/dateRange';
import { DateRangeFilter } from '../test-runs/DateRangeFilter';

const DEFAULT_WINDOW_DAYS = 7;

interface DateBounds {
  from?: string | undefined;
  to?: string | undefined;
}

// runsQuery builds the query string for the project's run list. Cleared
// bounds mean all time, which must pass allTime=1: without it the v2
// endpoint applies its own 7-day default whenever from/to are absent.
function runsQuery(projectId: string, range: DateBounds): string {
  const params = new URLSearchParams({ project: projectId, first: '20' });
  if (!range.from && !range.to) {
    params.set('allTime', '1');
    return params.toString();
  }
  if (range.from) params.set('from', range.from);
  if (range.to) params.set('to', range.to);
  return params.toString();
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

  const { data, isLoading, error } = useQuery({
    queryKey: ['project-runs', projectId, range.from, range.to],
    queryFn: () =>
      restFetch<TestRunConnection>(
        `/api/v2/test-runs?${runsQuery(projectId, range)}`,
      ),
    staleTime: 30_000,
  });

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
          <Table>
            <THead>
              <TR>
                <TH>Started</TH>
                <TH>Status</TH>
                <TH>Branch</TH>
                <TH className="text-right">Duration</TH>
                <TH className="text-right">Passed / Failed</TH>
              </TR>
            </THead>
            <TBody>
              {edges.map(({ node }) => (
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
                  <TD className="text-right tabular-nums">
                    {node.end_time
                      ? formatDuration(
                          new Date(node.end_time).getTime() -
                            new Date(node.start_time).getTime(),
                        )
                      : '—'}
                  </TD>
                  <TD className="text-right tabular-nums">
                    <span className="text-green-600 dark:text-green-400">{node.passed_tests}</span>
                    {' / '}
                    <span className="text-red-600 dark:text-red-400">{node.failed_tests}</span>
                  </TD>
                </TR>
              ))}
            </TBody>
          </Table>
        </Card>
        </>
      )}
    </div>
  );
}
