// Issue #216: the project detail header identified the project by its
// raw projectId. It now leads with the display name and keeps the id
// visible beneath, matching the Test Runs list and run detail header.

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, cleanup, fireEvent } from '@testing-library/react';

vi.mock('@tanstack/react-router', () => ({
  Link: ({ to, children }: { to: string; children: React.ReactNode }) => (
    <a href={to} data-to={to}>{children}</a>
  ),
  useParams: () => ({ projectId: 'java-1-001' }),
  useRouter: () => ({ history: { length: 1, back: vi.fn() } }),
}));

const mockRuns = vi.fn();
const mockProject = vi.fn();
vi.mock('@tanstack/react-query', () => ({
  useQuery: (opts: { queryKey: unknown[] }) =>
    (opts.queryKey[0] === 'project' ? mockProject : mockRuns)(opts),
}));

vi.mock('./TestHistoryChart', () => ({
  TestHistoryChart: () => <div data-testid="history-chart" />,
}));

const restFetchSpy = vi.fn();
vi.mock('@/lib/api', () => ({
  restFetch: (url: string) => restFetchSpy(url),
  graphqlFetch: vi.fn(),
}));

import ProjectDetail from './ProjectDetail';

function setup(projectName: string | null) {
  mockRuns.mockReturnValue({
    data: {
      edges: [],
      pageInfo: { hasNextPage: false, endCursor: '' },
      totalCount: 0,
      totalCountIsEstimate: false,
      facets: { byStatus: [], byBranch: [], byTag: [], byProject: [] },
    },
    isLoading: false,
    error: null,
  });
  mockProject.mockReturnValue({
    data: projectName === null ? null : { name: projectName },
    isLoading: false,
    error: null,
  });
  return render(<ProjectDetail />);
}

describe('ProjectDetail header', () => {
  beforeEach(() => {
    mockRuns.mockReset();
    mockProject.mockReset();
  });
  afterEach(cleanup);

  it('leads with the project display name', () => {
    setup('Java Service 1');
    expect(screen.getByRole('heading', { level: 1 }).textContent).toBe('Java Service 1');
  });

  it('still shows the project id alongside the name', () => {
    setup('Java Service 1');
    expect(screen.getByText('java-1-001')).toBeTruthy();
  });

  it('falls back to the id as the heading when no name is available', () => {
    setup(null);
    expect(screen.getByRole('heading', { level: 1 }).textContent).toBe('java-1-001');
    // and does not repeat the id underneath
    expect(screen.getAllByText('java-1-001')).toHaveLength(1);
  });
});

// Time filter on the project page. The v2 endpoint already clamps to the
// last 7 days when a client sends no from/to, so the page previously had
// an invisible window; these presets make it explicit and adjustable.
describe('ProjectDetail time filter', () => {
  beforeEach(() => {
    mockRuns.mockReset();
    mockProject.mockReset();
  });
  afterEach(cleanup);

  // The runs query URL is built inside queryFn; capture it by invoking
  // the queryFn the component handed to useQuery.
  function runsUrl(): string {
    const opts = mockRuns.mock.calls.at(-1)?.[0];
    let captured = '';
    restFetchSpy.mockImplementation((url: string) => {
      captured = url;
      return Promise.resolve(null);
    });
    opts.queryFn();
    return captured;
  }

  it('defaults to the last 7 days', () => {
    setup('Java Service 1');
    const url = runsUrl();
    expect(url).toContain('from=');
    expect(url).toContain('to=');
    const params = new URLSearchParams(url.split('?')[1]);
    const span =
      new Date(params.get('to')!).getTime() - new Date(params.get('from')!).getTime();
    expect(Math.round(span / 86_400_000)).toBe(7);
  });

  it('marks the active preset', () => {
    setup('Java Service 1');
    const btn = screen.getByRole('button', { name: 'Last 7d' });
    expect(btn.className).toContain('border-primary');
  });

  it('switches the window when another preset is picked', () => {
    setup('Java Service 1');
    fireEvent.click(screen.getByRole('button', { name: 'Last 30d' }));
    const params = new URLSearchParams(runsUrl().split('?')[1]);
    const span =
      new Date(params.get('to')!).getTime() - new Date(params.get('from')!).getTime();
    expect(Math.round(span / 86_400_000)).toBe(30);
  });

  it('drops the window and opts out of the server default for All time', () => {
    setup('Java Service 1');
    fireEvent.click(screen.getByRole('button', { name: 'All time' }));
    const url = runsUrl();
    expect(url).not.toContain('from=');
    expect(url).toContain('allTime=1');
  });

  it('keeps the project filter on every request', () => {
    setup('Java Service 1');
    expect(runsUrl()).toContain('project=java-1-001');
  });
});

// Active state on the presets is otherwise conveyed by colour alone.
describe('DateRangeFilter accessibility', () => {
  beforeEach(() => {
    mockRuns.mockReset();
    mockProject.mockReset();
  });
  afterEach(cleanup);

  it('exposes the selected range to assistive tech', () => {
    setup('Java Service 1');
    expect(screen.getByRole('button', { name: 'Last 7d' }).getAttribute('aria-pressed')).toBe('true');
    expect(screen.getByRole('button', { name: 'Last 30d' }).getAttribute('aria-pressed')).toBe('false');
  });

  it('moves the pressed state when another range is chosen', () => {
    setup('Java Service 1');
    fireEvent.click(screen.getByRole('button', { name: 'Last 30d' }));
    expect(screen.getByRole('button', { name: 'Last 30d' }).getAttribute('aria-pressed')).toBe('true');
    expect(screen.getByRole('button', { name: 'Last 7d' }).getAttribute('aria-pressed')).toBe('false');
  });

  it('marks All time pressed when the range is cleared', () => {
    setup('Java Service 1');
    fireEvent.click(screen.getByRole('button', { name: 'All time' }));
    expect(screen.getByRole('button', { name: 'All time' }).getAttribute('aria-pressed')).toBe('true');
  });
});
