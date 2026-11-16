import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, cleanup, fireEvent, within } from '@testing-library/react';

const tags = [
  { id: '1', name: 'jira:gwcp-1', category: 'jira', value: 'gwcp-1', description: null },
  { id: '2', name: 'smoke', category: '', value: 'smoke', description: 'Quick checks' },
];

vi.mock('@tanstack/react-query', () => ({
  useQuery: () => ({
    data: { tags: { totalCount: tags.length, edges: tags.map((node) => ({ node })) } },
    isLoading: false,
    error: null,
  }),
}));
vi.mock('@/lib/api', () => ({ graphqlFetch: vi.fn() }));

import TagsManagement from './TagsManagement';

const EMPTY = 'No tags match your search or category filter';

describe('TagsManagement empty state', () => {
  afterEach(cleanup);

  it('shows the empty state when the category filter hides every search match', () => {
    render(<TagsManagement />);
    // "smoke" matches the search but lives in "uncategorized"…
    fireEvent.change(screen.getByPlaceholderText('Search tags…'), { target: { value: 'smoke' } });
    expect(screen.queryByText(EMPTY)).toBeNull();
    // …so picking only the jira category leaves nothing to show.
    const chips = screen.getByRole('group', { name: 'Category' });
    fireEvent.click(within(chips).getByRole('button', { name: /^jira/ }));
    expect(screen.getByText(EMPTY)).toBeTruthy();
  });

  it('shows the empty state when the search matches nothing', () => {
    render(<TagsManagement />);
    fireEvent.change(screen.getByPlaceholderText('Search tags…'), { target: { value: 'zzz' } });
    expect(screen.getByText(EMPTY)).toBeTruthy();
  });
});
