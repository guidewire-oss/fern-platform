// Helpers for paginating the `projects` GraphQL query.
//
// The server supports cursor pagination but v2 callers (ProjectsList,
// TestSummaries, Dashboard) want a flat list of every project the
// signed-in user can see, so the page-grid filter bar can operate over
// the full set. Auto-paginate up to MAX_PROJECTS so a few-thousand-row
// DB doesn't burn the browser; expose a `truncated` flag callers can
// surface to the user.

import type { Project } from '@/lib/types';

export interface ProjectsPage {
  totalCount: number;
  edges: Array<{ cursor: string; node: Project }>;
  pageInfo: {
    hasNextPage: boolean;
    endCursor: string | null;
  };
}

// Treemap caps at 500 visible nodes (FR-16). Matching that here keeps
// the two views consistent — projects shown on /v2/projects line up
// with projects renderable on /v2/treemap.
export const MAX_PROJECTS = 500;

export interface FlattenedProjects {
  projects: Project[];
  totalCount: number;
  loadedCount: number;
  truncated: boolean;
}

export function flattenPages(pages: readonly ProjectsPage[]): FlattenedProjects {
  const seen = new Set<string>();
  const projects: Project[] = [];
  for (const page of pages) {
    for (const edge of page.edges) {
      // Deduplicate on `id` in case the server returns overlapping
      // pages (rare, but possible when projects are created between
      // page fetches — the cursor includes the timestamp, so a new
      // row at the boundary can land on both sides).
      if (seen.has(edge.node.id)) continue;
      seen.add(edge.node.id);
      projects.push(edge.node);
    }
  }
  const totalCount = pages.at(-1)?.totalCount ?? projects.length;
  return {
    projects,
    totalCount,
    loadedCount: projects.length,
    truncated: projects.length >= MAX_PROJECTS && projects.length < totalCount,
  };
}

// shouldFetchMore tells useInfiniteQuery when to stop. Stop when the
// server says there's no next page, OR when we've hit the local cap
// (so the browser never churns through more than MAX_PROJECTS).
export function shouldFetchMore(lastPage: ProjectsPage, loadedSoFar: number): boolean {
  if (!lastPage.pageInfo.hasNextPage) return false;
  if (loadedSoFar >= MAX_PROJECTS) return false;
  return true;
}
