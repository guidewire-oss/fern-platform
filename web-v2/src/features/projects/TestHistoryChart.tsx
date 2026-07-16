import { useMemo, useState } from 'react';
import * as d3 from 'd3';
import { Card } from '@/components/ui/Card';
import { EmptyState } from '@/components/ui/EmptyState';
import type { TestRunNode } from '@/lib/types';
import { toHistoryPoints, type HistoryPoint } from './historyChartData';

// Stacked area chart of passed / failed / skipped counts across the last
// 20 runs of a project — a v2 port of v1's TestHistoryChart. d3 owns the
// scales/stack/area generators; React owns the DOM (no useEffect-driven
// imperative selection.append).

const COLORS = {
  passed:  '#10b981',
  failed:  '#ef4444',
  skipped: '#6b7280',
} as const;

const STACK_KEYS = ['passed', 'failed', 'skipped'] as const;
type StackKey = (typeof STACK_KEYS)[number];

const VIEW_W = 1000;
const VIEW_H = 360;
const MARGIN = { top: 16, right: 28, bottom: 48, left: 56 };
const PLOT_W = VIEW_W - MARGIN.left - MARGIN.right;
const PLOT_H = VIEW_H - MARGIN.top - MARGIN.bottom;

export function TestHistoryChart({ runs }: { runs: readonly TestRunNode[] }) {
  const points = useMemo(() => toHistoryPoints(runs, 20), [runs]);

  if (points.length === 0) {
    return (
      <EmptyState
        title="No history yet"
        description="Once this project has test runs, you'll see the pass/fail trend here."
      />
    );
  }

  return (
    <Card className="p-4">
      <div className="mb-3 flex flex-wrap items-baseline justify-between gap-2">
        <div>
          <h2 className="text-base font-semibold tracking-tight">Test history</h2>
          <p className="text-xs text-muted">
            Passed / failed / skipped per run, last {points.length} runs (oldest left).
          </p>
        </div>
        <Legend />
      </div>
      <Chart points={points} />
    </Card>
  );
}

function Chart({ points }: { points: HistoryPoint[] }) {
  const [hover, setHover] = useState<{
    point: HistoryPoint;
    layer: StackKey;
    cx: number;
    cy: number;
  } | null>(null);

  const maxTotal = useMemo(
    () => Math.max(1, ...points.map((p) => p.total)),
    [points],
  );

  // Time domain. With a single point d3 collapses extent to [d, d]; pad
  // by ±1h so the line/dots aren't pinned to one edge.
  const [t0, t1] = useMemo(() => {
    const ext = d3.extent(points, (p) => p.date) as [Date, Date];
    if (ext[0].getTime() === ext[1].getTime()) {
      return [new Date(ext[0].getTime() - 3600_000), new Date(ext[1].getTime() + 3600_000)];
    }
    return ext;
  }, [points]);

  const xScale = useMemo(
    () => d3.scaleTime().domain([t0, t1]).range([0, PLOT_W]),
    [t0, t1],
  );
  const yScale = useMemo(
    () => d3.scaleLinear().domain([0, maxTotal]).nice().range([PLOT_H, 0]),
    [maxTotal],
  );

  const stacked = useMemo(() => {
    const s = d3
      .stack<HistoryPoint, StackKey>()
      .keys(STACK_KEYS as unknown as StackKey[])
      .order(d3.stackOrderNone)
      .offset(d3.stackOffsetNone);
    return s(points);
  }, [points]);

  const area = useMemo(
    () =>
      d3
        .area<d3.SeriesPoint<HistoryPoint>>()
        .x((d) => xScale(d.data.date))
        .y0((d) => yScale(d[0]))
        .y1((d) => yScale(d[1]))
        .curve(d3.curveMonotoneX),
    [xScale, yScale],
  );

  const yTicks = yScale.ticks(6);
  const xTicks = xScale.ticks(Math.min(points.length, 8));
  const totalReference =
    new Set(points.map((p) => p.total)).size === 1 && points[0]!.total > 0
      ? points[0]!.total
      : null;

  return (
    <div className="relative">
      <svg
        viewBox={`0 0 ${VIEW_W} ${VIEW_H}`}
        preserveAspectRatio="xMidYMid meet"
        className="block w-full"
        role="img"
        aria-label="Stacked area chart of test pass/fail history"
      >
        <g transform={`translate(${MARGIN.left},${MARGIN.top})`}>
          {/* Grid lines */}
          {yTicks.map((t) => (
            <line
              key={`grid-${t}`}
              x1={0}
              x2={PLOT_W}
              y1={yScale(t)}
              y2={yScale(t)}
              stroke="currentColor"
              strokeDasharray="3 3"
              className="text-border"
              opacity={0.5}
            />
          ))}

          {/* Stacked areas */}
          {stacked.map((layer) => {
            const key = layer.key as StackKey;
            return (
              <path
                key={key}
                d={area(layer) ?? undefined}
                fill={COLORS[key]}
                opacity={hover && hover.layer !== key ? 0.35 : 0.8}
              />
            );
          })}

          {/* Total-tests reference line (only when every run has the same total) */}
          {totalReference != null && (
            <g>
              <line
                x1={0}
                x2={PLOT_W}
                y1={yScale(totalReference)}
                y2={yScale(totalReference)}
                stroke="currentColor"
                strokeDasharray="5 5"
                strokeWidth={1.5}
                className="text-primary"
                opacity={0.7}
              />
              <text
                x={PLOT_W - 6}
                y={yScale(totalReference) - 6}
                textAnchor="end"
                className="fill-primary text-[11px]"
              >
                Total: {totalReference} tests
              </text>
            </g>
          )}

          {/* Data points — one per stack layer per run */}
          {stacked.map((layer) =>
            layer.map((d, i) => {
              const key = layer.key as StackKey;
              const point = points[i]!;
              const cx = xScale(d.data.date);
              const cy = yScale(d[1]);
              const isHover =
                hover && hover.point.runId === point.runId && hover.layer === key;
              return (
                <circle
                  key={`${key}-${point.runId}-${i}`}
                  cx={cx}
                  cy={cy}
                  r={isHover ? 6 : 4}
                  fill={COLORS[key]}
                  stroke="white"
                  strokeWidth={1.5}
                  className="cursor-pointer"
                  onMouseEnter={() => setHover({ point, layer: key, cx, cy })}
                  onMouseLeave={() => setHover(null)}
                />
              );
            }),
          )}

          {/* X axis */}
          <g transform={`translate(0,${PLOT_H})`}>
            <line x1={0} x2={PLOT_W} stroke="currentColor" className="text-border" />
            {xTicks.map((t, i) => {
              const x = xScale(t);
              return (
                <g key={i} transform={`translate(${x},0)`}>
                  <line y2={5} stroke="currentColor" className="text-border" />
                  <text
                    y={20}
                    textAnchor="end"
                    transform="rotate(-35)"
                    className="fill-muted text-[10px]"
                  >
                    {d3.timeFormat('%m/%d %H:%M')(t)}
                  </text>
                </g>
              );
            })}
          </g>

          {/* Y axis */}
          <g>
            <line y1={0} y2={PLOT_H} stroke="currentColor" className="text-border" />
            {yTicks.map((t) => (
              <g key={t} transform={`translate(0,${yScale(t)})`}>
                <line x2={-5} stroke="currentColor" className="text-border" />
                <text x={-8} dy="0.32em" textAnchor="end" className="fill-muted text-[10px]">
                  {t}
                </text>
              </g>
            ))}
            <text
              transform={`translate(${-MARGIN.left + 14},${PLOT_H / 2}) rotate(-90)`}
              textAnchor="middle"
              className="fill-muted text-[11px]"
            >
              Number of tests
            </text>
          </g>
        </g>
      </svg>

      {hover && <Tooltip hover={hover} />}
    </div>
  );
}

function Tooltip({
  hover,
}: {
  hover: { point: HistoryPoint; layer: StackKey; cx: number; cy: number };
}) {
  const { point } = hover;
  const passRate = point.total > 0 ? (point.passed / point.total) * 100 : 0;
  // The hover x/y are in SVG-viewport units. Convert to percentage so the
  // tooltip tracks the tile correctly under responsive scaling.
  const xPct = ((hover.cx + MARGIN.left) / VIEW_W) * 100;
  const yPct = ((hover.cy + MARGIN.top) / VIEW_H) * 100;
  const showLeft = xPct > 70;
  const style: React.CSSProperties = showLeft
    ? { right: `calc(${100 - xPct}% + 12px)`, top: `calc(${yPct}% - 24px)` }
    : { left: `calc(${xPct}% + 12px)`, top: `calc(${yPct}% - 24px)` };

  return (
    <div
      className="pointer-events-none absolute z-10 min-w-[180px] rounded-lg border border-border bg-surface/95 p-3 text-xs shadow-lg backdrop-blur"
      style={style}
    >
      <div className="mb-1 font-semibold text-foreground">
        Run #{point.index + 1}
      </div>
      <div className="text-muted">
        {point.date.toLocaleString()}
      </div>
      <div className="mt-2 grid grid-cols-2 gap-x-3 gap-y-0.5">
        <span className="text-status-passed-fg">Passed</span>
        <span className="text-right tabular-nums">{point.passed.toLocaleString()}</span>
        <span className="text-status-failed-fg">Failed</span>
        <span className="text-right tabular-nums">{point.failed.toLocaleString()}</span>
        <span className="text-muted">Skipped</span>
        <span className="text-right tabular-nums">{point.skipped.toLocaleString()}</span>
        <span className="text-foreground">Total</span>
        <span className="text-right font-medium tabular-nums">
          {point.total.toLocaleString()}
        </span>
      </div>
      <div className="mt-2 border-t border-border pt-1.5 text-[11px]">
        Pass rate:{' '}
        <span
          className={
            point.failed === 0 && point.total > 0
              ? 'text-status-passed-fg'
              : point.failed > 0
                ? 'text-status-failed-fg'
                : 'text-muted'
          }
        >
          {passRate.toFixed(1)}%
        </span>
      </div>
    </div>
  );
}

function Legend() {
  return (
    <div className="flex items-center gap-3 text-[11px] text-muted">
      <Swatch color={COLORS.passed} label="Passed" />
      <Swatch color={COLORS.failed} label="Failed" />
      <Swatch color={COLORS.skipped} label="Skipped" />
    </div>
  );
}
function Swatch({ color, label }: { color: string; label: string }) {
  return (
    <span className="inline-flex items-center gap-1">
      <span
        aria-hidden="true"
        className="inline-block h-2 w-3 rounded-sm"
        style={{ background: color }}
      />
      {label}
    </span>
  );
}
