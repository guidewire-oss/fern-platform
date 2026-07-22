import { describe, it, expect } from 'vitest';
import {
  coveragePercent,
  treeSummary,
  defaultRelease,
  storyResult,
  donutBreakdown,
  type RequirementCoverageTree,
  type StoryCoverageNode,
  type JiraRelease,
} from './coverage';

const issue = (key: string) => ({ key, summary: key, statusName: 'Done', issueType: 'Story' });
const story = (key: string, covered: boolean): StoryCoverageNode => ({
  issue: issue(key),
  covered,
  testRunCoverage: null,
  subTasks: [],
});

describe('coveragePercent', () => {
  it('guards divide-by-zero', () => {
    expect(coveragePercent(0, 0)).toBe(0);
    expect(coveragePercent(5, 0)).toBe(0);
  });
  it('rounds to a whole percent', () => {
    expect(coveragePercent(1, 2)).toBe(50);
    expect(coveragePercent(3, 3)).toBe(100);
    expect(coveragePercent(1, 3)).toBe(33);
    expect(coveragePercent(2, 3)).toBe(67);
  });
});

describe('treeSummary', () => {
  it('sums epic counts and unassigned covered flags', () => {
    const tree: RequirementCoverageTree = {
      fixVersion: { id: '1', name: 'v1', released: false },
      epics: [
        { issue: issue('E1'), stories: [], coveredCount: 3, totalCount: 5 },
        { issue: issue('E2'), stories: [], coveredCount: 2, totalCount: 2 },
      ],
      unassigned: [story('U1', true), story('U2', false), story('U3', false)],
    };
    // epics: 5/7 covered; unassigned: 1/3 covered → 6/10.
    expect(treeSummary(tree)).toEqual({ coveredCount: 6, totalCount: 10, percent: 60 });
  });

  it('is 0% for an empty tree', () => {
    const tree: RequirementCoverageTree = {
      fixVersion: { id: '1', name: 'v1', released: false },
      epics: [],
      unassigned: [],
    };
    expect(treeSummary(tree)).toEqual({ coveredCount: 0, totalCount: 0, percent: 0 });
  });
});

describe('storyResult', () => {
  it('is uncovered when the story has no tests', () => {
    expect(storyResult(story('U', false))).toBe('uncovered');
  });
  it('is failing when a covered story has ≥1 failure', () => {
    const s: StoryCoverageNode = {
      issue: issue('F'),
      covered: true,
      testRunCoverage: { total: 5, passed: 4, failed: 1, skipped: 0, lastRunAt: null },
      subTasks: [],
    };
    expect(storyResult(s)).toBe('failing');
  });
  it('is passing when a covered story has no failures', () => {
    const s: StoryCoverageNode = {
      issue: issue('P'),
      covered: true,
      testRunCoverage: { total: 5, passed: 5, failed: 0, skipped: 0, lastRunAt: null },
      subTasks: [],
    };
    expect(storyResult(s)).toBe('passing');
  });
});

describe('donutBreakdown', () => {
  it('tallies passing / failing / uncovered across epics and unassigned', () => {
    const covered = (key: string, failed: number): StoryCoverageNode => ({
      issue: issue(key),
      covered: true,
      testRunCoverage: { total: 3, passed: 3 - failed, failed, skipped: 0, lastRunAt: null },
      subTasks: [],
    });
    const tree: RequirementCoverageTree = {
      fixVersion: { id: '1', name: 'v1', released: false },
      epics: [
        { issue: issue('E1'), coveredCount: 2, totalCount: 3, stories: [covered('a', 0), covered('b', 2), story('c', false)] },
      ],
      unassigned: [covered('u1', 0), story('u2', false)],
    };
    // passing: a, u1 = 2; failing: b = 1; uncovered: c, u2 = 2; total 5.
    expect(donutBreakdown(tree)).toEqual({
      passing: 2,
      failing: 1,
      uncovered: 2,
      total: 5,
      covered: 3,
      percent: 60,
    });
  });
});

describe('defaultRelease', () => {
  const rel = (name: string, released: boolean): JiraRelease => ({ id: name, name, released });
  it('prefers the first unreleased version', () => {
    expect(defaultRelease([rel('1.0', true), rel('1.1', false), rel('1.2', false)])?.name).toBe('1.1');
  });
  it('falls back to the first when all are released', () => {
    expect(defaultRelease([rel('1.0', true), rel('0.9', true)])?.name).toBe('1.0');
  });
  it('returns undefined for no releases', () => {
    expect(defaultRelease([])).toBeUndefined();
  });
});
