// Data-shaping helpers for the project test-history chart.
//
// The server occasionally returns rows where passed+failed+skipped doesn't
// equal total_tests (an ingestion edge case observed in v1). The chart
// needs internally consistent rows so the stacked y-scale doesn't lie,
// so we reconcile skipped against total when the sum drifts.

import type { TestRunNode } from '@/lib/types';

export interface HistoryPoint {
  index: number;
  date: Date;
  runId: string;
  passed: number;
  failed: number;
  skipped: number;
  total: number;
}

export function toHistoryPoints(
  runs: readonly TestRunNode[],
  limit = 20,
): HistoryPoint[] {
  // Oldest → newest, capped at `limit`, with skipped reconciled.
  const sorted = [...runs].sort(
    (a, b) => new Date(a.start_time).getTime() - new Date(b.start_time).getTime(),
  );
  const recent = sorted.slice(-limit);
  return recent.map((r, i) => {
    const total = r.total_tests ?? 0;
    const passed = r.passed_tests ?? 0;
    const failed = r.failed_tests ?? 0;
    let skipped = r.skipped_tests ?? 0;
    if (total > 0 && passed + failed + skipped !== total) {
      skipped = Math.max(0, total - passed - failed);
    }
    return {
      index: i,
      date: new Date(r.start_time),
      runId: r.run_id,
      passed,
      failed,
      skipped,
      total,
    };
  });
}
