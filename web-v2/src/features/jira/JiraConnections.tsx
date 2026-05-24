import { useState } from 'react';
import { useParams, Link } from '@tanstack/react-router';
import { ArrowLeft, Plug, Plus, Trash2, Zap } from 'lucide-react';
import { Card } from '@/components/ui/Card';
import { Button } from '@/components/ui/Button';
import { Modal } from '@/components/ui/Modal';
import { Spinner } from '@/components/ui/Spinner';
import { EmptyState } from '@/components/ui/EmptyState';
import { cn } from '@/lib/cn';
import {
  useCreateJiraConnection,
  useDeleteJiraConnection,
  useJiraConnections,
  useTestJiraConnection,
  type JiraConnection,
  type NewJiraConnection,
} from './hooks';

const STATUS_TINT: Record<JiraConnection['status'], string> = {
  pending:   'bg-status-skipped-bg text-status-skipped-fg',
  connected: 'bg-status-passed-bg  text-status-passed-fg',
  failed:    'bg-status-failed-bg  text-status-failed-fg',
};

// Standalone page route — reads projectId from /projects/:projectId/integrations/jira.
export default function JiraConnectionsPage() {
  const { projectId } = useParams({ from: '/projects/$projectId/integrations/jira' });
  return (
    <div className="space-y-4">
      <Link
        to="/projects/$projectId"
        params={{ projectId }}
        className="inline-flex items-center gap-1 text-sm text-muted hover:text-foreground"
      >
        <ArrowLeft className="h-3 w-3" /> Back to {projectId}
      </Link>
      <JiraConnectionsContent projectId={projectId} />
    </div>
  );
}

// Embeddable variant — accepts projectId as a prop so it works inside
// the project-settings tab UI where the route param key is different.
export function JiraConnectionsContent({ projectId }: { projectId: string }) {
  const list = useJiraConnections(projectId);
  const del = useDeleteJiraConnection(projectId);
  const test = useTestJiraConnection(projectId);
  const [showCreate, setShowCreate] = useState(false);
  const [testResult, setTestResult] = useState<Record<string, { success: boolean; message?: string }>>({});

  const conns = list.data?.connections ?? [];

  const handleTest = async (id: string) => {
    try {
      const r = await test.mutateAsync(id);
      setTestResult((m) => ({ ...m, [id]: r }));
    } catch (e) {
      setTestResult((m) => ({ ...m, [id]: { success: false, message: (e as Error).message } }));
    }
  };

  return (
    <div className="space-y-4">
      <header className="flex items-start justify-between gap-4">
        <div>
          <h1 className="text-xl font-semibold tracking-tight">JIRA integration</h1>
          <p className="mt-1 text-sm text-muted">
            Link failed tests to JIRA tickets. {conns.length} connection{conns.length === 1 ? '' : 's'} configured.
          </p>
        </div>
        <Button onClick={() => setShowCreate(true)}>
          <Plus className="h-3.5 w-3.5" /> Add connection
        </Button>
      </header>

      {list.isLoading && (
        <div className="flex items-center gap-2 text-muted"><Spinner /> Loading…</div>
      )}
      {list.error && (
        <EmptyState title="Couldn't load connections" description={(list.error as Error).message} />
      )}
      {!list.isLoading && conns.length === 0 && (
        <EmptyState
          title="No JIRA connections yet"
          description="Add one to start linking failed tests."
          action={
            <Button onClick={() => setShowCreate(true)}>
              <Plus className="h-3.5 w-3.5" /> Add your first
            </Button>
          }
        />
      )}

      <div className="grid gap-3 lg:grid-cols-2">
        {conns.map((c) => {
          const t = testResult[c.id];
          return (
            <Card key={c.id} className="p-4">
              <div className="flex items-start justify-between gap-3">
                <div className="min-w-0">
                  <div className="flex items-center gap-2">
                    <Plug className="h-4 w-4 text-primary" />
                    <h3 className="truncate font-semibold">{c.name}</h3>
                    <span
                      className={cn(
                        'rounded-full px-2 py-0.5 text-[10px] font-medium uppercase tracking-wider',
                        STATUS_TINT[c.status],
                      )}
                    >
                      {c.status}
                    </span>
                  </div>
                  <div className="mt-1 truncate text-xs text-muted">{c.jiraUrl}</div>
                </div>
                <div className="flex items-center gap-1">
                  <Button
                    size="sm"
                    variant="secondary"
                    onClick={() => handleTest(c.id)}
                    disabled={test.isPending}
                  >
                    <Zap className="h-3 w-3" /> Test
                  </Button>
                  <Button
                    size="sm"
                    variant="ghost"
                    aria-label="Delete connection"
                    onClick={() => {
                      if (confirm(`Delete connection "${c.name}"?`)) del.mutate(c.id);
                    }}
                  >
                    <Trash2 className="h-3 w-3 text-red-600" />
                  </Button>
                </div>
              </div>

              <dl className="mt-3 grid grid-cols-2 gap-x-3 gap-y-1 text-xs">
                <dt className="text-muted">Project key</dt>
                <dd className="font-mono text-foreground">{c.projectKey}</dd>
                <dt className="text-muted">Auth</dt>
                <dd className="font-mono text-foreground">{c.authenticationType}</dd>
                <dt className="text-muted">User</dt>
                <dd className="truncate text-foreground" title={c.username}>{c.username}</dd>
                {c.lastTestedAt && (
                  <>
                    <dt className="text-muted">Last tested</dt>
                    <dd className="text-foreground">{new Date(c.lastTestedAt).toLocaleString()}</dd>
                  </>
                )}
              </dl>

              {t && (
                <div
                  className={cn(
                    'mt-3 rounded border px-2 py-1.5 text-xs',
                    t.success
                      ? 'border-emerald-200 bg-emerald-50 text-emerald-800'
                      : 'border-red-200 bg-red-50 text-red-800',
                  )}
                >
                  {t.success ? '✓ Connection OK' : `✗ ${t.message ?? 'Failed'}`}
                </div>
              )}
            </Card>
          );
        })}
      </div>

      <CreateModal
        open={showCreate}
        onClose={() => setShowCreate(false)}
        projectId={projectId}
      />
    </div>
  );
}

function CreateModal({
  open,
  onClose,
  projectId,
}: {
  open: boolean;
  onClose: () => void;
  projectId: string;
}) {
  const [fields, setFields] = useState<NewJiraConnection>({
    name: '',
    jiraUrl: '',
    authenticationType: 'api_token',
    projectKey: '',
    username: '',
    credential: '',
  });
  const create = useCreateJiraConnection(projectId);
  const set = <K extends keyof NewJiraConnection>(k: K, v: NewJiraConnection[K]) =>
    setFields((f) => ({ ...f, [k]: v }));
  const valid =
    fields.name.trim() &&
    fields.jiraUrl.trim() &&
    fields.projectKey.trim() &&
    fields.username.trim() &&
    fields.credential.trim();

  return (
    <Modal
      open={open}
      onClose={() => {
        setFields({ name: '', jiraUrl: '', authenticationType: 'api_token', projectKey: '', username: '', credential: '' });
        onClose();
      }}
      title="New JIRA connection"
      description="Credentials are stored encrypted at rest."
      size="lg"
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={create.isPending}>Cancel</Button>
          <Button
            disabled={!valid || create.isPending}
            onClick={async () => {
              await create.mutateAsync(fields);
              setFields({ name: '', jiraUrl: '', authenticationType: 'api_token', projectKey: '', username: '', credential: '' });
              onClose();
            }}
          >
            {create.isPending ? <Spinner className="text-white" /> : 'Create connection'}
          </Button>
        </>
      }
    >
      <div className="space-y-3">
        <Row>
          <Field label="Name" required>
            <Input value={fields.name} onChange={(v) => set('name', v)} placeholder="Production JIRA" />
          </Field>
          <Field label="Project key" required hint="JIRA project key (e.g. FERN).">
            <Input value={fields.projectKey} onChange={(v) => set('projectKey', v)} placeholder="FERN" />
          </Field>
        </Row>
        <Field label="JIRA URL" required>
          <Input
            type="url"
            value={fields.jiraUrl}
            onChange={(v) => set('jiraUrl', v)}
            placeholder="https://your-domain.atlassian.net"
          />
        </Field>
        <Row>
          <Field label="Auth method" required>
            <select
              value={fields.authenticationType}
              onChange={(e) => set('authenticationType', e.target.value as NewJiraConnection['authenticationType'])}
              className="w-full rounded border border-border bg-surface px-2.5 py-1.5 text-sm focus:border-primary focus:outline-none"
            >
              <option value="api_token">API token</option>
              <option value="personal_access_token">Personal access token</option>
              <option value="oauth">OAuth</option>
            </select>
          </Field>
          <Field label="Username / email" required>
            <Input value={fields.username} onChange={(v) => set('username', v)} placeholder="you@example.com" />
          </Field>
        </Row>
        <Field label="Token / credential" required hint="Encrypted before storage.">
          <Input
            type="password"
            value={fields.credential}
            onChange={(v) => set('credential', v)}
            placeholder="••••••••"
          />
        </Field>
        {create.error && (
          <div className="rounded border border-red-200 bg-red-50 px-2.5 py-1.5 text-xs text-red-800">
            {(create.error as Error).message}
          </div>
        )}
      </div>
    </Modal>
  );
}

function Row({ children }: { children: React.ReactNode }) {
  return <div className="grid grid-cols-2 gap-3">{children}</div>;
}

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
  type = 'text',
  placeholder,
}: {
  value: string;
  onChange: (v: string) => void;
  type?: string;
  placeholder?: string;
}) {
  return (
    <input
      type={type}
      value={value}
      onChange={(e) => onChange(e.target.value)}
      placeholder={placeholder}
      className="w-full rounded border border-border bg-surface px-2.5 py-1.5 text-sm focus:border-primary focus:outline-none"
    />
  );
}
