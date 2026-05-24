import { useEffect, useState } from 'react';
import { Modal } from '@/components/ui/Modal';
import { Button } from '@/components/ui/Button';
import { Spinner } from '@/components/ui/Spinner';
import { useCreateProject, useUpdateProject } from './mutations';

interface ProjectFields {
  projectId: string;
  name: string;
  description: string;
  repository: string;
  defaultBranch: string;
  team: string;
}

const EMPTY: ProjectFields = {
  projectId: '',
  name: '',
  description: '',
  repository: '',
  defaultBranch: 'main',
  team: '',
};

interface Props {
  open: boolean;
  onClose: () => void;
  initial?: (Partial<ProjectFields> & { dbId?: string }) | undefined;
}

export function ProjectFormModal({ open, onClose, initial }: Props) {
  const editing = !!initial?.dbId;
  const [fields, setFields] = useState<ProjectFields>({ ...EMPTY, ...initial });
  const create = useCreateProject();
  const update = useUpdateProject();
  const busy = create.isPending || update.isPending;
  const err = (create.error ?? update.error) as Error | null;

  useEffect(() => {
    if (open) setFields({ ...EMPTY, ...initial });
  }, [open, initial]);

  const set = <K extends keyof ProjectFields>(k: K, v: ProjectFields[K]) =>
    setFields((f) => ({ ...f, [k]: v }));

  const valid =
    fields.name.trim().length > 0 && (editing || fields.projectId.trim().length > 0);

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
        projectId: fields.projectId.trim(),
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
      description={editing ? `Project ID: ${initial?.projectId}` : 'Project ID is permanent and used in client libraries.'}
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
        {!editing && (
          <Field label="Project ID" required hint="Used in FERN_PROJECT_ID by clients. Lowercase, hyphens.">
            <input
              type="text"
              value={fields.projectId}
              onChange={(e) => set('projectId', e.target.value)}
              placeholder="my-service"
              className="w-full rounded border border-border bg-surface px-2.5 py-1.5 text-sm focus:border-primary focus:outline-none"
            />
          </Field>
        )}
        <Field label="Name" required>
          <input
            type="text"
            value={fields.name}
            onChange={(e) => set('name', e.target.value)}
            placeholder="My Service"
            className="w-full rounded border border-border bg-surface px-2.5 py-1.5 text-sm focus:border-primary focus:outline-none"
          />
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
            <input
              type="text"
              value={fields.team}
              onChange={(e) => set('team', e.target.value)}
              placeholder="team-a"
              className="w-full rounded border border-border bg-surface px-2.5 py-1.5 text-sm focus:border-primary focus:outline-none"
            />
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
