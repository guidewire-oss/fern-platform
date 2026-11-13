import { describe, it, expect } from 'vitest';
import type { SpecRunHistory } from '@/lib/types';
import { CHART, columnPath, durationLayout, failedRuns } from './specHistory';

const run = (id: number) => ({
  id,
  runId: `r${id}`,
  branch: 'main',
  status: 'passed',
  startTime: `2026-05-2${id}T10:00:00Z`,
  durationMs: 0,
});

function history(results: SpecRunHistory['results']): SpecRunHistory {
  return { runs: results.map((_, i) => run(i + 1)), suiteName: 'S', specName: 'x', results };
}

describe('durationLayout', () => {
  it('scales bars from zero to the longest run and marks the viewed run', () => {
    const l = durationLayout(
      history([
        { status: 'passed', durationMs: 100 },
        null,
        { status: 'failed', durationMs: 400 },
        { status: 'passed', durationMs: 200 },
      ]),
    );
    expect(l.maxMs).toBe(400);
    expect(l.bars.map((b) => b.height)).toEqual([16, 0, 64, 32]);
    expect(l.bars.map((b) => b.viewed)).toEqual([false, false, false, true]);
    expect(l.bars[1]!.cell).toBeNull();
    expect(l.bars[2]!.x).toBe(2 * (CHART.barWidth + CHART.gap));
  });

  it('takes the median over the runs the spec was in', () => {
    const odd = durationLayout(history([{ status: 'passed', durationMs: 300 }, null, { status: 'passed', durationMs: 100 }, { status: 'passed', durationMs: 200 }]));
    expect(odd.medianMs).toBe(200);
    expect(odd.medianY).toBe(CHART.plotHeight - (200 / 300) * CHART.plotHeight);
    const even = durationLayout(history([{ status: 'passed', durationMs: 100 }, { status: 'passed', durationMs: 300 }]));
    expect(even.medianMs).toBe(200);
  });

  it('handles a spec absent from every run, and all-zero durations', () => {
    const none = durationLayout(history([null, null]));
    expect(none.medianMs).toBeNull();
    expect(none.medianY).toBeNull();
    const zero = durationLayout(history([{ status: 'passed', durationMs: 0 }]));
    expect(zero.bars[0]!.height).toBe(0);
  });
});

describe('columnPath', () => {
  it('rounds the top and keeps the baseline square', () => {
    const d = columnPath(0, 64, 14, 30, 4);
    expect(d.startsWith('M0,64 V38 Q0,34 4,34')).toBe(true);
    expect(d.endsWith('V64 Z')).toBe(true);
  });

  it('draws nothing for an empty bar and shrinks the radius for a short one', () => {
    expect(columnPath(0, 64, 14, 0, 4)).toBe('');
    expect(columnPath(0, 64, 14, 2, 4)).toContain('Q0,62 2,62');
  });
});

describe('failedRuns', () => {
  it('lists failed runs newest first and flags the viewed run', () => {
    const f = failedRuns(
      history([
        { status: 'failed', durationMs: 1, errorMessage: 'old' },
        { status: 'passed', durationMs: 1 },
        null,
        { status: 'failed', durationMs: 1, errorMessage: 'now' },
      ]),
    );
    expect(f.map((x) => x.cell.errorMessage)).toEqual(['now', 'old']);
    expect(f.map((x) => x.viewed)).toEqual([true, false]);
  });
});
