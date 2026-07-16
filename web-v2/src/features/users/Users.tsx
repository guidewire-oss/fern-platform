import { useEffect, useState } from 'react';
import { Search, MoreVertical, ShieldCheck, User as UserIcon } from 'lucide-react';
import { Card } from '@/components/ui/Card';
import { Button } from '@/components/ui/Button';
import { Spinner } from '@/components/ui/Spinner';
import { EmptyState } from '@/components/ui/EmptyState';
import { Modal } from '@/components/ui/Modal';
import { cn } from '@/lib/cn';
import { useUpdateUserRole, useUsers, type AdminUser } from './hooks';

const ROLE_TINT: Record<string, string> = {
  admin:   'bg-gradient-primary text-white',
  manager: 'bg-indigo-100 text-indigo-800',
  user:    'bg-slate-100 text-slate-800',
};

export default function Users() {
  const [q, setQ] = useState('');
  const [editing, setEditing] = useState<AdminUser | null>(null);
  const { data, isLoading, error } = useUsers({ limit: 200 });

  const items = data?.items ?? [];
  const filtered = q
    ? items.filter((u) =>
        [u.name, u.email, u.userId, u.role].some((s) =>
          (s ?? '').toLowerCase().includes(q.toLowerCase()),
        ),
      )
    : items;

  return (
    <div className="space-y-6">
      <header className="flex items-start justify-between gap-4">
        <div>
          <h1 className="text-3xl font-semibold tracking-tight">Users</h1>
          <p className="mt-1 text-sm text-muted">
            {data ? `${data.total} users` : '—'} · roles control which projects and admin actions are visible
          </p>
        </div>
        <div className="relative">
          <Search className="absolute left-2 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-muted" />
          <input
            type="search"
            value={q}
            onChange={(e) => setQ(e.target.value)}
            placeholder="Search users…"
            className="w-64 rounded-md border border-border bg-surface py-1.5 pl-7 pr-2 text-sm focus:border-primary focus:outline-none"
          />
        </div>
      </header>

      {isLoading && (
        <div className="flex items-center gap-2 text-muted">
          <Spinner /> Loading users…
        </div>
      )}
      {error && (
        <EmptyState title="Couldn't load users" description={(error as Error).message} />
      )}
      {!isLoading && !error && items.length === 0 && (
        <EmptyState
          title="No users yet"
          description="When real users authenticate via OAuth they appear here. With AUTH_ENABLED=false you only see the synthetic dev-admin once they've signed in at least once."
        />
      )}

      {filtered.length > 0 && (
        <Card>
          <table className="w-full text-sm">
            <thead className="bg-surface-2 text-left">
              <tr>
                {['User', 'Role', 'Status', 'Last login', ''].map((h) => (
                  <th
                    key={h}
                    className="px-4 py-2.5 text-xs font-medium uppercase tracking-wider text-muted"
                  >
                    {h}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {filtered.map((u, i) => (
                <tr
                  key={u.userId}
                  className={cn(i % 2 ? 'bg-surface-2/30' : '', 'border-t border-border')}
                >
                  <td className="px-4 py-3">
                    <div className="flex items-center gap-3">
                      <div className="flex h-8 w-8 items-center justify-center rounded-full bg-gradient-primary text-xs font-bold text-white">
                        {(u.name || u.email).split(/\s|@/).filter(Boolean).map((p) => p[0]).join('').slice(0, 2).toUpperCase()}
                      </div>
                      <div>
                        <div className="font-medium text-foreground">{u.name || u.email}</div>
                        <div className="text-xs text-muted">{u.email}</div>
                      </div>
                    </div>
                  </td>
                  <td className="px-4 py-3">
                    <span
                      className={cn(
                        'inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[11px] font-medium',
                        ROLE_TINT[u.role] ?? ROLE_TINT.user,
                      )}
                    >
                      {u.role === 'admin' ? (
                        <ShieldCheck className="h-3 w-3" />
                      ) : (
                        <UserIcon className="h-3 w-3" />
                      )}
                      {u.role}
                    </span>
                  </td>
                  <td className="px-4 py-3 text-xs">
                    {u.status === 'active' ? (
                      <span className="inline-flex items-center gap-1.5 text-status-passed-fg">
                        <span className="h-1.5 w-1.5 rounded-full bg-emerald-500" />
                        Active
                      </span>
                    ) : (
                      <span className="inline-flex items-center gap-1.5 text-muted">
                        <span className="h-1.5 w-1.5 rounded-full bg-slate-400" />
                        {u.status || 'Inactive'}
                      </span>
                    )}
                  </td>
                  <td className="px-4 py-3 text-xs text-muted">
                    {u.lastLogin ? new Date(u.lastLogin).toLocaleString() : '—'}
                  </td>
                  <td className="px-4 py-3 text-right">
                    <Button
                      size="sm"
                      variant="ghost"
                      aria-label={`Edit ${u.name}`}
                      onClick={() => setEditing(u)}
                    >
                      <MoreVertical className="h-3.5 w-3.5" />
                    </Button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </Card>
      )}

      <RoleEditModal user={editing} onClose={() => setEditing(null)} />
    </div>
  );
}

function RoleEditModal({
  user,
  onClose,
}: {
  user: AdminUser | null;
  onClose: () => void;
}) {
  const [role, setRole] = useState<AdminUser['role']>('user');
  const updateRole = useUpdateUserRole();

  useEffect(() => {
    if (user) setRole(user.role);
  }, [user]);

  return (
    <Modal
      open={!!user}
      onClose={onClose}
      title="Edit user"
      description={user?.email}
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={updateRole.isPending}>
            Cancel
          </Button>
          <Button
            disabled={!user || role === user.role || updateRole.isPending}
            onClick={async () => {
              if (!user) return;
              await updateRole.mutateAsync({ userId: user.userId, role });
              onClose();
            }}
          >
            {updateRole.isPending ? <Spinner className="text-white" /> : 'Save changes'}
          </Button>
        </>
      }
    >
      {user && (
        <div className="space-y-3">
          <div>
            <span id="user-role-label" className="block text-xs font-medium text-foreground">Role</span>
            <div role="radiogroup" aria-labelledby="user-role-label" className="mt-1 flex gap-2">
              {(['user', 'manager', 'admin'] as const).map((r) => (
                <button
                  key={r}
                  type="button"
                  onClick={() => setRole(r)}
                  className={cn(
                    'flex-1 rounded border px-3 py-2 text-sm capitalize transition-colors',
                    role === r
                      ? 'border-primary bg-primary/10 font-medium text-primary'
                      : 'border-border bg-surface text-foreground hover:bg-surface-2',
                  )}
                >
                  {r}
                </button>
              ))}
            </div>
            <p className="mt-1 text-[11px] text-muted">
              admin → all projects + admin endpoints · manager → team-scoped manage · user → read access
            </p>
          </div>
          {updateRole.error && (
            <div className="rounded border border-red-200 bg-red-50 px-2.5 py-1.5 text-xs text-red-800">
              {(updateRole.error as Error).message}
            </div>
          )}
        </div>
      )}
    </Modal>
  );
}
