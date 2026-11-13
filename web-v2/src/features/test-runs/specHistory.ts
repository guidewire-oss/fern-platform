import type { HistoryRun, SpecHistoryCell, SpecRunHistory } from '@/lib/types';

// Geometry of the duration chart, in SVG user units. Columns stay at
// most 24 wide with a 2-unit gap, so the chart reads as separate runs.
export const CHART = {
  barWidth: 14,
  gap: 2,
  plotHeight: 64,
  radius: 4,
} as const;

export interface DurationBar {
  index: number;
  run: HistoryRun | undefined;
  cell: SpecHistoryCell | null;
  x: number;
  // Bar height in the plot (0 for an absent run or a zero duration).
  height: number;
  viewed: boolean;
}

export interface DurationLayout {
  bars: DurationBar[];
  width: number;
  maxMs: number;
  // Median of the runs the spec was in; null when it was in none.
  medianMs: number | null;
  // y of the median line from the plot top, or null.
  medianY: number | null;
}

function median(values: number[]): number | null {
  if (values.length === 0) return null;
  const s = [...values].sort((a, b) => a - b);
  const mid = Math.floor(s.length / 2);
  return s.length % 2 === 1 ? s[mid]! : (s[mid - 1]! + s[mid]!) / 2;
}

// durationLayout places one column per run of the window, in the same
// order as the history strip (oldest left, viewed run last). Heights
// scale linearly from a zero baseline to the longest duration, so bars
// compare honestly.
export function durationLayout(history: SpecRunHistory): DurationLayout {
  const { results, runs } = history;
  const present = results.filter((c): c is SpecHistoryCell => c !== null).map((c) => c.durationMs);
  const maxMs = Math.max(0, ...present);
  const med = median(present);
  const scale = (ms: number) => (maxMs > 0 ? (ms / maxMs) * CHART.plotHeight : 0);
  const step = CHART.barWidth + CHART.gap;
  return {
    bars: results.map((cell, i) => ({
      index: i,
      run: runs[i],
      cell,
      x: i * step,
      height: cell ? scale(cell.durationMs) : 0,
      viewed: i === results.length - 1,
    })),
    width: Math.max(0, results.length * step - CHART.gap),
    maxMs,
    medianMs: med,
    medianY: med === null ? null : CHART.plotHeight - scale(med),
  };
}

// columnPath draws a column standing on the baseline: rounded at the
// data end (top), square at the baseline. Short columns shrink the
// radius so the shape never inverts.
export function columnPath(x: number, baseline: number, width: number, height: number, radius: number): string {
  if (height <= 0) return '';
  const r = Math.min(radius, width / 2, height);
  const top = baseline - height;
  return [
    `M${x},${baseline}`,
    `V${top + r}`,
    `Q${x},${top} ${x + r},${top}`,
    `H${x + width - r}`,
    `Q${x + width},${top} ${x + width},${top + r}`,
    `V${baseline}`,
    'Z',
  ].join(' ');
}

export interface FailedRun {
  run: HistoryRun;
  cell: SpecHistoryCell;
  viewed: boolean;
}

// failedRuns lists the window's failed runs, newest first, for the
// panel's failures section.
export function failedRuns(history: SpecRunHistory): FailedRun[] {
  const out: FailedRun[] = [];
  const last = history.results.length - 1;
  history.results.forEach((cell, i) => {
    const run = history.runs[i];
    if (cell && cell.status === 'failed' && run) out.push({ run, cell, viewed: i === last });
  });
  return out.reverse();
}
