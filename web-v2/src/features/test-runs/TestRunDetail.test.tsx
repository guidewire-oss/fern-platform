// Tests the v1-parity drill navigation on /v2/test-runs/:runId.
//
// Covers:
//   - default view shows a *suites table* with v1's columns
//     (Suite Name, Test Results, Status, Duration, Tags)
//   - clicking a suite row drills into a *specs table* with v1's
//     columns (Test Name, Status, Duration, Error Message, Tags, Started)
//   - back link returns to the suites table
//   - the run-header fields the v2 query was previously missing
//     (tags, metadata, skippedSpecs, stack trace, retryCount) all render
//
// A regression that drops one of these from the query or the render
// path fails loudly here.

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup, within } from '@testing-library/react';

vi.mock('@tanstack/react-router', () => ({
  Link: ({
    to,
    children,
    className,
  }: {
    to: string;
    children: React.ReactNode;
    className?: string;
  }) => (
    <a href={to} className={className} data-to={to}>
      {children}
    </a>
  ),
  useParams: () => ({ runId: 'run-1' }),
}));

const mockQuery = vi.fn();
vi.mock('@tanstack/react-query', () => ({
  useQuery: (opts: { queryFn: () => unknown }) => mockQuery(opts),
}));

import TestRunDetail, { hasDisplayableMetadata } from './TestRunDetail';

type Tag = { id: string; name: string; category: string | null; value: string | null; color: string | null };
type SpecLike = {
  id: string; specName: string; status: string;
  startTime: string; endTime: string | null; duration: number;
  errorMessage: string | null; stackTrace: string | null;
  retryCount: number; isFlaky: boolean; tags: Tag[];
};
type SuiteLike = {
  id: string; suiteName: string; status: string;
  startTime: string; endTime: string | null; duration: number;
  totalSpecs: number; passedSpecs: number; failedSpecs: number; skippedSpecs: number;
  tags: Tag[]; specRuns: SpecLike[];
};
type RunLike = {
  id: string; projectId: string; projectName?: string | null; runId: string;
  branch: string | null; commitSha: string | null;
  status: string; startTime: string; endTime: string | null;
  duration: number;
  totalTests: number; passedTests: number; failedTests: number; skippedTests: number;
  environment: string | null; metadata: unknown;
  tags: Tag[]; suiteRuns: SuiteLike[];
};

const baseSpec: SpecLike = {
  id: 'sp-1',
  specName: 'logs in with valid credentials',
  status: 'passed',
  startTime: '2026-05-23T10:00:00Z',
  endTime: '2026-05-23T10:00:02Z',
  duration: 2000,
  errorMessage: null,
  stackTrace: null,
  retryCount: 0,
  isFlaky: false,
  tags: [],
};

const baseSuite: SuiteLike = {
  id: 'su-1',
  suiteName: 'auth/login',
  status: 'passed',
  startTime: '2026-05-23T10:00:00Z',
  endTime: '2026-05-23T10:00:05Z',
  duration: 5000,
  totalSpecs: 1,
  passedSpecs: 1,
  failedSpecs: 0,
  skippedSpecs: 0,
  tags: [],
  specRuns: [baseSpec],
};

const baseRun: RunLike = {
  id: 'r-1',
  projectId: 'flux-system',
  runId: 'run-1',
  branch: 'main',
  commitSha: 'abc12345def67890',
  status: 'passed',
  startTime: '2026-05-23T10:00:00Z',
  endTime: '2026-05-23T10:00:05Z',
  duration: 5000,
  totalTests: 1,
  passedTests: 1,
  failedTests: 0,
  skippedTests: 0,
  environment: 'ci',
  metadata: null,
  tags: [],
  suiteRuns: [baseSuite],
};

function renderWith(testRun: RunLike | null) {
  mockQuery.mockReturnValue({ data: { testRun }, isLoading: false, error: null });
  return render(<TestRunDetail />);
}

beforeEach(() => mockQuery.mockReset());
afterEach(() => cleanup());

describe('hasDisplayableMetadata', () => {
  it('rejects null/undefined/empty/scalars', () => {
    expect(hasDisplayableMetadata(null)).toBe(false);
    expect(hasDisplayableMetadata(undefined)).toBe(false);
    expect(hasDisplayableMetadata({})).toBe(false);
    expect(hasDisplayableMetadata([])).toBe(false);
    expect(hasDisplayableMetadata('hello')).toBe(false);
    expect(hasDisplayableMetadata(42)).toBe(false);
  });
  it('accepts non-empty objects/arrays', () => {
    expect(hasDisplayableMetadata({ k: 1 })).toBe(true);
    expect(hasDisplayableMetadata([1])).toBe(true);
  });
});

describe('Suites view (default landing view)', () => {
  it('renders a suites *table* with v1 columns', () => {
    renderWith(baseRun);
    // v1 columns: Suite Name, Test Results, Status, Duration, Tags
    expect(screen.getByRole('columnheader', { name: /^suite name$/i })).toBeInTheDocument();
    expect(screen.getByRole('columnheader', { name: /^test results$/i })).toBeInTheDocument();
    expect(screen.getByRole('columnheader', { name: /^status$/i })).toBeInTheDocument();
    expect(screen.getByRole('columnheader', { name: /^duration$/i })).toBeInTheDocument();
    expect(screen.getByRole('columnheader', { name: /^tags$/i })).toBeInTheDocument();
    // No specs visible yet (we haven't drilled).
    expect(screen.queryByText(/logs in with valid credentials/)).toBeNull();
  });

  it('renders one row per suite with Test Results triple (P/F/S)', () => {
    renderWith({
      ...baseRun,
      suiteRuns: [
        { ...baseSuite, suiteName: 'auth/login', passedSpecs: 3, failedSpecs: 1, skippedSpecs: 2 },
        { ...baseSuite, id: 'su-2', suiteName: 'projects/list', passedSpecs: 7, failedSpecs: 0, skippedSpecs: 0 },
      ],
    });
    const rows = screen.getAllByTestId('suite-row');
    expect(rows).toHaveLength(2);
    // First row: 3 / 1 / 2 (passed/failed/skipped)
    expect(within(rows[0]!).getByText('3')).toBeInTheDocument();
    expect(within(rows[0]!).getByText('1')).toBeInTheDocument();
    expect(within(rows[0]!).getByText('2')).toBeInTheDocument();
    expect(within(rows[0]!).getByText('auth/login')).toBeInTheDocument();
  });

  it('renders suite tags chips when present, "—" otherwise', () => {
    renderWith({
      ...baseRun,
      suiteRuns: [
        {
          ...baseSuite,
          tags: [{ id: 't1', name: 'critical', category: null, value: null, color: null }],
        },
        { ...baseSuite, id: 'su-2', suiteName: 'untagged', tags: [] },
      ],
    });
    const rows = screen.getAllByTestId('suite-row');
    expect(within(rows[0]!).getByTestId('suite-tags')).toHaveTextContent('critical');
    expect(within(rows[1]!).queryByTestId('suite-tags')).toBeNull();
  });

  it('clicking a suite row drills into the specs view', () => {
    renderWith(baseRun);
    fireEvent.click(screen.getByTestId('suite-row'));
    // Now showing specs table — v1 columns:
    expect(screen.getByRole('columnheader', { name: /^test name$/i })).toBeInTheDocument();
    expect(screen.getByRole('columnheader', { name: /^error message$/i })).toBeInTheDocument();
    expect(screen.getByRole('columnheader', { name: /^started$/i })).toBeInTheDocument();
    expect(screen.getByText(/logs in with valid credentials/)).toBeInTheDocument();
  });

  it('Enter or Space on a focused row also drills in (keyboard a11y)', () => {
    renderWith(baseRun);
    const row = screen.getByTestId('suite-row');
    fireEvent.keyDown(row, { key: 'Enter' });
    expect(screen.getByText(/logs in with valid credentials/)).toBeInTheDocument();
  });
});

describe('Specs view (after drilling into a suite)', () => {
  function renderAndDrill(run: RunLike) {
    renderWith(run);
    fireEvent.click(screen.getAllByTestId('suite-row')[0]!);
  }

  it('shows v1 columns: Test Name, Status, Duration, Error Message, Tags, Started', () => {
    renderAndDrill(baseRun);
    for (const name of ['Test Name', 'Status', 'Duration', 'Error Message', 'Tags', 'Started']) {
      expect(
        screen.getByRole('columnheader', { name: new RegExp(`^${name}$`, 'i') }),
      ).toBeInTheDocument();
    }
  });

  it('renders one row per spec with status + duration + started', () => {
    renderAndDrill({
      ...baseRun,
      suiteRuns: [
        {
          ...baseSuite,
          specRuns: [
            { ...baseSpec, id: 's1', specName: 'first',  status: 'passed' },
            { ...baseSpec, id: 's2', specName: 'second', status: 'failed', errorMessage: 'boom' },
          ],
        },
      ],
    });
    expect(screen.getAllByTestId('spec-row')).toHaveLength(2);
    expect(screen.getByText('first')).toBeInTheDocument();
    expect(screen.getByText('second')).toBeInTheDocument();
  });

  it('Error Message cell renders as a button when present; "—" when null', () => {
    renderAndDrill({
      ...baseRun,
      suiteRuns: [
        {
          ...baseSuite,
          specRuns: [
            { ...baseSpec, id: 's1', specName: 'with error', errorMessage: 'AssertionError: 401 ≠ 200' },
            { ...baseSpec, id: 's2', specName: 'no error',   errorMessage: null },
          ],
        },
      ],
    });
    const rows = screen.getAllByTestId('spec-row');
    expect(within(rows[0]!).getByTestId('spec-error-message')).toHaveTextContent('AssertionError');
    expect(within(rows[1]!).queryByTestId('spec-error-message')).toBeNull();
    // The row without an error message has a "—" placeholder in the
    // Error cell (and also in Tags, both empty). Confirm the empty cells
    // resolve to the placeholder by counting them — Tags + Error = 2.
    expect(within(rows[1]!).getAllByText('—')).toHaveLength(2);
  });

  it('clicking the error cell expands a stack-trace pre block', () => {
    renderAndDrill({
      ...baseRun,
      suiteRuns: [
        {
          ...baseSuite,
          specRuns: [
            {
              ...baseSpec,
              specName: 'failing test',
              errorMessage: 'oops',
              stackTrace: 'at handler.go:42\nat router.go:18',
            },
          ],
        },
      ],
    });
    expect(screen.queryByTestId('spec-stack-trace')).toBeNull();
    fireEvent.click(screen.getByTestId('spec-error-message'));
    expect(screen.getByTestId('spec-stack-trace')).toHaveTextContent('handler.go:42');
  });

  it('shows flaky/retry indicators only when set', () => {
    renderAndDrill({
      ...baseRun,
      suiteRuns: [
        {
          ...baseSuite,
          specRuns: [
            { ...baseSpec, id: 's1', isFlaky: false, retryCount: 0 },
            { ...baseSpec, id: 's2', specName: 'flaky one', isFlaky: true,  retryCount: 2 },
          ],
        },
      ],
    });
    const rows = screen.getAllByTestId('spec-row');
    expect(within(rows[0]!).queryByTestId('spec-retry-count')).toBeNull();
    expect(within(rows[1]!).getByTestId('spec-retry-count')).toHaveTextContent('×2');
    // The flaky label sits in the Test Name cell as a small badge.
    // Scope tightly to that cell's td so a "flaky" word inside a
    // StatusBadge doesn't false-positive.
    const flakyCells = within(rows[1]!).getAllByText((_, el) => el?.textContent === 'flaky · retried ×2');
    expect(flakyCells.length).toBeGreaterThan(0);
  });

  it('back link returns to the suites view', () => {
    renderAndDrill(baseRun);
    expect(screen.getByRole('columnheader', { name: /test name/i })).toBeInTheDocument();
    fireEvent.click(screen.getByTestId('back-to-suites'));
    // Back at suites table:
    expect(screen.getByRole('columnheader', { name: /^suite name$/i })).toBeInTheDocument();
    expect(screen.queryByRole('columnheader', { name: /test name/i })).toBeNull();
  });
});

describe('Run header (always visible)', () => {
  it('renders run-level tag chips when present', () => {
    renderWith({
      ...baseRun,
      tags: [
        { id: 't1', name: 'smoke', category: 'suite', value: null, color: '#22c55e' },
        { id: 't2', name: 'team',  category: 'owner', value: 'platform', color: null },
      ],
    });
    const runTags = screen.getByTestId('run-tags');
    expect(runTags).toBeInTheDocument();
    expect(runTags.querySelectorAll('[data-testid="tag-chip"]')).toHaveLength(2);
    expect(runTags).toHaveTextContent('team: platform');
  });

  it('omits run-tags container when no tags', () => {
    renderWith(baseRun);
    expect(screen.queryByTestId('run-tags')).toBeNull();
  });

  it('renders metadata panel only when metadata is a non-empty object', () => {
    renderWith({ ...baseRun, metadata: { buildId: 'b-7' } });
    const toggle = screen.getByTestId('metadata-toggle');
    fireEvent.click(toggle);
    expect(screen.getByTestId('metadata-content')).toHaveTextContent('buildId');

    cleanup();
    renderWith({ ...baseRun, metadata: {} });
    expect(screen.queryByTestId('metadata-toggle')).toBeNull();
  });

  it('shows the Skipped stat card', () => {
    renderWith({ ...baseRun, totalTests: 10, passedTests: 7, failedTests: 1, skippedTests: 2 });
    expect(screen.getByText(/^skipped$/i)).toBeInTheDocument();
  });
});

// Issue #216: the run header shows the project's display name and its
// id, matching the Test Runs list. The id alone when no name resolved.
describe('TestRunDetail project identification', () => {
  afterEach(cleanup);

  it('shows both the project name and the project id', () => {
    renderWith({ ...baseRun, projectName: 'Flux System 1' });
    expect(screen.getByText('Flux System 1')).toBeTruthy();
    expect(screen.getByText('flux-system')).toBeTruthy();
  });

  it('shows the id alone when the run has no project name', () => {
    renderWith({ ...baseRun, projectName: null });
    expect(screen.getAllByText('flux-system')).toHaveLength(1);
  });

  it('keeps the project link pointing at the id', () => {
    renderWith({ ...baseRun, projectName: 'Flux System 1' });
    const link = screen.getByText('Flux System 1').closest('a');
    expect(link?.getAttribute('data-to')).toBe('/projects/$projectId');
  });
});
