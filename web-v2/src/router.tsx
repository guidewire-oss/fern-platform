// Code-based TanStack Router setup. We prefer code-based over
// file-based routing because the route tree is small and putting it
// in one file keeps the routing story easy to follow.
//
// Routes use lazy components so each feature gets its own JS chunk
// after `pnpm build`. The shell + design-system bundle stays small.

import { lazy, Suspense, type ComponentType } from 'react';
import {
  createRootRoute,
  createRoute,
  createRouter,
  Outlet,
  lazyRouteComponent,
} from '@tanstack/react-router';

import { AppShell } from '@/components/layout/AppShell';
import { RoleGuard } from '@/features/auth/RoleGuard';
import { Spinner } from '@/components/ui/Spinner';

// guardedLazyComponent loads a route's component lazily and wraps it
// in a client-side RoleGuard. Server still gates access; this just
// trims the "flash of unauthorized chrome + 401 toast" sequence for a
// non-privileged user who deep-links to an admin URL.
//
// Target components must take no props (route components don't).
function guardedLazyComponent(
  loader: () => Promise<{ default: ComponentType<Record<string, never>> }>,
  allow: Array<'admin' | 'manager'>,
): () => JSX.Element {
  const LazyInner = lazy(loader);
  return () => (
    <RoleGuard allow={allow}>
      <Suspense
        fallback={
          <div className="flex items-center gap-2 text-muted">
            <Spinner /> Loading…
          </div>
        }
      >
        <LazyInner />
      </Suspense>
    </RoleGuard>
  );
}

const rootRoute = createRootRoute({
  component: () => (
    <AppShell>
      <Outlet />
    </AppShell>
  ),
});

const dashboardRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/',
  component: lazyRouteComponent(() => import('@/features/dashboard/Dashboard')),
});

const projectsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'projects',
  component: lazyRouteComponent(() => import('@/features/projects/ProjectsList')),
});

const projectDetailRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'projects/$projectId',
  component: lazyRouteComponent(() => import('@/features/projects/ProjectDetail')),
});

const testRunsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'test-runs',
  component: lazyRouteComponent(() => import('@/features/test-runs/TestRunsList')),
});

const testRunDetailRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'test-runs/$runId',
  component: lazyRouteComponent(() => import('@/features/test-runs/TestRunDetail')),
});

const savedViewsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'saved-views',
  component: lazyRouteComponent(() => import('@/features/saved-views/SavedViewsList')),
});

const tagsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'tags',
  component: lazyRouteComponent(() => import('@/features/tags/TagsManagement')),
});

const usersRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'users',
  component: guardedLazyComponent(() => import('@/features/users/Users'), ['admin']),
});

const settingsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'settings',
  component: lazyRouteComponent(() => import('@/features/settings/Settings')),
});

const adminRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'admin',
  component: guardedLazyComponent(() => import('@/features/admin/AdminOverview'), ['admin']),
});

const summariesRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'summaries',
  // Search params are read by TestSummaries on first mount as initial
  // overrides for view-mode and favorites-only. Lets /manager-dashboard
  // deep-link to a preset state without forking the component.
  validateSearch: (search: Record<string, unknown>) => ({
    view: search.view === 'tree' ? 'tree' : search.view === 'card' ? 'card' : undefined,
    favoritesOnly: search.favoritesOnly === 'true' ? 'true' : undefined,
  }),
  component: lazyRouteComponent(() => import('@/features/summaries/TestSummaries')),
});

const managerDashboardRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'manager-dashboard',
  component: lazyRouteComponent(
    () => import('@/features/summaries/ManagerDashboardRedirect'),
  ),
});

const jiraConnectionsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'projects/$projectId/integrations/jira',
  component: lazyRouteComponent(() => import('@/features/jira/JiraConnections')),
});

const profileRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'profile',
  component: lazyRouteComponent(() => import('@/features/profile/Profile')),
});

const projectSettingsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'projects/$projectId/settings',
  component: lazyRouteComponent(() => import('@/features/projects/ProjectSettings')),
});

const projectCoverageRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'projects/$projectId/coverage',
  component: lazyRouteComponent(() => import('@/features/jira/CoverageView')),
});

const routeTree = rootRoute.addChildren([
  dashboardRoute,
  projectsRoute,
  projectDetailRoute,
  testRunsRoute,
  testRunDetailRoute,
  savedViewsRoute,
  tagsRoute,
  usersRoute,
  settingsRoute,
  adminRoute,
  summariesRoute,
  managerDashboardRoute,
  jiraConnectionsRoute,
  profileRoute,
  projectSettingsRoute,
  projectCoverageRoute,
]);

export const router = createRouter({
  routeTree,
  basepath: '/v2',
  defaultPreload: 'intent',
});

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router;
  }
}
