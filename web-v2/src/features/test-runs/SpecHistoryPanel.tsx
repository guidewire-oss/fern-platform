import { useEffect, useRef, useState } from 'react';
import { Link } from '@tanstack/react-router';
import { ChevronDown, ChevronRight, X } from 'lucide-react';
import { cn } from '@/lib/cn';
import { formatDuration } from '@/lib/duration';
import { Spinner } from '@/components/ui/Spinner';
import { useSpecHistory } from './hooks';
import { CHART, columnPath, durationLayout, failedRuns, type DurationLayout, type FailedRun } from './specHistory';
import { STATUS_FILL } from './statusColors';
import type { SpecRunHistory } from '@/lib/types';

// SpecHistoryPanel opens under a spec row when its history strip is
// clicked: the spec's duration in each run of the strip's window, and the
// error of every failed run. Its data (which carries the error text) is
// fetched only while the panel is open.
export function SpecHistoryPanel({
  runDbId,
  suiteName,
  specName,
  onClose,
}: {
  runDbId: string;
  suiteName: string;
  specName: string;
  onClose: () => void;
}) {
  const { data, isLoading, error } = useSpecHistory(runDbId, suiteName, specName, true);

  // Escape closes the panel while focus is inside it. A native listener
  // on the container, since the container itself isn't interactive.
  const container = useRef<HTMLDivElement>(null);
  const close = useRef(onClose);
  close.current = onClose;
  useEffect(() => {
    const el = container.current;
    if (!el) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        e.stopPropagation();
        close.current();
      }
    };
    el.addEventListener('keydown', onKey);
    return () => el.removeEventListener('keydown', onKey);
  }, []);

  // The close button is on every state, so a panel that is still loading
  // or failed can be dismissed too.
  return (
    <div ref={container} className="relative" data-testid="spec-history-panel">
      <button
        type="button"
        onClick={onClose}
        className="absolute right-2 top-2 inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-xs text-muted hover:bg-surface-2 hover:text-foreground"
        data-testid="spec-history-close"
      >
        <X className="h-3 w-3" aria-hidden /> Close
      </button>
      <PanelBody data={data} isLoading={isLoading} error={error} />
    </div>
  );
}

function PanelBody({
  data,
  isLoading,
  error,
}: {
  data: SpecRunHistory | undefined;
  isLoading: boolean;
  error: unknown;
}) {
  if (isLoading) {
    return (
      <div className="flex items-center gap-2 p-3 text-xs text-muted">
        <Spinner /> Loading history…
      </div>
    );
  }
  if (error || !data || !Array.isArray(data.results)) {
    return <div className="p-3 text-xs text-muted">Couldn't load this spec's history.</div>;
  }

  const layout = durationLayout(data);
  const failures = failedRuns(data);
  const branch = data.runs[data.runs.length - 1]?.branch;
  return (
    <div className="space-y-4 p-3 pr-20">
      <section className="space-y-1">
        <div className="flex flex-wrap items-baseline justify-between gap-2">
          <h3 className="text-xs font-semibold">
            Duration, last {data.runs.length} runs{branch ? ` on ${branch}` : ''}
          </h3>
          <span className="text-xs text-muted tabular-nums" data-testid="duration-summary">
            {layout.medianMs === null
              ? 'not in any of these runs'
              : `median ${formatDuration(layout.medianMs)} · longest ${formatDuration(layout.maxMs)}`}
          </span>
        </div>
        <DurationChart layout={layout} />
      </section>
      <section className="space-y-1">
        <h3 className="text-xs font-semibold">
          Failures ({failures.length} of {data.runs.length} runs)
        </h3>
        {failures.length === 0 ? (
          <p className="text-xs text-muted">No failures in these runs.</p>
        ) : (
          <ul className="space-y-1">
            {failures.map((f) => (
              <FailureItem key={f.run.id} failure={f} />
            ))}
          </ul>
        )}
      </section>
    </div>
  );
}

// Extra room under the baseline for the viewed-run marker.
const MARKER_SPACE = 6;

function DurationChart({ layout }: { layout: DurationLayout }) {
  const base = CHART.plotHeight;
  const label =
    layout.medianMs === null
      ? 'Duration chart: the spec was not in any of these runs'
      : `Duration per run: median ${formatDuration(layout.medianMs)}, longest ${formatDuration(layout.maxMs)}`;
  return (
    <svg
      role="img"
      aria-label={label}
      width={layout.width}
      height={base + MARKER_SPACE}
      viewBox={`0 0 ${layout.width} ${base + MARKER_SPACE}`}
      className="block overflow-visible"
      data-testid="duration-chart"
    >
      <line x1={0} x2={layout.width} y1={base} y2={base} className="stroke-border" strokeWidth={1} />
      {layout.bars.map((b) => {
        const when = b.run ? new Date(b.run.startTime).toLocaleString() : '';
        const branch = b.run?.branch ? ` · ${b.run.branch}` : '';
        const what = b.cell ? `${b.cell.status} · ${formatDuration(b.cell.durationMs)}` : 'not in this run';
        return (
          <g
            key={b.run?.id ?? b.index}
            data-testid="duration-bar"
            data-status={b.cell?.status ?? 'absent'}
            data-viewed={b.viewed || undefined}
          >
            <title>{`${when}${branch} · ${what}${b.viewed ? ' (this run)' : ''}`}</title>
            {/* The hit target covers the whole slot, not just the bar. */}
            <rect x={b.x} y={0} width={CHART.barWidth + CHART.gap} height={base + MARKER_SPACE} fill="transparent" />
            {b.cell ? (
              <path
                d={columnPath(b.x, base, CHART.barWidth, Math.max(b.height, 1), CHART.radius)}
                className={STATUS_FILL[b.cell.status] ?? 'fill-gray-400'}
              />
            ) : (
              <line
                x1={b.x + 2}
                x2={b.x + CHART.barWidth - 2}
                y1={base - 1}
                y2={base - 1}
                className="stroke-muted"
                strokeWidth={1}
                strokeDasharray="2 2"
              />
            )}
            {b.viewed && (
              <rect x={b.x} y={base + 3} width={CHART.barWidth} height={2} rx={1} className="fill-foreground" />
            )}
          </g>
        );
      })}
      {layout.medianY !== null && (
        <line
          x1={0}
          x2={layout.width}
          y1={layout.medianY}
          y2={layout.medianY}
          className="stroke-muted"
          strokeWidth={1}
          strokeDasharray="4 3"
          data-testid="median-line"
          pointerEvents="none"
        />
      )}
    </svg>
  );
}

function FailureItem({ failure }: { failure: FailedRun }) {
  const [open, setOpen] = useState(false);
  const { run, cell, viewed } = failure;
  return (
    <li className="space-y-1 text-xs" data-testid="failed-run">
      <div className="flex flex-wrap items-center gap-2">
        <Link
          to="/test-runs/$runId"
          params={{ runId: String(run.id) }}
          className="text-muted hover:text-foreground"
        >
          {new Date(run.startTime).toLocaleString()}
        </Link>
        {run.branch && <span className="text-muted">· {run.branch}</span>}
        {viewed && <span className="text-muted">· this run</span>}
      </div>
      {cell.errorMessage ? (
        <button
          type="button"
          onClick={() => setOpen((v) => !v)}
          className="inline-flex max-w-full items-center gap-1 rounded bg-red-50 px-1.5 py-0.5 text-left font-mono text-[11px] text-red-800 hover:bg-red-100"
          aria-expanded={open}
          data-testid="failed-run-error"
        >
          {open ? <ChevronDown className="h-3 w-3 shrink-0" /> : <ChevronRight className="h-3 w-3 shrink-0" />}
          <span className={cn(!open && 'truncate')}>{cell.errorMessage}</span>
        </button>
      ) : (
        <span className="text-muted">No error message recorded.</span>
      )}
      {open && cell.stackTrace && (
        <pre
          className="overflow-x-auto rounded bg-surface-2 p-2 font-mono text-[11px] text-foreground"
          data-testid="failed-run-stack"
        >
          {cell.stackTrace}
        </pre>
      )}
    </li>
  );
}
