import { useEffect, useMemo, useRef, useState } from 'react';
import { Link, useSearch } from '@tanstack/react-router';
import { useQuery } from '@tanstack/react-query';
import { TrendingUp, TrendingDown, Minus, History, LayoutGrid, BarChart3, X } from 'lucide-react';
import { restFetch } from '@/lib/api';
import { Card } from '@/components/ui/Card';
import { Spinner } from '@/components/ui/Spinner';
import { EmptyState } from '@/components/ui/EmptyState';
import { Sparkline } from '@/components/ui/Sparkline';
import { Button } from '@/components/ui/Button';
import { cn } from '@/lib/cn';
import {
  ProjectsFilterBar,
  DEFAULT_FILTER as DEFAULT_PROJECTS_FILTER,
  type ProjectsFilter,
} from '../projects/ProjectsFilterBar';
import { useUserPreferences } from '../profile/hooks';
import { useProjects } from '../projects/hooks';
import { TreemapView } from '../treemap/Treemap';
import { sumWindowStats } from './sumWindowStats';
import { sortProjects } from '../projects/sortProjects';
import { customRangeToQuery, queryToRangeInputs } from '../test-runs/dateRange';

type ViewMode = 'card' | 'tree';
const VIEW_MODE_KEY = 'fern-v2.summaries.viewMode';
const FILTER_KEY    = 'fern-v2.summaries.filter';
const DAYS_KEY      = 'fern-v2.summaries.days';

function loadViewMode(): ViewMode {
  if (typeof window === 'undefined') return 'card';
  const v = window.localStorage.getItem(VIEW_MODE_KEY);
  return v === 'tree' ? 'tree' : 'card';
}
function persistViewMode(v: ViewMode) {
  if (typeof window === 'undefined') return;
  window.localStorage.setItem(VIEW_MODE_KEY, v);
}

// Filter/days survive a round-trip to a project history page and back.
// sessionStorage (per-tab) is the right scope: persists across navigation
// but does not bleed into another tab the user opened beforehand.
function loadFilter(): ProjectsFilter {
  if (typeof window === 'undefined') return DEFAULT_PROJECTS_FILTER;
  try {
    const raw = window.sessionStorage.getItem(FILTER_KEY);
    if (!raw) return DEFAULT_PROJECTS_FILTER;
    const parsed = JSON.parse(raw) as Partial<ProjectsFilter>;
    return { ...DEFAULT_PROJECTS_FILTER, ...parsed };
  } catch {
    return DEFAULT_PROJECTS_FILTER;
  }
}
function persistFilter(f: ProjectsFilter) {
  if (typeof window === 'undefined') return;
  window.sessionStorage.setItem(FILTER_KEY, JSON.stringify(f));
}
function loadDays(fallback: number): number {
  if (typeof window === 'undefined') return fallback;
  const raw = window.sessionStorage.getItem(DAYS_KEY);
  const n = raw ? Number(raw) : NaN;
  return Number.isFinite(n) && n > 0 ? n : fallback;
}
function persistDays(d: number) {
  if (typeof window === 'undefined') return;
  window.sessionStorage.setItem(DAYS_KEY, String(d));
}

const RANGE_KEY = 'fern-v2.summaries.range';
// An anchored from/to range (RFC3339). Empty object means "use the rolling
// days window" — the two are mutually exclusive, mirroring the test-runs
// date filter.
type SummariesRange = { from?: string | undefined; to?: string | undefined };
function loadRange(): SummariesRange {
  if (typeof window === 'undefined') return {};
  try {
    const raw = window.sessionStorage.getItem(RANGE_KEY);
    if (!raw) return {};
    const p = JSON.parse(raw) as SummariesRange;
    return { from: p.from, to: p.to };
  } catch {
    return {};
  }
}
function persistRange(r: SummariesRange) {
  if (typeof window === 'undefined') return;
  window.sessionStorage.setItem(RANGE_KEY, JSON.stringify(r));
}

// Server-aggregated row shape returned by /api/v2/test-runs/trends.
interface TrendRow {
  day: string;          // YYYY-MM-DD UTC
  totalRuns: number;
  totalTests: number;
  passedTests: number;
  failedTests: number;
  skippedTests: number;
  durationMs: number;
}

interface TrendsResponse {
  from: string;
  to: string;
  days: number;
  buckets: Record<string, TrendRow[]>; // projectId → daily rows (server returns oldest first)
}

const WINDOWS = [
  { days: 7,   label: '7 days'   },
  { days: 30,  label: '30 days'  },
  { days: 90,  label: '90 days'  },
  { days: 180, label: '180 days' },
];

export default function TestSummaries() {
  // URL search params win over persisted sessionStorage on first mount,
  // so deep-links like /manager-dashboard → /summaries?view=tree&favoritesOnly=true
  // land in the intended state regardless of what the user did last time.
  const search = useSearch({ from: '/summaries' }) as {
    view?: 'card' | 'tree';
    favoritesOnly?: 'true';
  };

  const [days, setDaysState] = useState(() => loadDays(7));
  const [range, setRangeState] = useState<SummariesRange>(() => loadRange());
  // Custom (anchored) mode when both bounds are set; otherwise the rolling
  // days window drives the view. Picking a preset clears the range and a
  // range clears the preset highlight — mutually exclusive, like test-runs.
  const customActive = !!(range.from && range.to);
  const [filter, setFilterState] = useState<ProjectsFilter>(() => {
    const base = loadFilter();
    if (search.favoritesOnly === 'true') return { ...base, favoritesOnly: true };
    return base;
  });
  const [viewMode, setViewMode] = useState<ViewMode>(() =>
    search.view ?? loadViewMode(),
  );
  const setDays = (d: number) => {
    setDaysState(d);
    persistDays(d);
    // Selecting a rolling preset exits custom-range mode.
    setRangeState({});
    persistRange({});
  };
  const setRange = (next: SummariesRange) => {
    setRangeState(next);
    persistRange(next);
  };
  const setFilter = (next: ProjectsFilter | ((p: ProjectsFilter) => ProjectsFilter)) => {
    setFilterState((prev) => {
      const resolved = typeof next === 'function' ? next(prev) : next;
      persistFilter(resolved);
      return resolved;
    });
  };
  const selectViewMode = (v: ViewMode) => {
    setViewMode(v);
    persistViewMode(v);
  };

  const projects = useProjects();
  const prefs = useUserPreferences();

  // Adapt the flat Project[] from useProjects to the {node} shape this
  // page's filter/sort logic was originally written against. Saves
  // churning the rest of the file.
  const edges = useMemo(
    () => (projects.data?.projects ?? []).map((p) => ({ node: p })),
    [projects.data?.projects],
  );
  const favoriteSet = useMemo(
    () => new Set(prefs.data?.userPreferences?.favorites ?? []),
    [prefs.data],
  );
  const availableTeams = useMemo(
    () =>
      Array.from(
        new Set(edges.map((e) => e.node.team).filter((t): t is string => !!t)),
      ).sort(),
    [edges],
  );

  const filteredUnsorted = useMemo(() => {
    const q = filter.q.trim().toLowerCase();
    const teamSet = new Set(filter.teams);
    const catSet = new Set(filter.categories);
    return edges.filter(({ node }) => {
      if (filter.favoritesOnly && !favoriteSet.has(node.projectId)) return false;
      if (teamSet.size > 0 && !(node.team && teamSet.has(node.team))) return false;
      if (catSet.size > 0) {
        const id = node.projectId.toLowerCase();
        const matches = Array.from(catSet).some((c) =>
          id.startsWith(c.toLowerCase() + '-') ||
          id.includes('-' + c.toLowerCase() + '-') ||
          id.startsWith(c.toLowerCase()),
        );
        if (!matches) return false;
      }
      if (q) {
        const hay = `${node.name} ${node.projectId} ${node.team ?? ''}`.toLowerCase();
        if (!hay.includes(q)) return false;
      }
      return true;
    });
  }, [edges, filter, favoriteSet]);

  // Trends for the selected window drive both the cards and the "Runs" sort.
  // Sorting off the *window* run totals (not all-time stats) is what makes
  // the Runs toggle visibly reorder — projects can share an all-time count
  // (e.g. 450) yet differ over 7/30/90/180 days.
  const trends = useWindowTrends(
    filteredUnsorted.map((e) => e.node.projectId),
    customActive ? { from: range.from!, to: range.to! } : { days },
  );

  const filteredEdges = useMemo(
    () =>
      sortProjects(
        filteredUnsorted.map((e) => e.node),
        filter.sortKey,
        filter.sortDir,
        trends.runsByProject,
      ).map((node) => ({ node })),
    [filteredUnsorted, filter.sortKey, filter.sortDir, trends.runsByProject],
  );

  const projectIds = filteredEdges.map((e) => e.node.projectId);

  return (
    <div className="space-y-6">
      <header className="flex items-end justify-between gap-3">
        <div>
          <h1 className="text-3xl font-semibold tracking-tight">Test summaries</h1>
          <p className="mt-1 text-sm text-muted">
            Per-project trends over the selected window. Each sparkline is a
            day-bucket of pass rate aggregated server-side via
            /api/v2/test-runs/trends.
          </p>
        </div>
        <div className="flex items-center gap-2">
          <div
            role="group"
            aria-label="View mode"
            className="flex overflow-hidden rounded-md border border-border bg-surface"
          >
            <button
              type="button"
              aria-pressed={viewMode === 'card'}
              onClick={() => selectViewMode('card')}
              className={cn(
                'inline-flex items-center gap-1 px-3 py-1.5 text-xs transition-colors',
                viewMode === 'card'
                  ? 'bg-primary text-white'
                  : 'text-foreground hover:bg-surface-2',
              )}
            >
              <LayoutGrid className="h-3.5 w-3.5" /> Card view
            </button>
            <button
              type="button"
              aria-pressed={viewMode === 'tree'}
              onClick={() => selectViewMode('tree')}
              className={cn(
                'inline-flex items-center gap-1 px-3 py-1.5 text-xs transition-colors',
                viewMode === 'tree'
                  ? 'bg-primary text-white'
                  : 'text-foreground hover:bg-surface-2',
              )}
            >
              <BarChart3 className="h-3.5 w-3.5" /> Tree view
            </button>
          </div>
          <div className="flex items-center gap-2">
            <div className="flex overflow-hidden rounded-md border border-border bg-surface">
              {WINDOWS.map((w) => (
                <button
                  key={w.days}
                  onClick={() => setDays(w.days)}
                  className={cn(
                    'px-3 py-1.5 text-xs transition-colors',
                    !customActive && days === w.days
                      ? 'bg-primary text-white'
                      : 'text-foreground hover:bg-surface-2',
                  )}
                >
                  {w.label}
                </button>
              ))}
            </div>
            <SummariesRangePicker range={range} customActive={customActive} onChange={setRange} />
          </div>
        </div>
      </header>

      {projects.isLoading && (
        <div className="flex items-center gap-2 text-muted"><Spinner /> Loading projects…</div>
      )}
      {projects.error && (
        <EmptyState
          title="Couldn't load projects"
          description={(projects.error as Error).message}
        />
      )}
      {!projects.isLoading && edges.length === 0 && (
        <EmptyState title="No projects yet" description="Seed with `make docker-test-seed`." />
      )}

      {edges.length > 0 && (
        <ProjectsFilterBar
          filter={filter}
          onChange={setFilter}
          availableTeams={availableTeams}
          favoritesCount={favoriteSet.size}
          visibleCount={filteredEdges.length}
          totalCount={edges.length}
          // A treemap packs tiles by area/health — no list order to sort —
          // so the sort control only applies in card view.
          showSort={viewMode === 'card'}
        />
      )}

      {viewMode === 'tree' ? (
        <TreemapView days={days} projectIdAllowlist={projectIds} />
      ) : edges.length > 0 && projectIds.length === 0 ? (
        <EmptyState
          title="No projects match"
          description="Try clearing some filters."
          action={
            <Button variant="ghost" onClick={() => setFilter(DEFAULT_PROJECTS_FILTER)}>
              Reset filters
            </Button>
          }
        />
      ) : (
        <TrendCardsGrid edges={filteredEdges} trends={trends} />
      )}
    </div>
  );
}

// useWindowTrends makes the single /test-runs/trends call covering every
// visible project for the selected window. The previous design fanned out
// one HTTP request per card — at 100 visible projects that was 100 parallel
// requests, each driving COUNT(*) + facet GROUP BYs on the server. One call
// returns the same data in one SQL aggregate.
//
// It also rolls each project's window buckets into a `projectId → totalRuns`
// map so the page can sort by the same run totals the cards display.
// TrendWindow is either a rolling day count or an anchored from/to range
// (RFC3339). The range mirrors the test-runs date filter so Summaries can
// look at an arbitrary historical span, not just the last N days.
type TrendWindow = { days: number } | { from: string; to: string };

// windowSpanDays returns the inclusive number of day-buckets a window
// covers — the day count directly, or the ceil'd span of a range. Used for
// zero-filling the sparklines to a gap-free length.
function windowSpanDays(w: TrendWindow): number {
  if ('days' in w) return w.days;
  const ms = new Date(w.to).getTime() - new Date(w.from).getTime();
  return Math.max(1, Math.ceil(ms / (24 * 60 * 60 * 1000)));
}

function useWindowTrends(projectIds: string[], window: TrendWindow) {
  // Stable key for React Query — sort the IDs so visible-order shuffles
  // don't bust the cache, and the server cache stays warm too (its key
  // is also sorted).
  const sortedKey = useMemo(
    () => [...projectIds].sort().join('|'),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [sortedKeyJoiner(projectIds)],
  );

  const isRange = 'from' in window;
  const params = new URLSearchParams();
  for (const id of projectIds) params.append('project', id);
  if (isRange) {
    params.set('from', window.from);
    params.set('to', window.to);
  } else {
    params.set('days', String(window.days));
  }
  // Token distinguishing this window in the query cache (rolling vs anchored).
  const winToken = isRange ? `r${window.from}_${window.to}` : `d${window.days}`;

  const query = useQuery({
    queryKey: ['summaries-trends', sortedKey, winToken],
    queryFn: () =>
      restFetch<TrendsResponse>(`/api/v2/test-runs/trends?${params.toString()}`),
    staleTime: 60_000,
    enabled: projectIds.length > 0,
  });

  const runsByProject = useMemo(() => {
    const m: Record<string, number> = {};
    for (const [pid, rows] of Object.entries(query.data?.buckets ?? {})) {
      m[pid] = sumWindowStats(rows).totalRuns;
    }
    return m;
  }, [query.data]);

  return {
    buckets: query.data?.buckets ?? {},
    runsByProject,
    isLoading: query.isLoading,
    error: query.error as Error | null,
    // Window shape for the cards: span length and the anchor end date
    // (undefined → today, for the rolling window).
    days: windowSpanDays(window),
    endDate: isRange ? window.to.slice(0, 10) : undefined,
  };
}

function TrendCardsGrid({
  edges,
  trends,
}: {
  edges: { node: { id: string; projectId: string; name: string; team: string } }[];
  trends: ReturnType<typeof useWindowTrends>;
}) {
  if (trends.isLoading) {
    return (
      <div className="flex items-center gap-2 text-muted">
        <Spinner /> Loading trends…
      </div>
    );
  }
  if (trends.error) {
    return <EmptyState title="Couldn't load trends" description={trends.error.message} />;
  }
  const { buckets } = trends;
  return (
    <div className="grid gap-4 lg:grid-cols-2 xl:grid-cols-3">
      {edges.map(({ node }) => (
        <ProjectTrendCard
          key={node.id}
          projectId={node.projectId}
          name={node.name}
          team={node.team}
          days={trends.days}
          endDate={trends.endDate}
          rows={buckets[node.projectId] ?? []}
        />
      ))}
    </div>
  );
}

// sortedKeyJoiner returns a string that changes whenever the *set* of
// project IDs changes (order-independent). Used as the dependency for
// the useMemo above so we don't refetch on a no-op shuffle.
function sortedKeyJoiner(ids: string[]): string {
  return [...ids].sort().join('|');
}

// SummariesRangePicker is the from/to date range for the summaries window,
// mirroring the test-runs date filter. Both bounds must be set for the
// range to take effect (customActive); a clear button returns to the
// rolling-days presets.
function SummariesRangePicker({
  range,
  customActive,
  onChange,
}: {
  range: SummariesRange;
  customActive: boolean;
  onChange: (r: SummariesRange) => void;
}) {
  const inputs = queryToRangeInputs(range.from, range.to);
  const setBound = (which: 'fromDate' | 'toDate', value: string) => {
    const next = { ...inputs, [which]: value };
    onChange(customRangeToQuery(next.fromDate, next.toDate));
  };
  return (
    <div
      className={cn(
        'flex items-center gap-1 rounded-md border px-2 py-1 transition-colors',
        customActive ? 'border-primary bg-primary/5' : 'border-border bg-surface',
      )}
    >
      <input
        type="date"
        aria-label="Range start"
        max={inputs.toDate || undefined}
        value={inputs.fromDate}
        onChange={(e) => setBound('fromDate', e.target.value)}
        className="rounded border border-border bg-surface px-1.5 py-0.5 text-xs text-foreground"
      />
      <span className="text-xs text-muted">→</span>
      <input
        type="date"
        aria-label="Range end"
        min={inputs.fromDate || undefined}
        value={inputs.toDate}
        onChange={(e) => setBound('toDate', e.target.value)}
        className="rounded border border-border bg-surface px-1.5 py-0.5 text-xs text-foreground"
      />
      {(range.from || range.to) && (
        <button
          onClick={() => onChange({})}
          aria-label="Clear date range"
          className="ml-0.5 rounded p-0.5 text-muted hover:bg-surface-2 hover:text-foreground"
        >
          <X className="h-3.5 w-3.5" />
        </button>
      )}
    </div>
  );
}

// useInViewOnce reports `true` once the observed element has entered
// the viewport (with a generous rootMargin so paint begins just before
// the card scrolls in). Stays true thereafter — we don't want
// sparklines to flicker on/off as the user scrolls past. Defers the
// SVG paint cost of off-screen cards, which matters when the page
// renders 100+ project cards.
function useInViewOnce(rootMargin = '200px'): {
  ref: React.RefObject<HTMLDivElement>;
  inView: boolean;
} {
  const ref = useRef<HTMLDivElement>(null);
  const [inView, setInView] = useState(false);
  useEffect(() => {
    if (inView) return;
    const el = ref.current;
    if (!el) return;
    // IntersectionObserver is universally supported in v2's target
    // browsers; no polyfill needed.
    const obs = new IntersectionObserver(
      (entries) => {
        if (entries.some((e) => e.isIntersecting)) {
          setInView(true);
          obs.disconnect();
        }
      },
      { rootMargin },
    );
    obs.observe(el);
    return () => obs.disconnect();
  }, [inView, rootMargin]);
  return { ref, inView };
}

// zeroFillRows turns the server's sparse rows (days with at least one
// run) into a dense per-day series across the full window. Days with
// no runs get { totalRuns: 0, ... } so the sparkline is gap-free.
function zeroFillRows(rows: TrendRow[], days: number, endDate?: string): TrendRow[] {
  const byDay = new Map(rows.map((r) => [r.day, r]));
  const out: TrendRow[] = [];
  // Anchor at endDate (custom ranges end in the past) or today (rolling
  // window). Parse as UTC midnight so day arithmetic is timezone-stable.
  const anchor = endDate ? new Date(`${endDate}T00:00:00Z`) : new Date();
  for (let i = days - 1; i >= 0; i--) {
    const d = new Date(anchor);
    d.setUTCDate(anchor.getUTCDate() - i);
    const key = d.toISOString().slice(0, 10);
    out.push(
      byDay.get(key) ?? {
        day: key,
        totalRuns: 0,
        totalTests: 0,
        passedTests: 0,
        failedTests: 0,
        skippedTests: 0,
        durationMs: 0,
      },
    );
  }
  return out;
}

function ProjectTrendCard({
  projectId,
  name,
  team,
  days,
  endDate,
  rows,
}: {
  projectId: string;
  name: string;
  team: string;
  days: number;
  endDate?: string | undefined;
  rows: TrendRow[];
}) {
  const buckets = useMemo(() => zeroFillRows(rows, days, endDate), [rows, days, endDate]);

  const stats = useMemo(() => sumWindowStats(buckets), [buckets]);
  const { totalTests, passedTests, failedTests, skippedTests, passRate } = stats;
  const passSeries = buckets.map((b) => (b.totalTests > 0 ? b.passedTests / b.totalTests : 0));
  // Average duration per run (seconds) for each day; zero days collapse to 0.
  const durationSeries = buckets.map((b) =>
    b.totalRuns > 0 ? b.durationMs / b.totalRuns / 1000 : 0,
  );

  // Trend = compare last 25% of window to first 25%
  const trend = useMemo(() => {
    const slice = Math.max(2, Math.floor(passSeries.length / 4));
    const early = avg(passSeries.slice(0, slice).filter((v) => v > 0));
    const late  = avg(passSeries.slice(-slice).filter((v) => v > 0));
    if (early === 0 || late === 0) return 0;
    return late - early;
  }, [passSeries]);

  const { ref, inView } = useInViewOnce();

  return (
    <Card className="p-5" ref={ref}>
      <div className="flex items-start justify-between">
        <div>
          <Link
            to="/projects/$projectId"
            params={{ projectId }}
            className="text-base font-semibold leading-tight hover:text-primary"
          >
            {name}
          </Link>
          <div className="mt-0.5 text-[11px] text-muted">
            {team || 'no team'} · {projectId}
          </div>
        </div>
        <TrendBadge delta={trend} />
      </div>

      <div className="mt-4 grid grid-cols-3 gap-2">
        <Stat label="Total tests" value={totalTests.toLocaleString()} />
        <Stat
          label="Passed"
          value={passedTests.toLocaleString()}
          tone={totalTests ? 'text-status-passed-fg' : 'text-muted'}
        />
        <Stat
          label="Failed"
          value={failedTests.toLocaleString()}
          tone={failedTests > 0 ? 'text-status-failed-fg' : 'text-muted'}
        />
      </div>

      {(totalTests > 0 || skippedTests > 0) && (
        <div className="mt-1 flex items-center gap-3 text-[11px] text-muted">
          <span>
            Pass rate:{' '}
            <span
              className={cn(
                'font-medium tabular-nums',
                totalTests
                  ? passRate >= 0.9
                    ? 'text-status-passed-fg'
                    : passRate >= 0.7
                    ? 'text-status-flaky-fg'
                    : 'text-status-failed-fg'
                  : 'text-muted',
              )}
            >
              {totalTests ? `${Math.round(passRate * 100)}%` : '—'}
            </span>
          </span>
          {skippedTests > 0 && (
            <span>
              Skipped:{' '}
              <span className="font-medium tabular-nums text-foreground">
                {skippedTests.toLocaleString()}
              </span>
            </span>
          )}
          <span className="ml-auto">
            Avg{' '}
            {(() => {
              const nz = durationSeries.filter((v) => v > 0);
              return nz.length ? `${avg(nz).toFixed(1)}s/run` : '—';
            })()}
          </span>
        </div>
      )}

      <div className="mt-4">
        <div className="mb-1 flex items-center justify-between text-[10px] uppercase tracking-wider text-muted">
          <span>Pass rate per day</span>
          <span>{days}d</span>
        </div>
        {/* Sparkline rendering is deferred until the card scrolls into
            view, so the first paint stays cheap when the page renders
            many project cards. Placeholder keeps layout stable. */}
        {inView ? (
          <Sparkline
            values={passSeries}
            min={0}
            max={1}
            color="rgb(14 165 233)"
            fill="rgb(14 165 233 / 0.12)"
            className="h-12 w-full text-primary"
          />
        ) : (
          <div className="h-12 w-full" aria-hidden="true" />
        )}
      </div>

      <div className="mt-3">
        <div className="mb-1 flex items-center justify-between text-[10px] uppercase tracking-wider text-muted">
          <span>Avg duration (s)</span>
        </div>
        {inView ? (
          <Sparkline
            values={durationSeries}
            color="rgb(139 92 246)"
            fill="rgb(139 92 246 / 0.10)"
            className="h-10 w-full"
          />
        ) : (
          <div className="h-10 w-full" aria-hidden="true" />
        )}
      </div>

      <div className="mt-4 border-t border-border pt-3">
        <Link
          to="/projects/$projectId"
          params={{ projectId }}
          className="inline-flex w-full items-center justify-center gap-1.5 rounded-md border border-border bg-surface-2 px-3 py-1.5 text-xs font-medium text-foreground hover:bg-primary hover:text-white"
        >
          <History className="h-3.5 w-3.5" /> View history
        </Link>
      </div>
    </Card>
  );
}

function TrendBadge({ delta }: { delta: number }) {
  if (Math.abs(delta) < 0.02) {
    return (
      <span className="inline-flex items-center gap-1 rounded-full bg-status-skipped-bg px-2 py-0.5 text-[10px] font-medium text-status-skipped-fg">
        <Minus className="h-3 w-3" /> Stable
      </span>
    );
  }
  const up = delta > 0;
  return (
    <span
      className={cn(
        'inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[10px] font-medium',
        up
          ? 'bg-status-passed-bg text-status-passed-fg'
          : 'bg-status-failed-bg text-status-failed-fg',
      )}
    >
      {up ? <TrendingUp className="h-3 w-3" /> : <TrendingDown className="h-3 w-3" />}
      {(delta * 100).toFixed(1).replace(/^-/, '−')}pp
    </span>
  );
}

function Stat({ label, value, tone }: { label: string; value: string; tone?: string }) {
  return (
    <div>
      <div className="text-[10px] font-medium uppercase tracking-wider text-muted">
        {label}
      </div>
      <div className={cn('mt-0.5 text-base font-semibold tabular-nums', tone)}>
        {value}
      </div>
    </div>
  );
}

function avg(xs: number[]): number {
  if (xs.length === 0) return 0;
  return xs.reduce((s, x) => s + x, 0) / xs.length;
}
