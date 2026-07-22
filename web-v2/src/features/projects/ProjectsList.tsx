import { useMemo, useState } from 'react';
import { Link } from '@tanstack/react-router';
import {
  FolderKanban,
  Users,
  PlayCircle,
  TrendingUp,
  Plus,
  Pencil,
  Trash2,
  Star,
  Settings as SettingsIcon,
} from 'lucide-react';
import { Spinner } from '@/components/ui/Spinner';
import { EmptyState } from '@/components/ui/EmptyState';
import { Button } from '@/components/ui/Button';
import { cn } from '@/lib/cn';
import type { Project } from '@/lib/types';
import { useProjects } from './hooks';
import { ProjectFormModal } from './ProjectFormModal';
import { DeleteProjectModal } from './DeleteProjectModal';
import { useToggleFavorite, useUserPreferences } from '../profile/hooks';
import { useCurrentUser } from '@/features/auth/useCurrentUser';
import {
  ProjectsFilterBar,
  DEFAULT_FILTER as DEFAULT_PROJECTS_FILTER,
  type ProjectsFilter,
} from './ProjectsFilterBar';
import { sortProjects } from './sortProjects';

interface Editing {
  dbId: string;
  projectId: string;
  name: string;
  description?: string;
  team?: string;
  defaultBranch?: string;
}

export default function ProjectsList() {
  const { data, isLoading, isFetchingMore, error } = useProjects();
  const prefs = useUserPreferences();
  // Only admins and managers may create projects (the server enforces
  // this too). Hide the affordance entirely for everyone else so team
  // members aren't shown a button that would only ever 403.
  const { data: currentUser } = useCurrentUser();
  const currentRole = (currentUser?.role ?? '').toLowerCase();
  const canCreate = currentRole === 'admin' || currentRole === 'manager';
  const [creating, setCreating] = useState(false);
  const [editing, setEditing] = useState<Editing | null>(null);
  const [deleting, setDeleting] = useState<{ dbId: string; projectId: string; name: string } | null>(null);
  const [filter, setFilter] = useState<ProjectsFilter>(DEFAULT_PROJECTS_FILTER);

  // Adapt the flat Project[] to the {node: Project} shape the grid and
  // filter logic below already use. Keeps the existing render code
  // unchanged; the pagination change is contained to the hook.
  const edges = useMemo(
    () => (data?.projects ?? []).map((p) => ({ node: p })),
    [data?.projects],
  );
  const truncated = data?.truncated ?? false;
  const totalCount = data?.totalCount ?? edges.length;
  const favoriteSet = useMemo(
    () => new Set(prefs.data?.userPreferences?.favorites ?? []),
    [prefs.data],
  );
  const availableTeams = useMemo(
    () =>
      Array.from(
        new Set(edges.map((e) => e.node.team).filter((t): t is string => !!t)),
      ).sort(),
    [edges],
  );

  const filteredEdges = useMemo(() => {
    const q = filter.q.trim().toLowerCase();
    const teamSet = new Set(filter.teams);
    const catSet = new Set(filter.categories);

    const out = edges.filter(({ node }) => {
      if (filter.favoritesOnly && !favoriteSet.has(node.projectId)) return false;
      if (teamSet.size > 0 && !(node.team && teamSet.has(node.team))) return false;
      if (catSet.size > 0) {
        const matches = Array.from(catSet).some((c) =>
          node.projectId.toLowerCase().startsWith(c.toLowerCase() + '-') ||
          node.projectId.toLowerCase().includes('-' + c.toLowerCase() + '-') ||
          node.projectId.toLowerCase().startsWith(c.toLowerCase()),
        );
        if (!matches) return false;
      }
      if (q) {
        const hay = `${node.name} ${node.projectId} ${node.team ?? ''}`.toLowerCase();
        if (!hay.includes(q)) return false;
      }
      return true;
    });

    return sortProjects(out.map((e) => e.node), filter.sortKey, filter.sortDir).map((node) => ({ node }));
  }, [edges, filter, favoriteSet]);

  if (isLoading) {
    return (
      <div className="flex items-center gap-2 text-muted">
        <Spinner /> Loading projects…
      </div>
    );
  }
  if (error) {
    return <EmptyState title="Couldn't load projects" description={(error as Error).message} />;
  }

  return (
    <div className="space-y-6">
      <header className="flex items-start justify-between gap-4">
        <div>
          <h1 className="text-3xl font-semibold tracking-tight">Projects</h1>
          <p className="mt-1 text-sm text-muted">
            {filteredEdges.length === edges.length
              ? `${totalCount} projects`
              : `${filteredEdges.length} of ${edges.length} shown`}
            {isFetchingMore && ' · loading more…'}
            {' · click a card to drill into recent runs'}
          </p>
        </div>
        {canCreate && (
          <Button onClick={() => setCreating(true)}>
            <Plus className="h-3.5 w-3.5" /> New project
          </Button>
        )}
      </header>

      {truncated && (
        <div className="rounded-md border border-amber-300 bg-amber-50 px-3 py-2 text-xs text-amber-900 dark:border-amber-700 dark:bg-amber-950/40 dark:text-amber-200">
          Showing the first {edges.length.toLocaleString()} of{' '}
          {totalCount.toLocaleString()} projects. Use the search /
          team / favorites filters above to narrow down — the rest can
          still be reached by filtering.
        </div>
      )}

      {edges.length === 0 ? (
        <EmptyState
          title="No projects yet"
          description={
            canCreate
              ? 'Create one above, or seed sample data with `make docker-test-seed`.'
              : 'No projects are available to your team yet.'
          }
          action={
            canCreate ? (
              <Button onClick={() => setCreating(true)}>
                <Plus className="h-3.5 w-3.5" /> Create your first project
              </Button>
            ) : undefined
          }
        />
      ) : (
        <>
          <ProjectsFilterBar
            filter={filter}
            onChange={setFilter}
            availableTeams={availableTeams}
            favoritesCount={favoriteSet.size}
            visibleCount={filteredEdges.length}
            totalCount={edges.length}
          />
          {filteredEdges.length === 0 ? (
            <EmptyState
              title="No projects match"
              description="Try clearing some filters."
              action={
                <Button variant="ghost" onClick={() => setFilter(DEFAULT_PROJECTS_FILTER)}>
                  Reset filters
                </Button>
              }
            />
          ) : (
            <ProjectsGrid
              edges={filteredEdges}
              onEdit={setEditing}
              onDelete={setDeleting}
            />
          )}
        </>
      )}

      <ProjectFormModal open={creating} onClose={() => setCreating(false)} />
      <ProjectFormModal
        open={!!editing}
        onClose={() => setEditing(null)}
        initial={editing ?? undefined}
      />
      <DeleteProjectModal
        open={!!deleting}
        onClose={() => setDeleting(null)}
        project={deleting ?? undefined}
      />
    </div>
  );
}

function ProjectsGrid({
  edges,
  onEdit,
  onDelete,
}: {
  edges: { node: Project }[];
  onEdit: (e: Editing) => void;
  onDelete: (e: { dbId: string; projectId: string; name: string }) => void;
}) {
  const prefs = useUserPreferences();
  const toggle = useToggleFavorite();
  const favorites = new Set(prefs.data?.userPreferences?.favorites ?? []);
  // Admin / manager always sees the management affordances regardless
  // of per-row `canManage`. The resolver short-circuits admin/manager
  // to true server-side, but a stale cache or a per-row dataloader
  // hiccup can return `false`/`null` for some rows — hiding the gear
  // inconsistently. Trust the client-side role here as a UI guard;
  // the server still gates the actual mutations.
  const { data: user } = useCurrentUser();
  const role = (user?.role ?? '').toLowerCase();
  const isPrivileged = role === 'admin' || role === 'manager';

  return (
    <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
      {edges.map(({ node }) => {
        const successPct =
          node.stats?.successRate != null
            ? Math.round(node.stats.successRate * 100)
            : null;
        const isFav = favorites.has(node.projectId);
        const canManage = isPrivileged || node.canManage;
        return (
          <ProjectCard
            key={node.id}
            node={{ ...node, canManage }}
            successPct={successPct}
            isFav={isFav}
            onToggleFav={() => toggle.mutate(node.projectId)}
            onEdit={() =>
              onEdit({
                dbId: node.id,
                projectId: node.projectId,
                name: node.name,
                team: node.team,
              })
            }
            onDelete={() =>
              onDelete({ dbId: node.id, projectId: node.projectId, name: node.name })
            }
          />
        );
      })}
    </div>
  );
}

function ProjectCard({
  node,
  successPct,
  isFav,
  onToggleFav,
  onEdit,
  onDelete,
}: {
  node: Project;
  successPct: number | null;
  isFav: boolean;
  onToggleFav: () => void;
  onEdit: () => void;
  onDelete: () => void;
}) {
  return (
    <div className="fern-card fern-pop relative p-5">
      <div className="flex items-start justify-between">
        <Link
          to="/projects/$projectId"
          params={{ projectId: node.projectId }}
          className="flex h-10 w-10 items-center justify-center rounded-lg bg-gradient-primary text-white shadow-md hover:opacity-90"
        >
          <FolderKanban className="h-5 w-5" />
        </Link>
        <div className="flex items-center gap-1.5">
          {successPct != null && (
            <span
              className={
                successPct >= 90
                  ? 'rounded-full bg-status-passed-bg px-2 py-0.5 text-[11px] font-medium text-status-passed-fg'
                  : successPct >= 70
                  ? 'rounded-full bg-status-flaky-bg px-2 py-0.5 text-[11px] font-medium text-status-flaky-fg'
                  : 'rounded-full bg-status-failed-bg px-2 py-0.5 text-[11px] font-medium text-status-failed-fg'
              }
            >
              {successPct}% pass
            </span>
          )}
          <button
            onClick={onToggleFav}
            aria-label={isFav ? `Unstar ${node.name}` : `Star ${node.name}`}
            title={isFav ? 'Remove from favorites' : 'Add to favorites'}
            className={cn(
              'rounded p-1 transition-colors',
              isFav
                ? 'text-amber-500 hover:text-amber-600'
                : 'text-muted hover:bg-surface-2 hover:text-amber-500',
            )}
          >
            <Star className={cn('h-4 w-4', isFav && 'fill-current')} />
          </button>
        </div>
      </div>

      <Link
        to="/projects/$projectId"
        params={{ projectId: node.projectId }}
        className="block"
      >
        <h3 className="mt-3 text-base font-semibold leading-tight hover:text-primary">
          {node.name}
        </h3>
        <div className="mt-0.5 text-[11px] text-muted">{node.projectId}</div>

        <div className="mt-4 flex items-center gap-4 text-xs text-muted">
          <span className="inline-flex items-center gap-1">
            <Users className="h-3 w-3" />
            {node.team || 'no team'}
          </span>
          <span className="inline-flex items-center gap-1">
            <PlayCircle className="h-3 w-3" />
            {node.stats?.totalTestRuns ?? 0}
          </span>
          {successPct != null && (
            <span className="inline-flex items-center gap-1">
              <TrendingUp className="h-3 w-3" />
              {successPct}%
            </span>
          )}
        </div>
      </Link>

      <div className="mt-3 flex items-center justify-between border-t border-border pt-3 text-[11px] text-muted">
        <span>
          Last run:{' '}
          {node.stats?.lastRunTime
            ? new Date(node.stats.lastRunTime).toLocaleString()
            : '—'}
        </span>
        {node.canManage && (
          <div className="flex items-center gap-1">
            <Link
              to="/projects/$projectId/settings"
              params={{ projectId: node.projectId }}
              aria-label={`Settings for ${node.name}`}
              title="Settings (General / Integrations / Team / Notifications)"
              className="rounded p-1 text-muted hover:bg-surface-2 hover:text-foreground"
              onClick={(e) => e.stopPropagation()}
            >
              <SettingsIcon className="h-3 w-3" />
            </Link>
            <button
              onClick={onEdit}
              aria-label={`Quick edit ${node.name}`}
              title="Quick edit"
              className="rounded p-1 text-muted hover:bg-surface-2 hover:text-foreground"
            >
              <Pencil className="h-3 w-3" />
            </button>
            <button
              onClick={onDelete}
              aria-label={`Delete ${node.name}`}
              title="Delete project"
              className="rounded p-1 text-muted hover:bg-red-500/10 hover:text-red-600 dark:hover:text-red-400"
            >
              <Trash2 className="h-3 w-3" />
            </button>
          </div>
        )}
      </div>
    </div>
  );
}
