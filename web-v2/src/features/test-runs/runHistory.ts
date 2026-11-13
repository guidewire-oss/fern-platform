import type { HistoryCell, RunHistory } from '@/lib/types';

export type HistoryResults = (HistoryCell | null)[];

export interface HistoryLookup {
  suite(suiteName: string): HistoryResults | undefined;
  spec(suiteName: string, specName: string): HistoryResults | undefined;
}

const EMPTY_LOOKUP: HistoryLookup = {
  suite: () => undefined,
  spec: () => undefined,
};

// specKey joins the two parts with a character that can't appear in
// either name, so "a"+"b c" and "a b"+"c" stay distinct.
function specKey(suiteName: string, specName: string): string {
  return `${suiteName}\u0000${specName}`;
}

// historyLookup indexes a RunHistory response by suite and spec name.
// A spec is identified by (suite name, spec name), the same rule the
// server uses. Anything that isn't a well-formed response (still
// loading, an error, an older backend) yields a lookup that finds
// nothing, so the tables render without strips rather than failing.
export function historyLookup(history: RunHistory | null | undefined): HistoryLookup {
  if (!history || !Array.isArray(history.runs) || history.runs.length === 0) {
    return EMPTY_LOOKUP;
  }
  const suites = new Map<string, HistoryResults>();
  for (const s of history.suites ?? []) suites.set(s.suiteName, s.results);
  const specs = new Map<string, HistoryResults>();
  for (const s of history.specs ?? []) specs.set(specKey(s.suiteName, s.specName), s.results);
  return {
    suite: (suiteName) => suites.get(suiteName),
    spec: (suiteName, specName) => specs.get(specKey(suiteName, specName)),
  };
}

// historySummary is the strip's accessible name, e.g.
// "Last 20 runs: 18 passed, 2 failed".
export function historySummary(results: HistoryResults): string {
  const counts = new Map<string, number>();
  for (const c of results) {
    const key = c ? c.status : 'not run';
    counts.set(key, (counts.get(key) ?? 0) + 1);
  }
  const parts = [...counts].map(([status, n]) => `${n} ${status}`);
  const runs = results.length === 1 ? '1 run' : `${results.length} runs`;
  return `Last ${runs}: ${parts.join(', ')}`;
}
