// Covers the Project column on /test-runs (issue #216): the table shows
// the project's display name *and* its id, falls back to the id alone
// when no name is available, and leaves the link target pointing at the
// id.

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, cleanup } from '@testing-library/react';
import type { TestRunConnection, TestRunNode } from '@/lib/types';

vi.mock('@tanstack/react-router', () => ({
  Link: ({
    to,
    params,
    children,
    className,
    title,
  }: {
    to: string;
    params?: Record<string, string>;
    children: React.ReactNode;
    className?: string;
    title?: string;
  }) => {
    const href = Object.entries(params ?? {}).reduce(
      (acc, [k, v]) => acc.replace(`$${k}`, v),
      to,
    );
    return (
      <a href={href} className={className} title={title} data-to={to}>
        {children}
      </a>
    );
  },
}));

const mockUseTestRuns = vi.fn();
vi.mock('./hooks', () => ({
  useTestRuns: (filter: unknown) => mockUseTestRuns(filter),
}));

// The sidebar and saved-views bar have their own tests; stub them out so
// this file only exercises the table.
vi.mock('./FilterSidebar', () => ({
  FilterSidebar: () => <div data-testid="filter-sidebar" />,
}));
vi.mock('../saved-views/integration', () => ({
  TEST_RUNS_PAGE: 'test-runs',
  useSavedViews: () => ({ data: { views: [] }, isLoading: false }),
  useCreateSavedView: () => ({ mutateAsync: vi.fn(), isPending: false, error: null }),
}));

import TestRunsList from './TestRunsList';

function node(over: Partial<TestRunNode>): TestRunNode {
  return {
    id: 1,
    run_id: 'run-1',
    project_id: 'proj-a',
    branch: 'main',
    git_branch: 'main',
    git_commit: 'abc123',
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

function connection(nodes: TestRunNode[]): TestRunConnection {
  return {
    edges: nodes.map((n) => ({ cursor: `c${n.id}`, node: n })),
    pageInfo: { hasNextPage: false, endCursor: '' },
    totalCount: nodes.length,
    totalCountIsEstimate: false,
    facets: { byStatus: [], byBranch: [], byTag: [], byProject: [] },
  };
}

function renderWith(nodes: TestRunNode[]) {
  mockUseTestRuns.mockReturnValue({
    data: connection(nodes),
    isLoading: false,
    isFetching: false,
    error: null,
  });
  return render(<TestRunsList />);
}

describe('TestRunsList project column', () => {
  beforeEach(() => {
    mockUseTestRuns.mockReset();
    window.sessionStorage.clear();
  });
  afterEach(cleanup);

  it('shows both the project display name and the project id', () => {
    renderWith([node({ project_id: 'proj-a', project_name: 'Alpha Service' })]);
    expect(screen.getByText('Alpha Service')).toBeTruthy();
    expect(screen.getByText('proj-a')).toBeTruthy();
  });

  it('shows the id alone when no name is available', () => {
    renderWith([node({ project_id: 'proj-a' })]);
    expect(screen.getAllByText('proj-a')).toHaveLength(1);
  });

  it('shows the id alone when the name is an empty string', () => {
    renderWith([node({ project_id: 'proj-a', project_name: '' })]);
    expect(screen.getAllByText('proj-a')).toHaveLength(1);
  });

  it('links the project cell to the project id', () => {
    renderWith([node({ project_id: 'proj-a', project_name: 'Alpha Service' })]);
    const link = screen.getByText('Alpha Service').closest('a');
    expect(link?.getAttribute('href')).toBe('/projects/proj-a');
  });
});

describe('TestRunsList duration column', () => {
  beforeEach(() => {
    mockUseTestRuns.mockReset();
    window.sessionStorage.clear();
  });
  afterEach(cleanup);

  it('renders the duration reported by the server', () => {
    renderWith([node({ duration_ms: 90_000 })]);
    expect(screen.getByText('1m 30s')).toBeTruthy();
  });

  // A run with no end_time cannot have its duration derived client-side;
  // the server-reported value is the only source.
  it('renders a duration for a run that has no end time', () => {
    renderWith([node({ end_time: null, status: 'running', duration_ms: 30_000 })]);
    expect(screen.getByText('30.0s')).toBeTruthy();
  });

  // Older backends do not send duration_ms; fall back to the timestamps.
  it('falls back to end_time - start_time when duration_ms is absent', () => {
    renderWith([
      node({
        start_time: '2026-01-01T00:00:00Z',
        end_time: '2026-01-01T00:00:45Z',
      }),
    ]);
    expect(screen.getByText('45.0s')).toBeTruthy();
  });

  it('shows a dash when neither a duration nor an end time is available', () => {
    renderWith([node({ end_time: null, status: 'running' })]);
    expect(screen.getByText('—')).toBeTruthy();
  });
});

describe('TestRunsList duration for in-flight runs', () => {
  beforeEach(() => {
    mockUseTestRuns.mockReset();
    window.sessionStorage.clear();
  });
  afterEach(cleanup);

  // The API always sends duration_ms (no omitempty), so a run with no
  // recorded duration arrives as 0 rather than absent. That is "unknown",
  // not "zero milliseconds".
  it('does not render 0ms for a running run with no recorded duration', () => {
    renderWith([node({ end_time: null, status: 'running', duration_ms: 0 })]);
    expect(screen.queryByText('0ms')).toBeNull();
    expect(screen.getByText('—')).toBeTruthy();
  });

  it('falls back to the timestamps when duration_ms is zero but the run finished', () => {
    renderWith([
      node({
        duration_ms: 0,
        start_time: '2026-01-01T00:00:00Z',
        end_time: '2026-01-01T00:00:45Z',
      }),
    ]);
    expect(screen.getByText('45.0s')).toBeTruthy();
  });
});
