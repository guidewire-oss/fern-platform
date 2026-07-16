// Client-side model + pure helpers for the JIRA requirement-coverage view.
// Types mirror schema.graphql (RequirementCoverageTree and friends).

export interface JiraRelease {
  id: string;
  name: string;
  released: boolean;
  releaseDate?: string | null;
}

export interface JiraIssueSummary {
  key: string;
  summary: string;
  statusName: string;
  issueType: string;
}

export interface TestRunCoverage {
  total: number;
  passed: number;
  failed: number;
  skipped: number;
  lastRunAt?: string | null;
}

export interface StoryCoverageNode {
  issue: JiraIssueSummary;
  covered: boolean;
  testRunCoverage?: TestRunCoverage | null;
  subTasks: StoryCoverageNode[];
}

export interface EpicCoverageNode {
  issue: JiraIssueSummary;
  stories: StoryCoverageNode[];
  coveredCount: number;
  totalCount: number;
}

export interface RequirementCoverageTree {
  fixVersion: JiraRelease;
  epics: EpicCoverageNode[];
  unassigned: StoryCoverageNode[];
}

// coveragePercent is a whole-number percentage, guarding divide-by-zero.
export function coveragePercent(covered: number, total: number): number {
  if (total <= 0) return 0;
  return Math.round((covered / total) * 100);
}

export interface CoverageSummary {
  coveredCount: number;
  totalCount: number;
  percent: number;
}

// treeSummary rolls the whole tree up to a single covered/total/percent,
// counting epic stories via their pre-computed counts and unassigned
// stories by their own covered flag.
export function treeSummary(tree: RequirementCoverageTree): CoverageSummary {
  let coveredCount = 0;
  let totalCount = 0;
  for (const epic of tree.epics) {
    coveredCount += epic.coveredCount;
    totalCount += epic.totalCount;
  }
  for (const story of tree.unassigned) {
    totalCount += 1;
    if (story.covered) coveredCount += 1;
  }
  return { coveredCount, totalCount, percent: coveragePercent(coveredCount, totalCount) };
}

export type StoryResult = 'passing' | 'failing' | 'uncovered';

// storyResult classifies a story for the coverage colours/legend:
// uncovered (no tests), failing (covered with ≥1 failure), or passing.
export function storyResult(s: StoryCoverageNode): StoryResult {
  if (!s.covered) return 'uncovered';
  if ((s.testRunCoverage?.failed ?? 0) > 0) return 'failing';
  return 'passing';
}

export interface DonutBreakdown {
  passing: number;
  failing: number;
  uncovered: number;
  total: number;
  covered: number;
  percent: number;
}

// donutBreakdown tallies every top-level story across all epics and the
// unassigned bucket into passing / failing / uncovered — the three donut
// segments and legend counts.
export function donutBreakdown(tree: RequirementCoverageTree): DonutBreakdown {
  let passing = 0;
  let failing = 0;
  let uncovered = 0;
  const tally = (s: StoryCoverageNode) => {
    const r = storyResult(s);
    if (r === 'passing') passing += 1;
    else if (r === 'failing') failing += 1;
    else uncovered += 1;
  };
  for (const epic of tree.epics) for (const s of epic.stories) tally(s);
  for (const s of tree.unassigned) tally(s);
  const total = passing + failing + uncovered;
  const covered = passing + failing;
  return { passing, failing, uncovered, total, covered, percent: coveragePercent(covered, total) };
}

// Chooses the release to select by default: prefer the most recent
// unreleased version (active development), else the first in the list.
export function defaultRelease(releases: JiraRelease[]): JiraRelease | undefined {
  if (releases.length === 0) return undefined;
  return releases.find((r) => !r.released) ?? releases[0];
}
