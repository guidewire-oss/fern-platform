import type { Project } from '@/lib/types';
import type { SortKey, SortDir } from './ProjectsFilterBar';

// sortProjects orders projects for the Projects grid and Summaries.
//
// `runsByProject` optionally overrides the run count per project id — the
// Summaries page passes the *selected window's* run totals (what the
// cards show) so sorting by "Runs" reorders visibly and matches the
// displayed numbers; the Projects grid omits it and falls back to the
// all-time `stats.totalTestRuns`.
//
// Ties always break by name ascending, so equal values (e.g. many
// projects with the same run count) still land in a stable, sensible
// order instead of looking like the sort "did nothing".
//
// Projects with no runs have no meaningful pass rate or last-activity —
// those metrics are N/A, not zero. Treating them as 0 made them sort to
// the very top in ascending pass-rate order, which is misleading. So for
// the `rate` and `last` keys they're pushed to the end regardless of
// direction (they only carry a real value under the `runs` key, where 0
// legitimately IS the smallest).
export function sortProjects(
  projects: readonly Project[],
  sortKey: SortKey,
  sortDir: SortDir,
  runsByProject?: Record<string, number>,
): Project[] {
  const dir = sortDir === 'asc' ? 1 : -1;
  const runs = (p: Project) => runsByProject?.[p.projectId] ?? p.stats?.totalTestRuns ?? 0;
  const lastMs = (p: Project) => (p.stats?.lastRunTime ? new Date(p.stats.lastRunTime).getTime() : 0);
  const hasRuns = (p: Project) => runs(p) > 0;

  return [...projects].sort((a, b) => {
    // For metrics that are undefined without runs, sink the no-run
    // projects below every project that has runs, in both directions.
    if (sortKey === 'rate' || sortKey === 'last') {
      if (hasRuns(a) !== hasRuns(b)) return hasRuns(a) ? -1 : 1;
    }

    let cmp = 0;
    switch (sortKey) {
      case 'name':
        cmp = a.name.localeCompare(b.name);
        break;
      case 'runs':
        cmp = runs(a) - runs(b);
        break;
      case 'rate':
        cmp = (a.stats?.successRate ?? 0) - (b.stats?.successRate ?? 0);
        break;
      case 'last':
        cmp = lastMs(a) - lastMs(b);
        break;
    }
    if (cmp !== 0) return cmp * dir;
    // Stable, direction-independent tiebreaker.
    return a.name.localeCompare(b.name);
  });
}
