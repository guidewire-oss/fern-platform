import { describe, it, expect } from 'vitest';
import { sumWindowStats, type TrendBucket } from './sumWindowStats';

const row = (over: Partial<TrendBucket> = {}): TrendBucket => ({
  totalRuns: 0,
  totalTests: 0,
  passedTests: 0,
  failedTests: 0,
  skippedTests: 0,
  ...over,
});

describe('sumWindowStats', () => {
  it('returns zeroed stats and 0 passRate for empty input', () => {
    expect(sumWindowStats([])).toEqual({
      totalRuns: 0,
      totalTests: 0,
      passedTests: 0,
      failedTests: 0,
      skippedTests: 0,
      passRate: 0,
    });
  });

  it('sums each counter across rows', () => {
    const got = sumWindowStats([
      row({ totalRuns: 2, totalTests: 10, passedTests: 8, failedTests: 1, skippedTests: 1 }),
      row({ totalRuns: 3, totalTests: 20, passedTests: 18, failedTests: 2 }),
      row({}), // zero day stays a no-op
    ]);
    expect(got.totalRuns).toBe(5);
    expect(got.totalTests).toBe(30);
    expect(got.passedTests).toBe(26);
    expect(got.failedTests).toBe(3);
    expect(got.skippedTests).toBe(1);
  });

  it('computes passRate as passed / total', () => {
    const got = sumWindowStats([
      row({ totalTests: 100, passedTests: 90 }),
      row({ totalTests: 100, passedTests: 70 }),
    ]);
    expect(got.passRate).toBeCloseTo(0.8, 5);
  });

  it('passRate is 0 (not NaN) when no tests ran', () => {
    const got = sumWindowStats([row({ totalRuns: 1 })]);
    expect(got.passRate).toBe(0);
    expect(Number.isNaN(got.passRate)).toBe(false);
  });
});
