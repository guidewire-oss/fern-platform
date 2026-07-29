// Issue #216: the dashboard's "Recent test runs" rows identified each
// run by raw project_id. They now lead with the project's display name
// and keep the id on the detail line, matching the rest of the app.

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, cleanup, within } from '@testing-library/react';
import type { TestRunConnection, TestRunNode } from '@/lib/types';

vi.mock('@tanstack/react-router', () => ({
  Link: ({ to, children, className }: { to: string; children: React.ReactNode; className?: string }) => (
    <a href={to} className={className} data-to={to}>{children}</a>
  ),
}));

const mockQuery = vi.fn();
vi.mock('@tanstack/react-query', () => ({
  useQuery: (opts: { queryKey: unknown[] }) => mockQuery(opts),
}));

vi.mock('../projects/hooks', () => ({
  useProjects: () => ({
    data: { totalCount: 0, edges: [], projects: [] },
    isLoading: false,
    error: null,
  }),
}));

import Dashboard from './Dashboard';

function node(over: Partial<TestRunNode>): TestRunNode {
  return {
    id: 1,
    run_id: 'run-1',
    project_id: 'java-1-001',
    branch: 'main',
    git_branch: 'main',
    git_commit: 'abc',
    status: 'passed',
    start_time: '2026-01-01T00:00:00Z',
    end_time: '2026-01-01T00:01:00Z',
    total_tests: 1,
    passed_tests: 1,
    failed_tests: 0,
    skipped_tests: 0,
    environment: 'ci',
    ...over,
  };
}

function conn(nodes: TestRunNode[]): TestRunConnection {
  return {
    edges: nodes.map((n) => ({ cursor: 'c', node: n })),
    pageInfo: { hasNextPage: false, endCursor: '' },
    totalCount: nodes.length,
    totalCountIsEstimate: false,
    facets: { byStatus: [], byBranch: [], byTag: [], byProject: [] },
  };
}

function setup(n: TestRunNode) {
  mockQuery.mockImplementation((opts?: { queryKey?: unknown[] }) => ({
    data: opts?.queryKey?.[1] === 'all' ? conn([n]) : conn([]),
    isLoading: false,
    error: null,
  }));
  return render(<Dashboard />);
}

// The recent-runs rows link to /test-runs/$runId; scope queries to those
// so the stat tiles above don't match.
function recentRow(): HTMLElement {
  const row = document.querySelector('a[data-to="/test-runs/$runId"]');
  if (!row) throw new Error('no recent-run row rendered');
  return row as HTMLElement;
}

describe('Dashboard recent test runs', () => {
  beforeEach(() => mockQuery.mockReset());
  afterEach(cleanup);

  it('leads with the project display name', () => {
    setup(node({ project_name: 'Java Service 1' }));
    expect(within(recentRow()).getByText('Java Service 1')).toBeTruthy();
  });

  it('keeps the project id visible on the row', () => {
    setup(node({ project_name: 'Java Service 1' }));
    expect(within(recentRow()).getByText(/java-1-001/)).toBeTruthy();
  });

  it('falls back to the id alone when no name is available', () => {
    setup(node({}));
    const row = recentRow();
    expect(within(row).getByText('java-1-001')).toBeTruthy();
    expect(within(row).queryAllByText(/java-1-001/)).toHaveLength(1);
  });
});
