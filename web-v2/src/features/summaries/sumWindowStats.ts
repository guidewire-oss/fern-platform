// Pure helpers extracted from TestSummaries so they can be unit-tested
// without mounting the full component (and re-used by future widgets
// that want the same Total / Passed / Failed roll-up).

export interface TrendBucket {
  totalRuns: number;
  totalTests: number;
  passedTests: number;
  failedTests: number;
  skippedTests: number;
}

export interface WindowStats {
  totalRuns: number;
  totalTests: number;
  passedTests: number;
  failedTests: number;
  skippedTests: number;
  passRate: number; // 0..1; 0 when totalTests is 0
}

export function sumWindowStats(rows: readonly TrendBucket[]): WindowStats {
  let totalRuns = 0;
  let totalTests = 0;
  let passedTests = 0;
  let failedTests = 0;
  let skippedTests = 0;
  for (const r of rows) {
    totalRuns    += r.totalRuns;
    totalTests   += r.totalTests;
    passedTests  += r.passedTests;
    failedTests  += r.failedTests;
    skippedTests += r.skippedTests;
  }
  return {
    totalRuns,
    totalTests,
    passedTests,
    failedTests,
    skippedTests,
    passRate: totalTests > 0 ? passedTests / totalTests : 0,
  };
}
