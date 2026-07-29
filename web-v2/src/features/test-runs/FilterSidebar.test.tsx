// Covers the Project facet on the test-runs filter sidebar (issue #216):
// entries render the project's display name *and* its id, sort by the
// name, fall back to the id alone when unlabelled, and still emit the id
// when toggled so saved views and ?project= links keep working.

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup, within } from '@testing-library/react';
import type { FacetCount } from '@/lib/types';

vi.mock('../profile/hooks', () => ({
  useUserPreferences: () => ({ data: { userPreferences: { favorites: [] } } }),
}));

import { FilterSidebar } from './FilterSidebar';
import type { TestRunsFilter } from './hooks';

function renderSidebar(
  byProject: FacetCount[],
  filter: TestRunsFilter = {},
) {
  const onChange = vi.fn();
  render(
    <FilterSidebar
      filter={filter}
      onChange={onChange}
      facets={{ byStatus: [], byBranch: [], byTag: [], byProject }}
    />,
  );
  return { onChange };
}

// The sidebar renders one <section> per facet, headed by the facet name.
function projectSection(): HTMLElement {
  const heading = screen.getByText('Project');
  const section = heading.closest('section');
  if (!section) throw new Error('project facet section not found');
  return section;
}

describe('FilterSidebar project facet', () => {
  beforeEach(() => vi.clearAllMocks());
  afterEach(cleanup);

  it('renders both the project display name and the id', () => {
    renderSidebar([{ value: 'proj-a', count: 4, label: 'Alpha Service' }]);
    const section = projectSection();
    expect(within(section).getByText('Alpha Service')).toBeTruthy();
    expect(within(section).getByText('proj-a')).toBeTruthy();
  });

  it('shows the id alone when the facet has no label', () => {
    renderSidebar([{ value: 'proj-unnamed', count: 2 }]);
    expect(within(projectSection()).getAllByText('proj-unnamed')).toHaveLength(1);
  });

  it('sorts by the name, not the underlying id', () => {
    renderSidebar([
      { value: 'proj-001', count: 1, label: 'Zulu Service' },
      { value: 'proj-999', count: 1, label: 'Alpha Service' },
      { value: 'proj-500', count: 1, label: 'Mike Service' },
    ]);
    const shown = within(projectSection())
      .getAllByRole('button')
      .map((b) => b.querySelector('span > span')?.textContent);
    expect(shown).toEqual(['Alpha Service', 'Mike Service', 'Zulu Service']);
  });

  it('emits the project id — not the label — when an entry is toggled', () => {
    const { onChange } = renderSidebar([
      { value: 'proj-a', count: 4, label: 'Alpha Service' },
    ]);
    fireEvent.click(within(projectSection()).getByText('Alpha Service'));
    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({ project: ['proj-a'] }),
    );
  });

  it('marks an entry selected by id, even though it displays a name', () => {
    renderSidebar(
      [{ value: 'proj-a', count: 4, label: 'Alpha Service' }],
      { project: ['proj-a'] },
    );
    const button = within(projectSection()).getByText('Alpha Service').closest('button');
    expect(button?.className).toContain('text-primary');
  });

  it('still shows the count alongside the name', () => {
    renderSidebar([{ value: 'proj-a', count: 7, label: 'Alpha Service' }]);
    expect(within(projectSection()).getByText('7')).toBeTruthy();
  });
});

// The custom date inputs must be able to clear a bound. customRangeToQuery
// omits keys for empty inputs, so a caller that spreads its result over
// the existing filter would silently keep the old bound.
describe('FilterSidebar date range', () => {
  beforeEach(() => vi.clearAllMocks());
  afterEach(cleanup);

  it('clears the To bound when its input is emptied', () => {
    const onChange = vi.fn();
    render(
      <FilterSidebar
        filter={{ from: '2026-01-01T00:00:00.000Z', to: '2026-01-31T23:59:59.999Z' }}
        onChange={onChange}
        facets={{ byStatus: [], byBranch: [], byTag: [], byProject: [] }}
      />,
    );
    fireEvent.change(screen.getByLabelText('To'), { target: { value: '' } });
    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({ from: '2026-01-01T00:00:00.000Z', to: undefined }),
    );
  });

  it('clears the From bound when its input is emptied', () => {
    const onChange = vi.fn();
    render(
      <FilterSidebar
        filter={{ from: '2026-01-01T00:00:00.000Z', to: '2026-01-31T23:59:59.999Z' }}
        onChange={onChange}
        facets={{ byStatus: [], byBranch: [], byTag: [], byProject: [] }}
      />,
    );
    fireEvent.change(screen.getByLabelText('From'), { target: { value: '' } });
    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({ from: undefined, to: '2026-01-31T23:59:59.999Z' }),
    );
  });
});
