import { useState } from 'react';
import { Modal } from '@/components/ui/Modal';
import { Button } from '@/components/ui/Button';
import { Spinner } from '@/components/ui/Spinner';
import { useDeleteProject } from './mutations';

interface Props {
  open: boolean;
  onClose: () => void;
  project?: { dbId: string; projectId: string; name: string } | undefined;
}

export function DeleteProjectModal({ open, onClose, project }: Props) {
  const [typed, setTyped] = useState('');
  const del = useDeleteProject();

  if (!project) return null;
  const confirmed = typed === project.projectId;

  return (
    <Modal
      open={open}
      onClose={() => {
        setTyped('');
        onClose();
      }}
      title="Delete project"
      description="This removes the project AND every test run, suite, spec, and tag associated with it."
      footer={
        <>
          <Button variant="secondary" onClick={() => { setTyped(''); onClose(); }} disabled={del.isPending}>
            Cancel
          </Button>
          <Button
            variant="danger"
            disabled={!confirmed || del.isPending}
            onClick={async () => {
              await del.mutateAsync(project.dbId);
              setTyped('');
              onClose();
            }}
          >
            {del.isPending ? <Spinner className="text-white" /> : 'Delete forever'}
          </Button>
        </>
      }
    >
      <p className="text-sm text-foreground">
        You're about to delete <strong>{project.name}</strong>{' '}
        (<code className="font-mono text-xs">{project.projectId}</code>).
      </p>
      <p className="mt-2 text-xs text-muted">
        Type the project ID to confirm:
      </p>
      <input
        type="text"
        value={typed}
        onChange={(e) => setTyped(e.target.value)}
        placeholder={project.projectId}
        // Autofocus is correct UX inside a destructive-confirm modal —
        // the dialog has just opened on the user's request and the only
        // sensible next action is to type the project id.
        // eslint-disable-next-line jsx-a11y/no-autofocus
        autoFocus
        className="mt-1 w-full rounded border border-border bg-surface px-2.5 py-1.5 font-mono text-sm focus:border-red-500 focus:outline-none"
      />
      {del.error && (
        <div className="mt-2 rounded border border-red-200 bg-red-50 px-2.5 py-1.5 text-xs text-red-800">
          {(del.error as Error).message}
        </div>
      )}
    </Modal>
  );
}
