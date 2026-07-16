import { useEffect } from 'react';
import { useInfiniteQuery, useQueryClient } from '@tanstack/react-query';
import { graphqlFetch } from '@/lib/api';
import { useCurrentUser } from '@/features/auth/useCurrentUser';
import {
  flattenPages,
  shouldFetchMore,
  type FlattenedProjects,
  type ProjectsPage,
} from './pagedProjects';

const PAGE_SIZE = 100;

const GET_PROJECTS = /* GraphQL */ `
  query GetProjects($first: Int, $after: String) {
    projects(first: $first, after: $after) {
      totalCount
      pageInfo {
        hasNextPage
        endCursor
      }
      edges {
        cursor
        node {
          id
          projectId
          name
          description
          isActive
          team
          canManage
          stats {
            totalTestRuns
            successRate
            averageDuration
            lastRunTime
          }
        }
      }
    }
  }
`;

interface ProjectsResp {
  projects: ProjectsPage;
}

export interface UseProjectsResult {
  data: FlattenedProjects | undefined;
  isLoading: boolean;
  isFetchingMore: boolean;
  error: Error | null;
}

// useProjects pages through the GraphQL `projects` query and returns
// the flattened list. Auto-fetches every page (up to MAX_PROJECTS in
// pagedProjects.ts) so client-side filters in callers operate over the
// full set. Caller doesn't need to think about pagination.
//
// The query key includes the signed-in user's role so a role change
// (sign-in, role promotion via /admin, etc.) busts the cache and
// re-fetches. Without this, an admin who landed on the page before
// their auth resolved would see canManage=false everywhere — the gear
// on the project tiles would be hidden until they hard-reloaded.
export function useProjects(): UseProjectsResult {
  const qc = useQueryClient();
  const { data: user } = useCurrentUser();
  const role = user?.role ?? 'anonymous';

  // When the user transitions from anonymous → signed-in (or role
  // changes), drop the previous projects cache. Otherwise React Query
  // would keep serving the older `canManage: false` rows because the
  // query key doesn't depend on role-baked state.
  useEffect(() => {
    qc.invalidateQueries({ queryKey: ['projects', 'paged'] });
  }, [role, qc]);

  const q = useInfiniteQuery({
    queryKey: ['projects', 'paged', PAGE_SIZE, role],
    queryFn: async ({ pageParam }) => {
      const { projects } = await graphqlFetch<ProjectsResp>(GET_PROJECTS, {
        first: PAGE_SIZE,
        after: pageParam,
      });
      return projects;
    },
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (lastPage, allPages) => {
      const loaded = allPages.reduce((s, p) => s + p.edges.length, 0);
      if (!shouldFetchMore(lastPage, loaded)) return undefined;
      return lastPage.pageInfo.endCursor ?? undefined;
    },
    staleTime: 60_000,
  });

  // Auto-advance: keep fetching pages until shouldFetchMore says stop.
  // useInfiniteQuery doesn't do this on its own — getNextPageParam only
  // produces the next param when fetchNextPage() is explicitly called.
  useEffect(() => {
    if (q.hasNextPage && !q.isFetchingNextPage && !q.isFetching) {
      q.fetchNextPage();
    }
  }, [q.hasNextPage, q.isFetchingNextPage, q.isFetching, q]);

  const flattened = q.data ? flattenPages(q.data.pages) : undefined;
  return {
    data: flattened,
    // Treat the *first* page as "loading" so consumers can show their
    // existing spinner; subsequent pages stream in.
    isLoading: q.isLoading,
    isFetchingMore: q.isFetchingNextPage,
    error: (q.error as Error) ?? null,
  };
}
