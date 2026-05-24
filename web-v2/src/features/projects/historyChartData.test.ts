import { describe, it, expect } from 'vitest';
import { toHistoryPoints } from './historyChartData';
import type { TestRunNode } from '@/lib/types';

const run = (over: Partial<TestRunNode> = {}): TestRunNode => ({
  id: 1,
  run_id: 'r',
  project_id: 'p',
  branch: 'main',
  git_branch: 'main',
  git_commit: 'abc',
  status: 'passed',
  start_time: '2026-05-22T00:00:00Z',
  end_time: null,
  total_tests: 0,
  passed_tests: 0,
  failed_tests: 0,
  skipped_tests: 0,
  environment: 'ci',
  ...over,
});

describe('toHistoryPoints', () => {
  it('sorts oldest → newest and caps to limit', () => {
    const runs = [
      run({ run_id: 'c', start_time: '2026-05-22T03:00:00Z' }),
      run({ run_id: 'a', start_time: '2026-05-22T01:00:00Z' }),
      run({ run_id: 'b', start_time: '2026-05-22T02:00:00Z' }),
    ];
    const pts = toHistoryPoints(runs, 2);
    expect(pts.map((p) => p.runId)).toEqual(['b', 'c']);
    expect(pts.map((p) => p.index)).toEqual([0, 1]);
  });

  it('reconciles skipped when sum drifts from total', () => {
    const [p] = toHistoryPoints([
      run({ total_tests: 100, passed_tests: 80, failed_tests: 5, skipped_tests: 999 }),
    ]);
    expect(p!.skipped).toBe(15);
    expect(p!.passed + p!.failed + p!.skipped).toBe(p!.total);
  });

  it('leaves skipped alone when totals add up', () => {
    const [p] = toHistoryPoints([
      run({ total_tests: 30, passed_tests: 20, failed_tests: 5, skipped_tests: 5 }),
    ]);
    expect(p!.skipped).toBe(5);
  });

  it('handles total=0 without dividing or going negative', () => {
    const [p] = toHistoryPoints([run()]);
    expect(p!.total).toBe(0);
    expect(p!.skipped).toBe(0);
  });
});
