import { describe, it, expect } from 'vitest';
import { historyLookup, historySummary } from './runHistory';
import type { RunHistory } from '@/lib/types';

const history: RunHistory = {
  runs: [{ id: 1, runId: 'a', branch: 'main', status: 'passed', startTime: '2026-05-23T10:00:00Z', durationMs: 1 }],
  suites: [{ suiteName: 'S', results: [{ status: 'passed', durationMs: 1 }] }],
  specs: [
    { suiteName: 'a', specName: 'b c', results: [{ status: 'passed', durationMs: 1 }] },
    { suiteName: 'a b', specName: 'c', results: [{ status: 'failed', durationMs: 1 }] },
  ],
};

describe('historyLookup', () => {
  it('finds suites and specs by name', () => {
    const l = historyLookup(history);
    expect(l.suite('S')?.[0]?.status).toBe('passed');
    expect(l.spec('a', 'b c')?.[0]?.status).toBe('passed');
    expect(l.spec('a b', 'c')?.[0]?.status).toBe('failed');
    expect(l.spec('S', 'missing')).toBeUndefined();
  });

  it('finds nothing for a missing or malformed response', () => {
    for (const bad of [undefined, null, { ...history, runs: [] }, { testRun: {} } as unknown as RunHistory]) {
      const l = historyLookup(bad);
      expect(l.suite('S')).toBeUndefined();
      expect(l.spec('a', 'b c')).toBeUndefined();
    }
  });
});

describe('historySummary', () => {
  it('counts statuses and absent runs', () => {
    expect(
      historySummary([
        { status: 'passed', durationMs: 1 },
        null,
        { status: 'passed', durationMs: 1 },
        { status: 'failed', durationMs: 1 },
      ]),
    ).toBe('Last 4 runs: 2 passed, 1 not run, 1 failed');
  });

  it('uses the singular for one run', () => {
    expect(historySummary([{ status: 'passed', durationMs: 1 }])).toBe('Last 1 run: 1 passed');
  });
});
