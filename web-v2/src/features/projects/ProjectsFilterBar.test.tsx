import { describe, it, expect, vi } from 'vitest';
import { render, within } from '@testing-library/react';
import { ProjectsFilterBar, DEFAULT_FILTER } from './ProjectsFilterBar';

// Query within each render's own container — this suite's setup doesn't
// auto-clean the DOM between tests, so a shared `screen` would see leftover
// nodes from the previous render.
function renderBar(showSort?: boolean) {
  const { container } = render(
    <ProjectsFilterBar
      filter={DEFAULT_FILTER}
      onChange={vi.fn()}
      availableTeams={['team-a']}
      favoritesCount={0}
      visibleCount={3}
      totalCount={3}
      {...(showSort === undefined ? {} : { showSort })}
    />,
  );
  return within(container);
}

describe('ProjectsFilterBar sort control', () => {
  it('shows the Sort control by default (card view / Projects grid)', () => {
    const q = renderBar();
    expect(q.getByText('Sort')).toBeTruthy();
    expect(q.getByLabelText('Toggle sort direction')).toBeTruthy();
  });

  it('hides the Sort control when showSort is false (tree view)', () => {
    const q = renderBar(false);
    expect(q.queryByText('Sort')).toBeNull();
    expect(q.queryByLabelText('Toggle sort direction')).toBeNull();
    // Other filters remain available in tree view.
    expect(q.getByText('Team')).toBeTruthy();
    expect(q.getByText('Favorites')).toBeTruthy();
  });
});
