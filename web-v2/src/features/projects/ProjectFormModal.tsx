import { useEffect, useState } from 'react';
import { Modal } from '@/components/ui/Modal';
import { Button } from '@/components/ui/Button';
import { Spinner } from '@/components/ui/Spinner';
import { useCurrentUser } from '@/features/auth/useCurrentUser';
import { useRoleGroups, deriveUserTeams, isAdminUser, DEFAULT_ROLE_GROUPS } from '@/features/auth/roleGroups';
import { useProjects } from './hooks';
import { useCreateProject, useUpdateProject } from './mutations';

interface ProjectFields {
  name: string;
  description: string;
  repository: string;
  defaultBranch: string;
  team: string;
}

const EMPTY: ProjectFields = {
  name: '',
  description: '',
  repository: '',
  defaultBranch: 'main',
  team: '',
};

interface Props {
  open: boolean;
  onClose: () => void;
  initial?: (Partial<ProjectFields> & { dbId?: string; projectId?: string }) | undefined;
}

export function ProjectFormModal({ open, onClose, initial }: Props) {
  const editing = !!initial?.dbId;
  const [fields, setFields] = useState<ProjectFields>({ ...EMPTY, ...initial });
  const create = useCreateProject();
  const update = useUpdateProject();
  const busy = create.isPending || update.isPending;
  const err = (create.error ?? update.error) as Error | null;

  // Team is chosen from the user's teams (groups minus role groups) — same
  // as v1. Project ID is auto-generated server-side, so it is not collected.
  const { data: user } = useCurrentUser();
  const { data: roleGroupsData } = useRoleGroups();
  const roleGroups = roleGroupsData ?? DEFAULT_ROLE_GROUPS;
  const userTeams = deriveUserTeams(user?.groups, roleGroups);
  const admin = isAdminUser(user, roleGroups);

  // Keep the currently-selected team visible even if it is not one of the
  // current user's teams (e.g. an admin editing another team's project).
  const teamOptions =
    fields.team && !userTeams.includes(fields.team) ? [fields.team, ...userTeams] : userTeams;

  useEffect(() => {
    if (open) setFields({ ...EMPTY, ...initial });
  }, [open, initial]);

  // Default the team once options are known (create only): non-admins get
  // the first team, admins default to "No team" (empty), mirroring v1.
  useEffect(() => {
    if (!open || editing) return;
    setFields((f) => (f.team ? f : { ...f, team: admin ? '' : userTeams[0] ?? '' }));
    // userTeams is recomputed each render; key the effect on its contents.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, editing, admin, userTeams.join('|')]);

  const set = <K extends keyof ProjectFields>(k: K, v: ProjectFields[K]) =>
    setFields((f) => ({ ...f, [k]: v }));

  // v2 guard against duplicate project names (case-insensitive). Checked
  // against the full project list; on edit the project itself is excluded
  // so keeping its own name is allowed. This is a client-side UX guard —
  // the backend does not (yet) enforce name uniqueness.
  const { data: projectsData } = useProjects();
  const trimmedName = fields.name.trim();
  const duplicateName =
    trimmedName.length > 0 &&
    (projectsData?.projects ?? []).some(
      (p) =>
        p.projectId !== initial?.projectId &&
        p.name.trim().toLowerCase() === trimmedName.toLowerCase(),
    );

  const valid = trimmedName.length > 0 && !duplicateName;

  const submit = async () => {
    if (editing && initial?.dbId) {
      await update.mutateAsync({
        id: initial.dbId,
        input: {
          name: fields.name.trim(),
          description: fields.description.trim() || undefined,
          repository: fields.repository.trim() || undefined,
          defaultBranch: fields.defaultBranch.trim() || undefined,
          team: fields.team.trim() || undefined,
        },
      });
    } else {
      await create.mutateAsync({
        // Empty projectId → backend auto-generates a UUID (v1 behavior).
        projectId: '',
        name: fields.name.trim(),
        description: fields.description.trim() || undefined,
        repository: fields.repository.trim() || undefined,
        defaultBranch: fields.defaultBranch.trim() || undefined,
        team: fields.team.trim() || undefined,
      });
    }
    onClose();
  };

  return (
    <Modal
      open={open}
      onClose={onClose}
      title={editing ? 'Edit project' : 'Create project'}
      description={editing ? `Project ID: ${initial?.projectId}` : 'A Project ID is generated automatically.'}
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={busy}>Cancel</Button>
          <Button onClick={submit} disabled={!valid || busy}>
            {busy ? <Spinner className="text-white" /> : editing ? 'Save changes' : 'Create project'}
          </Button>
        </>
      }
    >
      <div className="space-y-3">
        <Field label="Name" required>
          <input
            type="text"
            value={fields.name}
            onChange={(e) => set('name', e.target.value)}
            placeholder="My Service"
            className="w-full rounded border border-border bg-surface px-2.5 py-1.5 text-sm focus:border-primary focus:outline-none"
          />
          {duplicateName && (
            <p className="mt-0.5 text-[11px] text-red-600">
              A project named “{trimmedName}” already exists.
            </p>
          )}
        </Field>
        <Field label="Description">
          <textarea
            rows={2}
            value={fields.description}
            onChange={(e) => set('description', e.target.value)}
            className="w-full rounded border border-border bg-surface px-2.5 py-1.5 text-sm focus:border-primary focus:outline-none"
          />
        </Field>
        <div className="grid grid-cols-2 gap-3">
          <Field label="Default branch">
            <input
              type="text"
              value={fields.defaultBranch}
              onChange={(e) => set('defaultBranch', e.target.value)}
              className="w-full rounded border border-border bg-surface px-2.5 py-1.5 text-sm focus:border-primary focus:outline-none"
            />
          </Field>
          <Field label="Team">
            <select
              value={fields.team}
              onChange={(e) => set('team', e.target.value)}
              className="w-full rounded border border-border bg-surface px-2.5 py-1.5 text-sm focus:border-primary focus:outline-none"
            >
              {admin && <option value="">No team</option>}
              {teamOptions.map((t) => (
                <option key={t} value={t}>
                  {t}
                </option>
              ))}
            </select>
          </Field>
        </div>
        <Field label="Repository URL">
          <input
            type="url"
            value={fields.repository}
            onChange={(e) => set('repository', e.target.value)}
            placeholder="https://github.com/org/repo"
            className="w-full rounded border border-border bg-surface px-2.5 py-1.5 text-sm focus:border-primary focus:outline-none"
          />
        </Field>
        {err && (
          <div className="rounded border border-red-200 bg-red-50 px-2.5 py-1.5 text-xs text-red-800">
            {err.message}
          </div>
        )}
      </div>
    </Modal>
  );
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
