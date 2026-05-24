import { describe, it, expect } from 'vitest';
import {
  flattenPages,
  shouldFetchMore,
  MAX_PROJECTS,
  type ProjectsPage,
} from './pagedProjects';
import type { Project } from '@/lib/types';

const proj = (id: string, over: Partial<Project> = {}): Project => ({
  id,
  projectId: id,
  name: `Project ${id}`,
  description: '',
  isActive: true,
  team: '',
  canManage: false,
  stats: null,
  ...over,
});

const page = (
  edgesIn: Project[],
  hasNextPage: boolean,
  totalCount: number,
): ProjectsPage => ({
  totalCount,
  edges: edgesIn.map((p) => ({ cursor: `c:${p.id}`, node: p })),
  pageInfo: {
    hasNextPage,
    endCursor: edgesIn.length ? `c:${edgesIn[edgesIn.length - 1]!.id}` : null,
  },
});

describe('flattenPages', () => {
  it('returns empty result on no pages', () => {
    expect(flattenPages([])).toEqual({
      projects: [],
      totalCount: 0,
      loadedCount: 0,
      truncated: false,
    });
  });

  it('concatenates edges across pages in order', () => {
    const got = flattenPages([
      page([proj('a'), proj('b')], true, 5),
      page([proj('c'), proj('d'), proj('e')], false, 5),
    ]);
    expect(got.projects.map((p) => p.id)).toEqual(['a', 'b', 'c', 'd', 'e']);
    expect(got.totalCount).toBe(5);
    expect(got.loadedCount).toBe(5);
    expect(got.truncated).toBe(false);
  });

  it('deduplicates by id when pages overlap', () => {
    const got = flattenPages([
      page([proj('a'), proj('b')], true, 3),
      page([proj('b'), proj('c')], false, 3),
    ]);
    expect(got.projects.map((p) => p.id)).toEqual(['a', 'b', 'c']);
  });

  it('reports truncated when loadedCount hits MAX and totalCount is higher', () => {
    // Build MAX_PROJECTS unique projects across pages
    const projects: Project[] = [];
    for (let i = 0; i < MAX_PROJECTS; i++) projects.push(proj(String(i)));
    const got = flattenPages([page(projects, false, MAX_PROJECTS + 200)]);
    expect(got.loadedCount).toBe(MAX_PROJECTS);
    expect(got.truncated).toBe(true);
  });

  it('does not flag truncated when total equals loaded', () => {
    const got = flattenPages([page([proj('a'), proj('b')], false, 2)]);
    expect(got.truncated).toBe(false);
  });
});

describe('shouldFetchMore', () => {
  it('stops when server reports no next page', () => {
    expect(shouldFetchMore(page([], false, 10), 10)).toBe(false);
  });

  it('continues when server has more and cap not yet hit', () => {
    expect(shouldFetchMore(page([], true, 1000), 100)).toBe(true);
  });

  it('stops once cap is hit, even if server has more', () => {
    expect(shouldFetchMore(page([], true, 1000), MAX_PROJECTS)).toBe(false);
  });
});
