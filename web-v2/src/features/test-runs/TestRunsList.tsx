import { useEffect, useMemo, useRef, useState } from 'react';
import { Link } from '@tanstack/react-router';
import { Bookmark, BookmarkPlus, Check, ChevronDown, X } from 'lucide-react';
import { Card } from '@/components/ui/Card';
import { Spinner } from '@/components/ui/Spinner';
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';
import { Modal } from '@/components/ui/Modal';
import { Pagination } from '@/components/ui/Pagination';
import { StatusBadge } from '@/components/ui/StatusBadge';
import { Table, TBody, TD, TH, THead, TR } from '@/components/ui/Table';
import { EmptyState } from '@/components/ui/EmptyState';
import { formatDuration } from '@/lib/duration';
import type { TestRunNode } from '@/lib/types';
import { cn } from '@/lib/cn';
import { useTestRuns, type TestRunsFilter } from './hooks';
import { FilterSidebar } from './FilterSidebar';
import { LabeledValue } from './LabeledValue';
import {
  TEST_RUNS_PAGE,
  useCreateSavedView,
  useSavedViews,
} from '../saved-views/integration';

// sessionStorage round-trips the filter + page-size so coming back from
// a test-run detail page doesn't reset the user's view. Per-tab scope
// (not localStorage) keeps a parallel tab independent.
const FILTER_KEY    = 'fern-v2.test-runs.filter';
const PAGE_SIZE_KEY = 'fern-v2.test-runs.pageSize';

function loadFilter(): TestRunsFilter {
  if (typeof window === 'undefined') return { first: 50 };
  try {
    const raw = window.sessionStorage.getItem(FILTER_KEY);
    if (!raw) return { first: 50 };
    const parsed = JSON.parse(raw) as TestRunsFilter;
    // Always reset the cursor on remount — a stale `after` would point
    // into a result set the user can't navigate around in (no prev
    // cursors in the stack to walk back through).
    return { ...parsed, after: undefined };
  } catch {
    return { first: 50 };
  }
}
function persistFilter(f: TestRunsFilter) {
  if (typeof window === 'undefined') return;
  // Drop `after` before persisting — the cursor stack lives in a ref
  // that doesn't round-trip, so persisting the cursor would leave the
  // user stranded on a non-first page they can't paginate back from.
  const { after: _after, ...rest } = f;
  window.sessionStorage.setItem(FILTER_KEY, JSON.stringify(rest));
}
function loadPageSize(): number {
  if (typeof window === 'undefined') return 50;
  const raw = window.sessionStorage.getItem(PAGE_SIZE_KEY);
  const n = raw ? Number(raw) : NaN;
  return Number.isFinite(n) && n > 0 ? n : 50;
}
function persistPageSize(n: number) {
  if (typeof window === 'undefined') return;
  window.sessionStorage.setItem(PAGE_SIZE_KEY, String(n));
}

// runDuration renders a run's wall-clock time.
//
// The server's recorded duration_ms wins: it is authoritative, and it is
// the only source for a run with no end_time (still running, or ended
// abnormally) — those used to render as an em dash. The end - start
// fallback keeps the column populated against a backend that predates
// duration_ms on this endpoint.
//
// A zero duration_ms is treated as "not recorded", not as a zero-length
// run: the field has no omitempty, so an unset duration arrives as 0,
// and rendering "0ms" for a run that is still going would be worse than
// falling back.
function runDuration(node: TestRunNode): string {
  if (node.duration_ms) return formatDuration(node.duration_ms);
  if (node.end_time) {
    return formatDuration(
      new Date(node.end_time).getTime() - new Date(node.start_time).getTime(),
    );
  }
  return '—';
}

export default function TestRunsList() {
  const [pageSize, setPageSizeState] = useState(() => loadPageSize());
  // The active filter for the API call. `after` flips when the user
  // pages forward; the cursorStack tracks history for back-navigation.
  const [filter, setFilterState] = useState<TestRunsFilter>(() => loadFilter());
  const cursorStackRef = useRef<string[]>([]); // cursors visited (excluding the current page's cursor)
  const [page, setPage] = useState(1);

  // Persist on every change so a navigate-away + back round-trip
  // restores the same view the user left.
  const setFilter: typeof setFilterState = (next) => {
    setFilterState((prev) => {
      const resolved = typeof next === 'function' ? next(prev) : next;
      persistFilter(resolved);
      return resolved;
    });
  };
  const setPageSize: typeof setPageSizeState = (next) => {
    setPageSizeState((prev) => {
      const resolved = typeof next === 'function' ? next(prev) : next;
      persistPageSize(resolved);
      return resolved;
    });
  };

  const { data, isLoading, isFetching, error } = useTestRuns(filter);

  // When the user changes any filter (status, branch, search, …), reset
  // pagination. We detect this by comparing the filter without `after`.
  // Anything that's not `first` or `after` is a "filter" knob.
  const filterKey = useMemo(() => {
    const { first: _f, after: _a, ...rest } = filter;
    return JSON.stringify(rest);
  }, [filter]);
  const prevFilterKey = useRef(filterKey);
  useEffect(() => {
    if (filterKey !== prevFilterKey.current) {
      prevFilterKey.current = filterKey;
      cursorStackRef.current = [];
      setPage(1);
      setFilter((f) => ({ ...f, after: undefined }));
    }
  }, [filterKey]);

  // Keep `first` in sync with the page-size selector.
  useEffect(() => {
    if (filter.first !== pageSize) {
      setFilter((f) => ({ ...f, first: pageSize, after: undefined }));
      cursorStackRef.current = [];
      setPage(1);
    }
  }, [pageSize, filter.first]);

  const goNext = () => {
    if (!data?.pageInfo.hasNextPage) return;
    cursorStackRef.current.push(filter.after ?? '');
    setFilter({ ...filter, after: data.pageInfo.endCursor });
    setPage((p) => p + 1);
  };
  const goPrev = () => {
    const prev = cursorStackRef.current.pop();
    if (prev === undefined) return;
    setFilter({ ...filter, after: prev || undefined });
    setPage((p) => Math.max(1, p - 1));
  };
  const goFirst = () => {
    cursorStackRef.current = [];
    setFilter({ ...filter, after: undefined });
    setPage(1);
  };

  return (
    <div className="grid gap-4 md:grid-cols-[260px_1fr]">
      <FilterSidebar
        filter={filter}
        onChange={setFilter}
        facets={data?.facets ?? { byStatus: [], byBranch: [], byTag: [], byProject: [] }}
      />

      <div className="space-y-3">
        <header className="flex flex-wrap items-baseline justify-between gap-2">
          <div>
            <h1 className="text-3xl font-semibold tracking-tight">Test runs</h1>
            <p className="mt-1 text-sm text-muted">
              {data
                ? `${data.totalCount.toLocaleString()}${data.totalCountIsEstimate ? ' (est.)' : ''} match`
                : '—'}
              {isFetching && <Spinner className="ml-2" />}
            </p>
          </div>
          <SavedViewsBar filter={filter} onApply={(f) => {
            cursorStackRef.current = [];
            setPage(1);
            setFilter({ first: pageSize, ...f });
          }} />
        </header>

        {error && (
          <EmptyState title="Couldn't load test runs" description={(error as Error).message} />
        )}

        {isLoading ? (
          <div className="flex items-center gap-2 text-muted">
            <Spinner /> Loading…
          </div>
        ) : (data?.edges.length ?? 0) === 0 ? (
          <EmptyState
            title="No runs match the current filter"
            description="Try clearing filters, or run `make docker-test-seed`."
          />
        ) : (
          <>
            <Card>
              <Table>
                <THead>
                  <TR>
                    <TH>Project</TH>
                    <TH>Run ID</TH>
                    <TH>Branch</TH>
                    <TH className="text-right">Test Results</TH>
                    <TH>Status</TH>
                    <TH className="text-right">Duration</TH>
                    <TH>Started</TH>
                  </TR>
                </THead>
                <TBody>
                  {data?.edges.map(({ node }) => (
                    <TR key={node.id}>
                      <TD>
                        <Link
                          to="/projects/$projectId"
                          params={{ projectId: node.project_id }}
                          className="text-foreground hover:text-primary"
                        >
                          <LabeledValue
                            value={node.project_id}
                            label={node.project_name}
                          />
                        </Link>
                      </TD>
                      <TD>
                        <Link
                          to="/test-runs/$runId"
                          params={{ runId: String(node.id) }}
                          className="text-primary hover:underline"
                        >
                          {node.run_id}
                        </Link>
                      </TD>
                      <TD className="font-mono text-xs">{node.branch || '—'}</TD>
                      <TD className="text-right tabular-nums text-sm">
                        <span className="text-green-600 dark:text-green-400" title="passed">
                          {node.passed_tests}
                        </span>
                        {' / '}
                        <span className="text-red-600 dark:text-red-400" title="failed">
                          {node.failed_tests}
                        </span>
                        {' / '}
                        <span className="text-muted" title="skipped">
                          {node.skipped_tests}
                        </span>
                      </TD>
                      <TD><StatusBadge status={node.status} /></TD>
                      <TD className="text-right tabular-nums">
                        {runDuration(node)}
                      </TD>
                      <TD className="text-xs text-muted">
                        {new Date(node.start_time).toLocaleString()}
                      </TD>
                    </TR>
                  ))}
                </TBody>
              </Table>
            </Card>

            <Pagination
              totalCount={data?.totalCount ?? 0}
              isEstimate={data?.totalCountIsEstimate ?? false}
              pageSize={pageSize}
              onPageSizeChange={setPageSize}
              page={page}
              hasNext={data?.pageInfo.hasNextPage ?? false}
              onFirst={goFirst}
              onPrev={goPrev}
              onNext={goNext}
              renderedCount={data?.edges.length ?? 0}
            />
          </>
        )}
      </div>
    </div>
  );
}

// ----- Saved views inline UI -----------------------------------------------

// canonicalFilter produces a stable string for a filter, ignoring
// pagination (first/after), empty arrays, and null/undefined, with array
// values sorted — so a saved view matches the live filter regardless of
// the order the user toggled facets in. Used to highlight the applied view.
function canonicalFilter(f: Record<string, unknown>): string {
  const norm: Record<string, unknown> = {};
  for (const k of Object.keys(f).sort()) {
    if (k === 'first' || k === 'after') continue;
    const v = f[k];
    if (v == null || v === '' || v === false) continue;
    if (Array.isArray(v)) {
      if (v.length === 0) continue;
      norm[k] = [...v].sort();
    } else {
      norm[k] = v;
    }
  }
  return JSON.stringify(norm);
}

function SavedViewsBar({
  filter,
  onApply,
}: {
  filter: TestRunsFilter;
  onApply: (f: TestRunsFilter) => void;
}) {
  const views = useSavedViews();
  const create = useCreateSavedView();
  const [showMenu, setShowMenu] = useState(false);
  const [showSave, setShowSave] = useState(false);
  const [name, setName] = useState('');

  const filterIsEmpty =
    !filter.project?.length &&
    !filter.status?.length &&
    !filter.branch?.length &&
    !filter.q &&
    !filter.tag?.length &&
    !filter.from &&
    !filter.to &&
    filter.durationGte == null &&
    filter.durationLte == null;

  // The shared query returns all pages' views; the bar shows only this page's.
  const list = (views.data?.views ?? []).filter((v) => v.page === TEST_RUNS_PAGE);
  const activeKey = canonicalFilter(filter as Record<string, unknown>);

  return (
    <div className="flex items-center gap-2">
      <div className="relative">
        <Button
          variant="secondary"
          size="sm"
          onClick={() => setShowMenu((s) => !s)}
        >
          <Bookmark className="h-3.5 w-3.5" />
          Views {list.length > 0 && <span className="text-muted">({list.length})</span>}
          <ChevronDown className="h-3 w-3" />
        </Button>
        {showMenu && (
          <div
            className="absolute right-0 z-20 mt-1 w-72 rounded-md border border-border bg-surface shadow-lg"
            onMouseLeave={() => setShowMenu(false)}
          >
            <div className="border-b border-border px-3 py-2 text-[11px] uppercase tracking-wider text-muted">
              Saved filters
            </div>
            {views.isLoading && (
              <div className="flex items-center gap-2 px-3 py-2 text-xs text-muted">
                <Spinner /> Loading…
              </div>
            )}
            {!views.isLoading && list.length === 0 && (
              <div className="px-3 py-2 text-xs text-muted">
                You haven't saved any views yet. Configure filters and click
                "Save view".
              </div>
            )}
            {list.map((v) => {
              const isActive = canonicalFilter(v.filter) === activeKey;
              return (
                <button
                  key={v.id}
                  className={cn(
                    'flex w-full items-center gap-2 px-3 py-1.5 text-left text-sm hover:bg-surface-2',
                    isActive ? 'text-primary' : 'text-foreground hover:text-primary',
                  )}
                  onClick={() => {
                    onApply((v.filter as TestRunsFilter) ?? {});
                    setShowMenu(false);
                  }}
                >
                  <Check className={cn('h-3.5 w-3.5 shrink-0', isActive ? 'opacity-100' : 'opacity-0')} />
                  <span className="flex-1 truncate">{v.name}</span>
                </button>
              );
            })}
            <div className="border-t border-border px-3 py-2 text-[11px]">
              <Link to="/saved-views" className="text-primary hover:underline">
                Manage saved views →
              </Link>
            </div>
          </div>
        )}
      </div>

      <Button
        variant="ghost"
        size="sm"
        onClick={() => onApply({})}
        disabled={filterIsEmpty}
        title={filterIsEmpty ? 'No filters to clear' : 'Clear all active filters'}
      >
        <X className="h-3.5 w-3.5" />
        Clear
      </Button>

      <Button
        variant="primary"
        size="sm"
        onClick={() => setShowSave(true)}
        disabled={filterIsEmpty}
        title={filterIsEmpty ? 'Configure at least one filter first' : 'Save the current filter'}
      >
        <BookmarkPlus className="h-3.5 w-3.5" />
        Save view
      </Button>

      <Modal
        open={showSave}
        onClose={() => {
          setShowSave(false);
          setName('');
        }}
        title="Save view"
        description="Captures every active filter on this page."
        footer={
          <>
            <Button variant="secondary" onClick={() => setShowSave(false)} disabled={create.isPending}>
              Cancel
            </Button>
            <Button
              disabled={!name.trim() || create.isPending}
              onClick={async () => {
                const { first: _f, after: _a, ...rest } = filter;
                await create.mutateAsync({
                  name: name.trim(),
                  filter: rest,
                });
                setName('');
                setShowSave(false);
              }}
            >
              {create.isPending ? <Spinner className="text-white" /> : 'Save'}
            </Button>
          </>
        }
      >
        <div className="space-y-2">
          <label htmlFor="save-view-name" className="block text-xs font-medium text-foreground">Name</label>
          <Input
            id="save-view-name"
            // Autofocus is correct UX in this save-view modal — the
            // dialog just opened on the user's "Save view" click and
            // the only sensible next action is to type a name.
            // eslint-disable-next-line jsx-a11y/no-autofocus
            autoFocus
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="e.g. Failed runs on main last 24h"
          />
          {create.error && (
            <div className={cn(
              'rounded border border-red-200 bg-red-50 px-2.5 py-1.5 text-xs text-red-800',
              'dark:border-red-900/40 dark:bg-red-950/30 dark:text-red-200',
            )}>
              {(create.error as Error).message}
            </div>
          )}
        </div>
      </Modal>
    </div>
  );
}
