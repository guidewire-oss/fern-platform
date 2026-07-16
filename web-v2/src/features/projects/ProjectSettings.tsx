import { useState, useEffect } from 'react';
import { Link, useParams, useNavigate } from '@tanstack/react-router';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import {
  ArrowLeft,
  Settings as SettingsIcon,
  Plug,
  BarChart3,
  Users,
  Bell,
  Save,
} from 'lucide-react';
import { graphqlFetch } from '@/lib/api';
import { Card, CardBody, CardHeader, CardTitle } from '@/components/ui/Card';
import { Button } from '@/components/ui/Button';
import { Spinner } from '@/components/ui/Spinner';
import { EmptyState } from '@/components/ui/EmptyState';
import { ErrorBoundary } from '@/components/ui/ErrorBoundary';
import { cn } from '@/lib/cn';
import { useUpdateProject, useDeleteProject } from './mutations';
import { JiraConnectionsContent } from '../jira/JiraConnections';
import { FieldMappingVisual } from '../jira/FieldMappingVisual';
import { CoverageContent } from '../jira/CoverageView';

type TabId = 'general' | 'integrations' | 'coverage' | 'team' | 'notifications';

interface Tab {
  id: TabId;
  label: string;
  icon: typeof SettingsIcon;
}

const TABS: Tab[] = [
  { id: 'general',       label: 'General',       icon: SettingsIcon },
  { id: 'integrations',  label: 'Integrations',  icon: Plug },
  { id: 'coverage',      label: 'Coverage',      icon: BarChart3 },
  { id: 'team',          label: 'Team',          icon: Users },
  { id: 'notifications', label: 'Notifications', icon: Bell },
];

// Direct lookup by projectId — no substring search, no 20-cap. The
// resolver returns the exact project (or null) regardless of how many
// other projects with similar IDs exist in the DB.
const GET_PROJECT = /* GraphQL */ `
  query ProjectSettingsLookup($projectId: String!) {
    projectByProjectId(projectId: $projectId) {
      id
      projectId
      name
      description
      repository
      defaultBranch
      team
      isActive
      canManage
    }
  }
`;

interface Project {
  id: string;
  projectId: string;
  name: string;
  description?: string;
  repository?: string;
  defaultBranch?: string;
  team?: string;
  isActive: boolean;
  canManage: boolean;
}

export default function ProjectSettings() {
  const { projectId } = useParams({ from: '/projects/$projectId/settings' });
  const [activeTab, setActiveTab] = useState<TabId>(() => readTabFromHash());

  // Keep state synced with URL hash so deep links to a tab work.
  useEffect(() => {
    const onHashChange = () => setActiveTab(readTabFromHash());
    window.addEventListener('hashchange', onHashChange);
    return () => window.removeEventListener('hashchange', onHashChange);
  }, []);

  const handleTabChange = (id: TabId) => {
    setActiveTab(id);
    window.history.replaceState(null, '', `#${id}`);
  };

  const { data, isLoading, error } = useQuery({
    queryKey: ['project-by-id', projectId],
    queryFn: () =>
      graphqlFetch<{ projectByProjectId: Project | null }>(GET_PROJECT, {
        projectId,
      }),
  });

  if (isLoading) {
    return (
      <div className="flex items-center gap-2 text-muted">
        <Spinner /> Loading {projectId}…
      </div>
    );
  }
  if (error) {
    return <EmptyState title="Couldn't load project" description={(error as Error).message} />;
  }
  const project = data?.projectByProjectId ?? null;
  if (!project) {
    return <EmptyState title={`Project "${projectId}" not found`} />;
  }

  return (
    <div className="space-y-6">
      <Link
        to="/projects/$projectId"
        params={{ projectId }}
        className="inline-flex items-center gap-1 text-sm text-muted hover:text-foreground"
      >
        <ArrowLeft className="h-3 w-3" /> Back to {projectId}
      </Link>

      <header>
        <h1 className="text-3xl font-semibold tracking-tight">{project.name}</h1>
        <p className="mt-1 text-sm text-muted">
          {project.projectId} · {project.team || 'no team'} · settings
        </p>
      </header>

      <div className="grid gap-6 lg:grid-cols-[200px_1fr]">
        <aside className="space-y-0.5">
          {TABS.map(({ id, label, icon: Icon }) => (
            <button
              key={id}
              onClick={() => handleTabChange(id)}
              className={cn(
                'flex w-full items-center gap-2 rounded-md px-3 py-2 text-sm transition-colors',
                activeTab === id
                  ? 'bg-primary/10 font-medium text-primary'
                  : 'text-foreground hover:bg-surface-2',
              )}
            >
              <Icon className="h-4 w-4" />
              {label}
            </button>
          ))}
        </aside>

        <div>
          {/* Each tab is wrapped in its own boundary so one tab
              crashing doesn't take the whole Settings page down. Keyed
              by activeTab so the boundary resets on tab change. */}
          <ErrorBoundary key={activeTab} label={`the ${activeTab} tab`}>
            {activeTab === 'general'       && <GeneralTab project={project} />}
            {activeTab === 'integrations'  && <IntegrationsTab projectId={projectId} canManage={project.canManage} />}
            {activeTab === 'coverage'      && <CoverageContent projectId={projectId} />}
            {activeTab === 'team'          && <TeamTab project={project} />}
            {activeTab === 'notifications' && <NotificationsTab projectId={projectId} />}
          </ErrorBoundary>
        </div>
      </div>
    </div>
  );
}

function readTabFromHash(): TabId {
  const h = window.location.hash.replace(/^#/, '');
  if (h === 'general' || h === 'integrations' || h === 'coverage' || h === 'team' || h === 'notifications') {
    return h;
  }
  return 'general';
}

// ----- General ---------------------------------------------------------------

function GeneralTab({ project }: { project: Project }) {
  const qc = useQueryClient();
  const navigate = useNavigate();
  const update = useUpdateProject();
  const del = useDeleteProject();

  const [name, setName]                 = useState(project.name);
  const [description, setDescription]   = useState(project.description ?? '');
  const [repository, setRepository]     = useState(project.repository ?? '');
  const [defaultBranch, setDefaultBranch] = useState(project.defaultBranch ?? 'main');
  const [team, setTeam]                 = useState(project.team ?? '');
  const [confirmDelete, setConfirmDelete] = useState('');

  const dirty =
    name !== project.name ||
    description !== (project.description ?? '') ||
    repository !== (project.repository ?? '') ||
    defaultBranch !== (project.defaultBranch ?? 'main') ||
    team !== (project.team ?? '');

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader>
          <CardTitle>General</CardTitle>
        </CardHeader>
        <CardBody className="space-y-4">
          <Field label="Project ID" hint="Permanent. Used by client libraries in FERN_PROJECT_ID.">
            <ReadonlyValue value={project.projectId} />
          </Field>
          <Field label="Name" required>
            <Input value={name} onChange={setName} />
          </Field>
          <Field label="Description">
            <Textarea value={description} onChange={setDescription} />
          </Field>
          <div className="grid grid-cols-2 gap-3">
            <Field label="Default branch">
              <Input value={defaultBranch} onChange={setDefaultBranch} />
            </Field>
            <Field label="Team">
              <Input value={team} onChange={setTeam} />
            </Field>
          </div>
          <Field label="Repository URL">
            <Input value={repository} onChange={setRepository} placeholder="https://github.com/org/repo" />
          </Field>

          <div className="flex items-center justify-end gap-2 border-t border-border pt-3">
            {update.error && (
              <span className="text-xs text-red-600 dark:text-red-400">{(update.error as Error).message}</span>
            )}
            {update.isSuccess && !dirty && (
              <span className="text-xs text-status-passed-fg">✓ Saved</span>
            )}
            <Button
              disabled={!dirty || update.isPending || !project.canManage}
              onClick={async () => {
                await update.mutateAsync({
                  id: project.id,
                  input: {
                    name: name.trim(),
                    description: description.trim() || undefined,
                    repository: repository.trim() || undefined,
                    defaultBranch: defaultBranch.trim() || undefined,
                    team: team.trim() || undefined,
                  },
                });
                qc.invalidateQueries({ queryKey: ['project', project.projectId] });
                qc.invalidateQueries({ queryKey: ['projects'] });
              }}
            >
              {update.isPending ? <Spinner className="text-white" /> : <><Save className="h-3.5 w-3.5" /> Save changes</>}
            </Button>
          </div>
        </CardBody>
      </Card>

      {project.canManage && (
        <Card>
          <CardHeader>
            <CardTitle className="text-red-600 dark:text-red-400">Danger zone</CardTitle>
          </CardHeader>
          <CardBody className="space-y-3">
            <p className="text-sm">
              Deleting <strong>{project.name}</strong> removes all test runs, suites, specs,
              tags, and any integrations attached to it. This action cannot be undone.
            </p>
            <Field label={`Type "${project.projectId}" to confirm`}>
              <Input value={confirmDelete} onChange={setConfirmDelete} />
            </Field>
            {/* Case-insensitive trim of leading/trailing whitespace so a
                paste with a stray space doesn't trip up the user. Exact
                projectId comparison still happens on the mutation; this
                is just the click-gate. */}
            {(() => {
              const trimmed = confirmDelete.trim();
              const matches = trimmed.toLowerCase() === project.projectId.toLowerCase();
              const mismatchHint =
                trimmed.length > 0 && !matches
                  ? `Doesn't match "${project.projectId}"`
                  : null;
              return (
                <>
                  {mismatchHint && (
                    <div className="text-[11px] text-status-failed-fg">{mismatchHint}</div>
                  )}
                  <div className="flex justify-end">
                    <Button
                      variant="danger"
                      disabled={!matches || del.isPending}
                      // Navigate back to the list on success: the current
                      // route is this project's own settings page, so without
                      // leaving it the page's project query refetches, finds
                      // the project gone, and collapses into a "not found"
                      // empty state (looks like a refresh). The list path
                      // works because it's already on /projects.
                      onClick={() =>
                        del.mutate(project.id, {
                          onSuccess: () => navigate({ to: '/projects' }),
                        })
                      }
                    >
                      {del.isPending ? <Spinner className="text-white" /> : 'Delete project forever'}
                    </Button>
                  </div>
                </>
              );
            })()}
            {del.error && (
              <div className="rounded border border-red-200 bg-red-50 px-2.5 py-1.5 text-xs text-red-800">
                {(del.error as Error).message}
              </div>
            )}
          </CardBody>
        </Card>
      )}
    </div>
  );
}

// ----- Integrations ----------------------------------------------------------

function IntegrationsTab({ projectId, canManage }: { projectId: string; canManage: boolean }) {
  // Reuse the JIRA connections panel inline, with the project id passed
  // explicitly so it doesn't try to read from a different route. The
  // field-mapping editor sits below it (read-only unless the user can
  // manage the project).
  return (
    <div className="space-y-6">
      <JiraConnectionsContent projectId={projectId} />
      <FieldMappingVisual projectId={projectId} canManage={canManage} />
    </div>
  );
}

// ----- Team ------------------------------------------------------------------

function TeamTab({ project }: { project: Project }) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>Team & permissions</CardTitle>
      </CardHeader>
      <CardBody className="space-y-4">
        <Field label="Owning team" hint="Members of this team see the project in their dashboard.">
          <ReadonlyValue value={project.team || '—'} />
        </Field>
        <Field label="Access" hint="The owning team (above) automatically has access. Add other teams below.">
          <ReadonlyValue value="Access management coming soon" />
        </Field>
        <div className="rounded border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-900">
          <strong>Note:</strong> Granular per-project ACLs (grant team access, revoke, role
          per-team) require the projects-domain authorization endpoints to be wired up.
          Today, the owning team's role on the user account governs access.
        </div>
      </CardBody>
    </Card>
  );
}

// ----- Notifications ---------------------------------------------------------

const PROJECT_NOTIF_KEYS = ['failed-runs', 'flaky', 'slow-run', 'first-failure'] as const;
type ProjectNotifKey = (typeof PROJECT_NOTIF_KEYS)[number];

const PROJECT_NOTIF_LABELS: Record<ProjectNotifKey, [string, string]> = {
  'failed-runs':    ['Failed test runs',  'Notify when a run finishes with failed > 0.'],
  'flaky':          ['Flaky spec detected', 'Notify when a previously-passing spec turns flaky.'],
  'slow-run':       ['Slow run',          'Notify when a run is in the top 5% by duration for this project.'],
  'first-failure':  ['First failure after green', 'Notify when a previously all-green branch breaks.'],
};

function NotificationsTab({ projectId }: { projectId: string }) {
  const storageKey = `fern.v2.project-notifs.${projectId}`;

  const [enabled, setEnabled] = useState<Record<ProjectNotifKey, boolean>>(() => {
    try {
      const v = localStorage.getItem(storageKey);
      if (v) return { ...defaults(), ...JSON.parse(v) };
    } catch {
      // ignore
    }
    return defaults();
  });

  function defaults(): Record<ProjectNotifKey, boolean> {
    return { 'failed-runs': true, flaky: true, 'slow-run': false, 'first-failure': true };
  }

  const toggle = (k: ProjectNotifKey) => {
    const next = { ...enabled, [k]: !enabled[k] };
    setEnabled(next);
    try {
      localStorage.setItem(storageKey, JSON.stringify(next));
    } catch {
      // ignore
    }
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle>Notifications</CardTitle>
      </CardHeader>
      <CardBody className="space-y-3">
        <p className="text-xs text-muted">
          Preferences persist to localStorage per browser. A real delivery backend (Slack /
          email) lands with the Integrations panel.
        </p>
        {PROJECT_NOTIF_KEYS.map((k) => {
          const [label, hint] = PROJECT_NOTIF_LABELS[k];
          return (
            <label
              key={k}
              aria-label={label}
              className="flex cursor-pointer items-center justify-between rounded-md border border-border bg-surface px-3 py-2 hover:bg-surface-2"
            >
              <div>
                <div className="text-sm font-medium">{label}</div>
                <div className="text-[11px] text-muted">{hint}</div>
              </div>
              <input
                type="checkbox"
                checked={enabled[k]}
                onChange={() => toggle(k)}
                className="h-4 w-4 accent-primary"
              />
            </label>
          );
        })}
      </CardBody>
    </Card>
  );
}

// ----- Small form primitives -------------------------------------------------

function Field({
  label,
  required,
  hint,
  children,
}: {
  label: string;
  required?: boolean;
  hint?: string;
  children: React.ReactNode;
}) {
  return (
    <div>
      <label className="block text-xs font-medium text-foreground">
        {label}
        {required && <span className="ml-0.5 text-red-600">*</span>}
      </label>
      <div className="mt-1">{children}</div>
      {hint && <p className="mt-0.5 text-[11px] text-muted">{hint}</p>}
    </div>
  );
}

function Input({
  value,
  onChange,
  placeholder,
}: {
  value: string;
  onChange: (v: string) => void;
  placeholder?: string;
}) {
  return (
    <input
      type="text"
      value={value}
      onChange={(e) => onChange(e.target.value)}
      placeholder={placeholder}
      className="w-full rounded border border-border bg-surface px-2.5 py-1.5 text-sm focus:border-primary focus:outline-none"
    />
  );
}

function Textarea({
  value,
  onChange,
}: {
  value: string;
  onChange: (v: string) => void;
}) {
  return (
    <textarea
      rows={2}
      value={value}
      onChange={(e) => onChange(e.target.value)}
      className="w-full rounded border border-border bg-surface px-2.5 py-1.5 text-sm focus:border-primary focus:outline-none"
    />
  );
}

function ReadonlyValue({ value }: { value: string }) {
  return (
    <div className="rounded border border-border bg-surface-2 px-2.5 py-1.5 font-mono text-xs text-foreground">
      {value}
    </div>
  );
}
