import { useMemo, useRef, useState } from 'react';
import { Link, useParams } from '@tanstack/react-router';
import { useQuery } from '@tanstack/react-query';
import { ArrowLeft, ChevronDown, ChevronRight, Search, X } from 'lucide-react';
import { graphqlFetch } from '@/lib/api';
import { Card, CardBody, CardHeader, CardTitle } from '@/components/ui/Card';
import { Spinner } from '@/components/ui/Spinner';
import { StatusBadge } from '@/components/ui/StatusBadge';
import { EmptyState } from '@/components/ui/EmptyState';
import { SortableTH, Table, THead, TBody, TR, TH, TD } from '@/components/ui/Table';
import { FilterChips } from '@/components/ui/FilterChips';
import { Input } from '@/components/ui/Input';
import { Button } from '@/components/ui/Button';
import { formatDuration } from '@/lib/duration';
import { optionsWithCounts, STATUS_ORDER } from '@/lib/tableFilter';
import type { SortState } from '@/lib/tableSort';
import type { HistoryRun } from '@/lib/types';
import { LabeledValue } from './LabeledValue';
import { HistoryStrip } from './HistoryStrip';
import { SpecHistoryPanel } from './SpecHistoryPanel';
import { useRunHistory } from './hooks';
import { historyLookup, type HistoryLookup, type HistoryResults } from './runHistory';
import {
  EMPTY_ROW_FILTER,
  filterRows,
  isRowFilterActive,
  sortSpecs,
  sortSuites,
  tagLabel,
  type RowFilter,
  type SpecSortKey,
  type SuiteSortKey,
} from './runDetailFilters';

// v1-parity test-run detail with drill navigation:
//   /v2/test-runs/:runId  →  suites table  →  click a suite  →  specs table
//
// View state is local (matches v1's `useState('runs'|'suites'|'specs')`
// model — no URL change between suites and specs because v1 doesn't
// deep-link either, and the run is already addressable by its URL).
//
// Fetches the same field set v1's GET_TEST_RUN_DETAILS query did:
// tags at all three levels, metadata, stack traces, per-spec
// timestamps, skippedSpecs, retryCount.
const GET_RUN = /* GraphQL */ `
  query GetTestRun($id: ID!) {
    testRun(id: $id) {
      id
      projectId
      projectName
      runId
      branch
      commitSha
      status
      startTime
      endTime
      duration
      totalTests
      passedTests
      failedTests
      skippedTests
      environment
      metadata
      tags {
        id
        name
        category
        value
        color
      }
      suiteRuns {
        id
        suiteName
        status
        startTime
        endTime
        duration
        totalSpecs
        passedSpecs
        failedSpecs
        skippedSpecs
        tags {
          id
          name
          category
          value
          color
        }
        specRuns {
          id
          specName
          status
          startTime
          endTime
          duration
          errorMessage
          stackTrace
          retryCount
          isFlaky
          tags {
            id
            name
            category
            value
            color
          }
        }
      }
    }
  }
`;

interface TagRef {
  id: string;
  name: string;
  category: string | null;
  value: string | null;
  color: string | null;
}

interface SpecLike {
  id: string;
  specName: string;
  status: string;
  startTime: string;
  endTime: string | null;
  duration: number;
  errorMessage: string | null;
  stackTrace: string | null;
  retryCount: number;
  isFlaky: boolean;
  tags: TagRef[];
}

interface SuiteLike {
  id: string;
  suiteName: string;
  status: string;
  startTime: string;
  endTime: string | null;
  duration: number;
  totalSpecs: number;
  passedSpecs: number;
  failedSpecs: number;
  skippedSpecs: number;
  tags: TagRef[];
  specRuns: SpecLike[];
}

interface RunLike {
  id: string;
  projectId: string;
  projectName: string | null;
  runId: string;
  branch: string | null;
  commitSha: string | null;
  status: string;
  startTime: string;
  endTime: string | null;
  duration: number;
  totalTests: number;
  passedTests: number;
  failedTests: number;
  skippedTests: number;
  environment: string | null;
  metadata: unknown;
  tags: TagRef[];
  suiteRuns: SuiteLike[];
}

interface RunResp {
  testRun: RunLike | null;
}

// `metadata` is a JSON scalar — it can be an object, array, string, or
// null. Treat anything other than a non-empty object/array as "no
// metadata to show" so we don't render an empty collapsible section.
// Exported alongside the component so the unit tests can exercise the
// predicate directly without rendering the page.
// eslint-disable-next-line react-refresh/only-export-components
export function hasDisplayableMetadata(m: unknown): boolean {
  if (m == null) return false;
  if (Array.isArray(m)) return m.length > 0;
  if (typeof m === 'object') return Object.keys(m as object).length > 0;
  return false;
}

export default function TestRunDetail() {
  const { runId } = useParams({ from: '/test-runs/$runId' });
  const { data, isLoading, error } = useQuery({
    queryKey: ['test-run', runId],
    queryFn: () => graphqlFetch<RunResp>(GET_RUN, { id: runId }),
  });
  const [selectedSuiteId, setSelectedSuiteId] = useState<string | null>(null);
  // Table view state lives here rather than in SuitesView/SpecsView:
  // drilling into a suite unmounts the suites table, and the user's
  // sort and filter should still be there when they come back. Spec
  // state carries across suites for the same reason (e.g. "hide
  // skipped" stays on while stepping through suites).
  const [suiteView, setSuiteView] = useState<TableView<SuiteSortKey>>(EMPTY_VIEW);
  const [specView, setSpecView] = useState<TableView<SpecSortKey>>(EMPTY_VIEW);
  // The history strip loads separately and never blocks the page: until
  // it arrives (or if it fails) the History cells show a dash.
  const historyQuery = useRunHistory(data?.testRun?.id);
  const history = useMemo<HistoryView>(
    () => ({ runs: historyQuery.data?.runs ?? [], lookup: historyLookup(historyQuery.data) }),
    [historyQuery.data],
  );

  if (isLoading) {
    return (
      <div className="flex items-center gap-2 text-muted">
        <Spinner /> Loading run {runId}…
      </div>
    );
  }
  if (error) {
    return <EmptyState title="Couldn't load run" description={(error as Error).message} />;
  }
  const run = data?.testRun;
  if (!run) {
    return <EmptyState title={`Run ${runId} not found`} />;
  }

  const suites = run.suiteRuns ?? [];
  const selectedSuite = selectedSuiteId
    ? suites.find((s) => s.id === selectedSuiteId) ?? null
    : null;

  return (
    <div className="space-y-4">
      <Link
        to="/test-runs"
        className="inline-flex items-center gap-1 text-sm text-muted hover:text-foreground"
      >
        <ArrowLeft className="h-3 w-3" /> All test runs
      </Link>

      <RunHeader run={run} />

      {hasDisplayableMetadata(run.metadata) && <MetadataPanel value={run.metadata} />}

      {suites.length === 0 ? (
        <EmptyState
          title="No suite details available"
          description="This run was recorded without suite information (or the seeder skipped suites)."
        />
      ) : selectedSuite ? (
        <SpecsView
          runDbId={run.id}
          suite={selectedSuite}
          history={history}
          view={specView}
          onViewChange={setSpecView}
          onBack={() => setSelectedSuiteId(null)}
        />
      ) : (
        <SuitesView
          suites={suites}
          history={history}
          view={suiteView}
          onViewChange={setSuiteView}
          onSelect={(id) => setSelectedSuiteId(id)}
        />
      )}
    </div>
  );
}

function RunHeader({ run }: { run: RunLike }) {
  return (
    <header className="space-y-2">
      <div className="flex items-center gap-2">
        <h1 className="text-2xl font-semibold">{run.runId}</h1>
        <StatusBadge status={run.status} />
      </div>
      <div className="text-sm text-muted">
        <Link
          to="/projects/$projectId"
          params={{ projectId: run.projectId }}
          className="hover:text-foreground"
        >
          <LabeledValue value={run.projectId} label={run.projectName ?? undefined} />
        </Link>{' '}
        · {run.branch || 'no branch'} · {new Date(run.startTime).toLocaleString()}
      </div>
      {run.tags.length > 0 && (
        <div className="flex flex-wrap gap-1" data-testid="run-tags">
          {run.tags.map((t) => (
            <TagChip key={t.id} tag={t} />
          ))}
        </div>
      )}
      <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
        <Stat label="Total"   value={run.totalTests} />
        <Stat label="Passed"  value={run.passedTests}  tone="text-green-600 dark:text-green-400" />
        <Stat label="Failed"  value={run.failedTests}  tone="text-red-600 dark:text-red-400" />
        <Stat label="Skipped" value={run.skippedTests} tone="text-amber-600 dark:text-amber-400" />
      </div>
      <div className="text-xs text-muted">
        Duration {formatDuration(run.duration)}
        {run.environment ? ` · env ${run.environment}` : ''}
        {run.commitSha ? ` · commit ${run.commitSha.slice(0, 8)}` : ''}
      </div>
    </header>
  );
}

interface TableView<K extends string> {
  sort: SortState<K> | null;
  filter: RowFilter;
}

const EMPTY_VIEW = { sort: null, filter: EMPTY_ROW_FILTER };

interface HistoryView {
  runs: HistoryRun[];
  lookup: HistoryLookup;
}

// HistoryColumn renders a row's strip, or a dash while the history is
// loading, failed, or has nothing for this suite or spec.
function HistoryColumn({ runs, results }: { runs: HistoryRun[]; results: HistoryResults | undefined }) {
  return results && results.length > 0 ? (
    <HistoryStrip runs={runs} results={results} />
  ) : (
    <span className="text-muted">—</span>
  );
}

function SuitesView({
  suites,
  history,
  view,
  onViewChange,
  onSelect,
}: {
  suites: SuiteLike[];
  history: HistoryView;
  view: TableView<SuiteSortKey>;
  onViewChange: (next: TableView<SuiteSortKey>) => void;
  onSelect: (suiteId: string) => void;
}) {
  const rows = sortSuites(filterRows(suites, view.filter, (s) => [s.suiteName]), view.sort);
  const setSort = (sort: SortState<SuiteSortKey> | null) => onViewChange({ ...view, sort });
  return (
    <Card>
      <CardHeader className="space-y-2">
        <CardTitle>Test Suites ({suites.length})</CardTitle>
        <RowFilterBar
          rows={suites}
          shown={rows.length}
          filter={view.filter}
          onChange={(filter) => onViewChange({ ...view, filter })}
          searchPlaceholder="Search suites…"
        />
      </CardHeader>
      <CardBody className="overflow-x-auto p-0">
        {rows.length === 0 ? (
          <EmptyState title="No suites match the current filter" />
        ) : (
        <Table>
          <THead>
            <TR>
              <SortableTH sortKey="name" sort={view.sort} onSortChange={setSort}>Suite Name</SortableTH>
              <SortableTH
                sortKey="failed"
                sort={view.sort}
                onSortChange={setSort}
                className="text-right"
                title="Sorts by failed count"
              >
                Test Results
              </SortableTH>
              <SortableTH sortKey="status" sort={view.sort} onSortChange={setSort}>Status</SortableTH>
              <TH title="This suite in the project's recent runs on the same branch, oldest left">History</TH>
              <SortableTH sortKey="duration" sort={view.sort} onSortChange={setSort} className="text-right">
                Duration
              </SortableTH>
              <TH>Tags</TH>
            </TR>
          </THead>
          <TBody>
            {rows.map((s) => (
              <TR
                key={s.id}
                role="button"
                tabIndex={0}
                onClick={() => onSelect(s.id)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter' || e.key === ' ') {
                    e.preventDefault();
                    onSelect(s.id);
                  }
                }}
                className="cursor-pointer hover:bg-surface-2"
                data-testid="suite-row"
              >
                <TD>
                  <div className="font-medium">{s.suiteName}</div>
                  <div className="text-[11px] text-muted">{s.totalSpecs} specs</div>
                </TD>
                <TD className="text-right tabular-nums text-sm">
                  <span className="text-green-600 dark:text-green-400" title="passed">{s.passedSpecs}</span>
                  {' / '}
                  <span className="text-red-600 dark:text-red-400"   title="failed">{s.failedSpecs}</span>
                  {' / '}
                  <span className="text-muted"    title="skipped">{s.skippedSpecs}</span>
                </TD>
                <TD><StatusBadge status={s.status} /></TD>
                <TD>
                  <HistoryColumn runs={history.runs} results={history.lookup.suite(s.suiteName)} />
                </TD>
                <TD className="text-right tabular-nums">{formatDuration(s.duration)}</TD>
                <TD>
                  {s.tags.length > 0 ? (
                    <span className="flex flex-wrap gap-1" data-testid="suite-tags">
                      {s.tags.map((t) => (
                        <TagChip key={t.id} tag={t} />
                      ))}
                    </span>
                  ) : (
                    <span className="text-muted">—</span>
                  )}
                </TD>
              </TR>
            ))}
          </TBody>
        </Table>
        )}
      </CardBody>
    </Card>
  );
}

function SpecsView({
  runDbId,
  suite,
  history,
  view,
  onViewChange,
  onBack,
}: {
  runDbId: string;
  suite: SuiteLike;
  history: HistoryView;
  view: TableView<SpecSortKey>;
  onViewChange: (next: TableView<SpecSortKey>) => void;
  onBack: () => void;
}) {
  const rows = sortSpecs(
    filterRows(suite.specRuns, view.filter, (s) => [s.specName, s.errorMessage]),
    view.sort,
  );
  const setSort = (sort: SortState<SpecSortKey> | null) => onViewChange({ ...view, sort });
  return (
    <Card>
      <CardHeader className="space-y-2">
        <button
          type="button"
          onClick={onBack}
          className="inline-flex items-center gap-1 text-sm text-muted hover:text-foreground"
          data-testid="back-to-suites"
        >
          <ArrowLeft className="h-3 w-3" /> All suites
        </button>
        <div className="flex items-center justify-between">
          <div>
            <CardTitle>Test Specs — {suite.suiteName}</CardTitle>
            <div className="text-xs text-muted">
              {suite.totalSpecs} specs ({suite.passedSpecs} passed, {suite.failedSpecs} failed
              {suite.skippedSpecs > 0 ? `, ${suite.skippedSpecs} skipped` : ''}) ·{' '}
              {formatDuration(suite.duration)}
            </div>
          </div>
          <StatusBadge status={suite.status} />
        </div>
        {suite.tags.length > 0 && (
          <div className="flex flex-wrap gap-1" data-testid="suite-tags">
            {suite.tags.map((t) => (
              <TagChip key={t.id} tag={t} />
            ))}
          </div>
        )}
        {suite.specRuns.length > 0 && (
          <RowFilterBar
            rows={suite.specRuns}
            shown={rows.length}
            filter={view.filter}
            onChange={(filter) => onViewChange({ ...view, filter })}
            searchPlaceholder="Search name or error…"
            showHideSkipped
          />
        )}
      </CardHeader>
      <CardBody className="overflow-x-auto p-0">
        {suite.specRuns.length === 0 ? (
          <EmptyState
            title="No spec details available"
            description="This suite was recorded without per-spec rows."
          />
        ) : rows.length === 0 ? (
          <EmptyState title="No specs match the current filter" />
        ) : (
          <Table>
            <THead>
              <TR>
                <SortableTH sortKey="name" sort={view.sort} onSortChange={setSort}>Test Name</SortableTH>
                <SortableTH sortKey="status" sort={view.sort} onSortChange={setSort}>Status</SortableTH>
                <TH title="This spec in the project's recent runs on the same branch, oldest left">History</TH>
                <SortableTH sortKey="duration" sort={view.sort} onSortChange={setSort} className="text-right">
                  Duration
                </SortableTH>
                <SortableTH sortKey="error" sort={view.sort} onSortChange={setSort}>Error Message</SortableTH>
                <TH>Tags</TH>
                <SortableTH sortKey="started" sort={view.sort} onSortChange={setSort}>Started</SortableTH>
              </TR>
            </THead>
            <TBody>
              {rows.map((spec) => (
                <SpecRow
                  key={spec.id}
                  runDbId={runDbId}
                  suiteName={suite.suiteName}
                  spec={spec}
                  historyRuns={history.runs}
                  history={history.lookup.spec(suite.suiteName, spec.specName)}
                />
              ))}
            </TBody>
          </Table>
        )}
      </CardBody>
    </Card>
  );
}

function SpecRow({
  runDbId,
  suiteName,
  spec,
  historyRuns,
  history,
}: {
  runDbId: string;
  suiteName: string;
  spec: SpecLike;
  historyRuns: HistoryRun[];
  history: HistoryResults | undefined;
}) {
  const [open, setOpen] = useState(false);
  const [historyOpen, setHistoryOpen] = useState(false);
  const historyToggle = useRef<HTMLButtonElement>(null);
  const panelId = `spec-history-${spec.id}`;
  // Closing from inside the panel removes the focused button, so hand
  // focus back to the strip that opened it.
  const closeHistory = () => {
    setHistoryOpen(false);
    historyToggle.current?.focus();
  };
  return (
    <>
      <TR data-testid="spec-row">
        <TD>
          <div className="font-medium">{spec.specName}</div>
          {(spec.isFlaky || spec.retryCount > 0) && (
            <div className="text-[11px] text-amber-600 dark:text-amber-400">
              {spec.isFlaky ? 'flaky' : null}
              {spec.isFlaky && spec.retryCount > 0 ? ' · ' : ''}
              {spec.retryCount > 0 ? (
                <span data-testid="spec-retry-count">retried ×{spec.retryCount}</span>
              ) : null}
            </div>
          )}
        </TD>
        <TD><StatusBadge status={spec.status} /></TD>
        <TD>
          {history && history.length > 0 ? (
            <button
              ref={historyToggle}
              type="button"
              onClick={() => setHistoryOpen((v) => !v)}
              aria-expanded={historyOpen}
              aria-controls={panelId}
              title={historyOpen ? 'Hide history' : 'Show duration and failures across these runs'}
              className="inline-flex items-center gap-1 rounded p-0.5 text-muted hover:bg-surface-2 hover:text-foreground focus-visible:outline focus-visible:outline-2"
              data-testid="spec-history-toggle"
            >
              <HistoryStrip runs={historyRuns} results={history} />
              {historyOpen ? (
                <ChevronDown className="h-3 w-3 shrink-0" aria-hidden />
              ) : (
                <ChevronRight className="h-3 w-3 shrink-0" aria-hidden />
              )}
            </button>
          ) : (
            <HistoryColumn runs={historyRuns} results={history} />
          )}
        </TD>
        <TD className="text-right tabular-nums">{formatDuration(spec.duration)}</TD>
        <TD>
          {spec.errorMessage ? (
            <button
              type="button"
              onClick={() => setOpen((v) => !v)}
              className="inline-flex max-w-[28rem] items-center gap-1 truncate rounded bg-red-50 px-1.5 py-0.5 text-left font-mono text-[11px] text-red-800 hover:bg-red-100"
              title={spec.errorMessage}
              aria-expanded={open}
              data-testid="spec-error-message"
            >
              {open ? <ChevronDown className="h-3 w-3 shrink-0" /> : <ChevronRight className="h-3 w-3 shrink-0" />}
              <span className="truncate">{spec.errorMessage}</span>
            </button>
          ) : (
            <span className="text-muted">—</span>
          )}
        </TD>
        <TD>
          {spec.tags.length > 0 ? (
            <span className="flex flex-wrap gap-1" data-testid="spec-tags">
              {spec.tags.map((t) => (
                <TagChip key={t.id} tag={t} />
              ))}
            </span>
          ) : (
            <span className="text-muted">—</span>
          )}
        </TD>
        <TD className="text-xs text-muted">
          {spec.startTime ? new Date(spec.startTime).toLocaleString() : '—'}
        </TD>
      </TR>
      {open && (spec.errorMessage || spec.stackTrace) && (
        <TR>
          <TD colSpan={7}>
            <pre
              className="overflow-x-auto rounded bg-surface-2 p-2 font-mono text-[11px] text-foreground"
              data-testid="spec-stack-trace"
            >
{spec.stackTrace ?? spec.errorMessage}
            </pre>
          </TD>
        </TR>
      )}
      {historyOpen && (
        <TR id={panelId} className="bg-surface-2/40">
          <TD colSpan={7}>
            <SpecHistoryPanel
              runDbId={runDbId}
              suiteName={suiteName}
              specName={spec.specName}
              onClose={closeHistory}
            />
          </TD>
        </TR>
      )}
    </>
  );
}

function TagChip({ tag }: { tag: TagRef }) {
  const label = tagLabel(tag);
  const style = tag.color ? { backgroundColor: `${tag.color}22`, color: tag.color } : undefined;
  return (
    <span
      className="inline-flex items-center rounded-full border border-border bg-surface-2 px-2 py-0.5 text-[10px] font-medium text-muted"
      style={style}
      data-testid="tag-chip"
    >
      {label}
    </span>
  );
}

function MetadataPanel({ value }: { value: unknown }) {
  const [open, setOpen] = useState(false);
  return (
    <Card>
      <CardHeader>
        <button
          type="button"
          onClick={() => setOpen((v) => !v)}
          className="flex items-center gap-1 text-sm font-medium text-foreground"
          aria-expanded={open}
          data-testid="metadata-toggle"
        >
          {open ? <ChevronDown className="h-3 w-3" /> : <ChevronRight className="h-3 w-3" />}
          Metadata
        </button>
      </CardHeader>
      {open && (
        <CardBody>
          <pre
            className="overflow-x-auto rounded bg-surface-2 p-2 font-mono text-xs text-foreground"
            data-testid="metadata-content"
          >
{JSON.stringify(value, null, 2)}
          </pre>
        </CardBody>
      )}
    </Card>
  );
}

function Stat({
  label,
  value,
  tone,
}: {
  label: string;
  value: string | number;
  tone?: string;
}) {
  return (
    <Card>
      <CardBody>
        <div className="text-xs uppercase tracking-wider text-muted">{label}</div>
        <div className={`mt-1 text-xl font-semibold tabular-nums ${tone ?? ''}`}>{value}</div>
      </CardBody>
    </Card>
  );
}

// RowFilterBar is the search box and status/tag chips above the suites
// and specs tables. Chip options come from the rows themselves, with
// counts, so only values that actually occur are offered.
function RowFilterBar({
  rows,
  shown,
  filter,
  onChange,
  searchPlaceholder,
  showHideSkipped = false,
}: {
  rows: { status: string; tags: TagRef[] }[];
  shown: number;
  filter: RowFilter;
  onChange: (next: RowFilter) => void;
  searchPlaceholder: string;
  showHideSkipped?: boolean;
}) {
  const statusOptions = optionsWithCounts(
    rows.map((r) => r.status).filter((s) => !(filter.hideSkipped && s === 'skipped')),
    STATUS_ORDER,
  );
  const tagOptions = optionsWithCounts(rows.flatMap((r) => r.tags.map(tagLabel)));
  return (
    <div className="space-y-2 border-t border-border pt-2" data-testid="row-filter-bar">
      <div className="flex flex-wrap items-center gap-3">
        <div className="relative w-64">
          <Search className="absolute left-2 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-muted" />
          <Input
            type="search"
            value={filter.q}
            onChange={(e) => onChange({ ...filter, q: e.target.value })}
            placeholder={searchPlaceholder}
            aria-label={searchPlaceholder}
            className="py-1 pl-7"
          />
        </div>
        {showHideSkipped && (
          <label className="inline-flex items-center gap-1.5 text-xs text-foreground">
            <input
              type="checkbox"
              checked={filter.hideSkipped}
              onChange={(e) => onChange({ ...filter, hideSkipped: e.target.checked })}
              data-testid="hide-skipped"
            />
            Hide skipped
          </label>
        )}
        <span className="ml-auto text-xs text-muted">
          Showing {shown} of {rows.length}
        </span>
        {isRowFilterActive(filter) && (
          <Button variant="ghost" size="sm" onClick={() => onChange(EMPTY_ROW_FILTER)}>
            <X className="h-3.5 w-3.5" />
            Clear
          </Button>
        )}
      </div>
      <FilterChips
        label="Status"
        options={statusOptions}
        selected={filter.statuses}
        onChange={(statuses) => onChange({ ...filter, statuses })}
      />
      <FilterChips
        label="Tags"
        options={tagOptions}
        selected={filter.tags}
        onChange={(tags) => onChange({ ...filter, tags })}
      />
    </div>
  );
}
