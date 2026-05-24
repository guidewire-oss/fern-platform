import { Link } from '@tanstack/react-router';
import { useQuery } from '@tanstack/react-query';
import {
  FolderKanban,
  PlayCircle,
  CheckCircle2,
  AlertTriangle,
  ArrowRight,
  Sparkles,
} from 'lucide-react';
import { restFetch } from '@/lib/api';
import type { TestRunConnection } from '@/lib/types';
import { Spinner } from '@/components/ui/Spinner';
import { StatusBadge } from '@/components/ui/StatusBadge';
import { useProjects } from '../projects/hooks';

export default function Dashboard() {
  const projects = useProjects();

  const allRuns = useQuery({
    queryKey: ['dash-runs', 'all'],
    queryFn: () => restFetch<TestRunConnection>('/api/v2/test-runs?first=10'),
    staleTime: 60_000,
  });

  const failedRuns = useQuery({
    queryKey: ['dash-runs', 'failed'],
    queryFn: () => restFetch<TestRunConnection>('/api/v2/test-runs?status=failed&first=1'),
    staleTime: 60_000,
  });

  const flakyRuns = useQuery({
    queryKey: ['dash-runs', 'flaky'],
    queryFn: () => restFetch<TestRunConnection>('/api/v2/test-runs?status=flaky&first=1'),
    staleTime: 60_000,
  });

  const totalProjects = projects.data?.totalCount ?? 0;
  const totalRuns = allRuns.data?.totalCount ?? 0;
  const failedCount = failedRuns.data?.totalCount ?? 0;
  const flakyCount = flakyRuns.data?.totalCount ?? 0;
  const successRate = totalRuns > 0
    ? Math.round(((totalRuns - failedCount) / totalRuns) * 1000) / 10
    : 0;

  return (
    <div className="space-y-8">
      <header>
        <div className="flex items-center gap-2 text-xs uppercase tracking-[0.18em] text-muted">
          <Sparkles className="h-3 w-3 text-primary" />
          v2 dashboard
        </div>
        <h1 className="mt-1 text-3xl font-semibold tracking-tight">
          Test intelligence at a glance
        </h1>
        <p className="mt-1 text-sm text-muted">
          Stats below are computed live from the v2 API against the seeded dataset.
        </p>
      </header>

      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
        <MetricCard
          label="Projects"
          value={totalProjects}
          icon={FolderKanban}
          gradient="bg-gradient-primary"
          loading={projects.isLoading}
          href="/projects"
        />
        <MetricCard
          label="Test runs"
          value={totalRuns.toLocaleString()}
          icon={PlayCircle}
          gradient="bg-gradient-to-br from-indigo-500 to-purple-600"
          loading={allRuns.isLoading}
          href="/test-runs"
        />
        <MetricCard
          label="Success rate"
          value={`${successRate}%`}
          icon={CheckCircle2}
          gradient="bg-gradient-success"
          loading={allRuns.isLoading || failedRuns.isLoading}
        />
        <MetricCard
          label="Flaky runs"
          value={flakyCount.toLocaleString()}
          icon={AlertTriangle}
          gradient="bg-gradient-warning"
          loading={flakyRuns.isLoading}
        />
      </div>

      <div className="grid gap-4 lg:grid-cols-3">
        <RecentRunsPanel
          edges={allRuns.data?.edges ?? []}
          loading={allRuns.isLoading}
          className="lg:col-span-2"
        />

        <div className="fern-card overflow-hidden">
          <div className="border-b border-border bg-gradient-to-r from-primary-soft to-transparent px-4 py-3">
            <h3 className="text-sm font-semibold">Active projects</h3>
            <p className="text-xs text-muted">{totalProjects} total</p>
          </div>
          <div className="divide-y divide-border">
            {projects.isLoading && (
              <div className="flex items-center gap-2 p-4 text-sm text-muted">
                <Spinner /> Loading…
              </div>
            )}
            {projects.data?.projects.slice(0, 6).map((node) => (
              <Link
                key={node.projectId}
                to="/projects/$projectId"
                params={{ projectId: node.projectId }}
                className="flex items-center justify-between px-4 py-3 transition-colors hover:bg-surface-2"
              >
                <div className="min-w-0">
                  <div className="truncate text-sm font-medium">{node.name}</div>
                  <div className="text-xs text-muted">
                    {node.team || '—'} · {node.stats?.totalTestRuns ?? 0} runs
                  </div>
                </div>
                <span className="text-xs tabular-nums text-foreground">
                  {node.stats?.successRate != null
                    ? `${Math.round(node.stats.successRate * 100)}%`
                    : '—'}
                </span>
              </Link>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}

function MetricCard({
  label,
  value,
  icon: Icon,
  gradient,
  loading,
  href,
}: {
  label: string;
  value: string | number;
  icon: typeof CheckCircle2;
  gradient: string;
  loading?: boolean;
  href?: string;
}) {
  const body = (
    <div className="fern-card fern-pop relative h-full overflow-hidden p-5">
      <div className={`mb-3 inline-flex h-10 w-10 items-center justify-center rounded-lg ${gradient} text-white shadow-md`}>
        <Icon className="h-5 w-5" />
      </div>
      <div className="text-[11px] font-medium uppercase tracking-wider text-muted">
        {label}
      </div>
      <div className="mt-0.5 flex items-baseline gap-2">
        <div className="text-3xl font-semibold tracking-tight tabular-nums">
          {loading ? <Spinner className="text-muted" /> : value}
        </div>
      </div>
      {href && (
        <div className="mt-3 flex items-center gap-1 text-xs text-primary">
          View details <ArrowRight className="h-3 w-3" />
        </div>
      )}
    </div>
  );
  return href ? <Link to={href} className="block">{body}</Link> : body;
}

function RecentRunsPanel({
  edges,
  loading,
  className,
}: {
  edges: TestRunConnection['edges'];
  loading: boolean;
  className?: string;
}) {
  return (
    <div className={`fern-card overflow-hidden ${className ?? ''}`}>
      <div className="flex items-center justify-between border-b border-border px-4 py-3">
        <div>
          <h3 className="text-sm font-semibold">Recent test runs</h3>
          <p className="text-xs text-muted">Newest first</p>
        </div>
        <Link to="/test-runs" className="text-xs text-primary hover:underline">
          View all →
        </Link>
      </div>
      <div className="divide-y divide-border">
        {loading && (
          <div className="flex items-center gap-2 p-4 text-sm text-muted">
            <Spinner /> Loading…
          </div>
        )}
        {edges.slice(0, 8).map(({ node }) => (
          <Link
            key={node.id}
            to="/test-runs/$runId"
            params={{ runId: String(node.id) }}
            className="flex items-center gap-3 px-4 py-2.5 transition-colors hover:bg-surface-2"
          >
            <StatusBadge status={node.status} />
            <div className="min-w-0 flex-1">
              <div className="truncate text-sm font-medium">{node.project_id}</div>
              <div className="truncate text-xs text-muted">
                {node.branch || 'no branch'} ·{' '}
                {new Date(node.start_time).toLocaleString()}
              </div>
            </div>
            <div className="hidden text-right text-xs tabular-nums sm:block">
              <span className="text-green-700">{node.passed_tests}</span>
              {' / '}
              <span className="text-red-700">{node.failed_tests}</span>
            </div>
          </Link>
        ))}
        {!loading && edges.length === 0 && (
          <div className="p-6 text-center text-sm text-muted">
            No test runs yet — try{' '}
            <code className="rounded bg-surface-2 px-1.5 py-0.5">make docker-test-seed</code>.
          </div>
        )}
      </div>
    </div>
  );
}
