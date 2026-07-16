import { describe, it, expect, afterEach } from 'vitest';
import { render, screen, cleanup } from '@testing-library/react';
import { CoverageTree } from './CoverageTree';
import type { RequirementCoverageTree, StoryCoverageNode } from './coverage';

const issue = (key: string, summary: string) => ({
  key,
  summary,
  statusName: 'Done',
  issueType: 'Story',
});

const story = (key: string, summary: string, covered: boolean, cov?: {
  total: number;
  passed: number;
  failed: number;
  skipped: number;
}): StoryCoverageNode => ({
  issue: issue(key, summary),
  covered,
  testRunCoverage: cov ? { ...cov, lastRunAt: null } : null,
  subTasks: [],
});

afterEach(() => cleanup());

const tree: RequirementCoverageTree = {
  fixVersion: { id: '1', name: 'Release 1.0', released: false },
  epics: [
    {
      issue: issue('E-1', 'Checkout epic'),
      coveredCount: 1,
      totalCount: 2,
      stories: [
        story('S-1', 'Covered story', true, { total: 5, passed: 5, failed: 0, skipped: 0 }),
        story('S-2', 'Uncovered story', false),
      ],
    },
  ],
  unassigned: [story('U-1', 'Loose story', false)],
};

describe('CoverageTree', () => {
  it('renders epics, stories, and the unassigned section', () => {
    render(<CoverageTree tree={tree} openEpics={new Set(["E-1"])} onToggleEpic={() => {}} />);
    expect(screen.getByText('Checkout epic')).toBeTruthy();
    expect(screen.getByText('Covered story')).toBeTruthy();
    expect(screen.getByText('Uncovered story')).toBeTruthy();
    expect(screen.getByText(/Unassigned stories/i)).toBeTruthy();
    // Epic covered/total badge.
    expect(screen.getByText('1/2')).toBeTruthy();
  });

  it('shows a coverage pill for covered stories and "uncovered" otherwise', () => {
    render(<CoverageTree tree={tree} openEpics={new Set(["E-1"])} onToggleEpic={() => {}} />);
    // Covered story shows the passed count from its coverage pill.
    expect(screen.getByText('5✓')).toBeTruthy();
    // At least the two uncovered rows (S-2 and U-1) render the label.
    expect(screen.getAllByText('uncovered').length).toBeGreaterThanOrEqual(2);
  });

  it('renders an empty message when there are no requirements', () => {
    render(
      <CoverageTree tree={{ fixVersion: tree.fixVersion, epics: [], unassigned: [] }} openEpics={new Set()} onToggleEpic={() => {}} />,
    );
    expect(screen.getByText(/No requirements found/i)).toBeTruthy();
  });
});
