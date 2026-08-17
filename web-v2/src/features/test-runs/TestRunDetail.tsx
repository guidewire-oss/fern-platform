import { useState } from 'react';
import { Link, useParams } from '@tanstack/react-router';
import { useQuery } from '@tanstack/react-query';
import { ArrowLeft, ChevronDown, ChevronRight } from 'lucide-react';
import { graphqlFetch } from '@/lib/api';
import { Card, CardBody, CardHeader, CardTitle } from '@/components/ui/Card';
import { Spinner } from '@/components/ui/Spinner';
import { StatusBadge } from '@/components/ui/StatusBadge';
import { EmptyState } from '@/components/ui/EmptyState';
import { Table, THead, TBody, TR, TH, TD } from '@/components/ui/Table';
import { formatDuration } from '@/lib/duration';
import { LabeledValue } from './LabeledValue';

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
          suite={selectedSuite}
          onBack={() => setSelectedSuiteId(null)}
        />
      ) : (
        <SuitesView suites={suites} onSelect={(id) => setSelectedSuiteId(id)} />
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

function SuitesView({
  suites,
  onSelect,
}: {
  suites: SuiteLike[];
  onSelect: (suiteId: string) => void;
}) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>Test Suites ({suites.length})</CardTitle>
      </CardHeader>
      <CardBody className="overflow-x-auto p-0">
        <Table>
          <THead>
            <TR>
              <TH>Suite Name</TH>
              <TH className="text-right">Test Results</TH>
              <TH>Status</TH>
              <TH className="text-right">Duration</TH>
              <TH>Tags</TH>
            </TR>
          </THead>
          <TBody>
            {suites.map((s) => (
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
      </CardBody>
    </Card>
  );
}

function SpecsView({
  suite,
  onBack,
}: {
  suite: SuiteLike;
  onBack: () => void;
}) {
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
      </CardHeader>
      <CardBody className="overflow-x-auto p-0">
        {suite.specRuns.length === 0 ? (
          <EmptyState
            title="No spec details available"
            description="This suite was recorded without per-spec rows."
          />
        ) : (
          <Table>
            <THead>
              <TR>
                <TH>Test Name</TH>
                <TH>Status</TH>
                <TH className="text-right">Duration</TH>
                <TH>Error Message</TH>
                <TH>Tags</TH>
                <TH>Started</TH>
              </TR>
            </THead>
            <TBody>
              {suite.specRuns.map((spec) => (
                <SpecRow key={spec.id} spec={spec} />
              ))}
            </TBody>
          </Table>
        )}
      </CardBody>
    </Card>
  );
}

function SpecRow({ spec }: { spec: SpecLike }) {
  const [open, setOpen] = useState(false);
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
          <TD colSpan={6}>
            <pre
              className="overflow-x-auto rounded bg-surface-2 p-2 font-mono text-[11px] text-foreground"
              data-testid="spec-stack-trace"
            >
{spec.stackTrace ?? spec.errorMessage}
            </pre>
          </TD>
        </TR>
      )}
    </>
  );
}

function TagChip({ tag }: { tag: TagRef }) {
  // `name` is sometimes the raw, unprocessed "category:value" string (e.g.
  // Ginkgo labels reported as-is) and sometimes a distinct human label with
  // category/value tracked separately (e.g. "team" / owner / platform).
  // Only substitute in "category: value" when name is exactly that raw
  // compound -- otherwise appending value to name would double it up.
  const isRawCompound = tag.value && tag.name === `${tag.category}:${tag.value}`;
  const label = !tag.value ? tag.name : isRawCompound ? `${tag.category}: ${tag.value}` : `${tag.name}: ${tag.value}`;
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
