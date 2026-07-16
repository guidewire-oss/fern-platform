import { useEffect } from 'react';
import { Link, useNavigate } from '@tanstack/react-router';
import { Spinner } from '@/components/ui/Spinner';
import { EmptyState } from '@/components/ui/EmptyState';
import { useCurrentUser } from '@/features/auth/useCurrentUser';

// Restores v1's `/manager-dashboard` bookmark by redirecting to
// `/v2/summaries` with view=tree + favoritesOnly=true preset. Role-gated
// to manager / admin to match v1's behavior (v1's nav rendered the
// item with `managerOnly: true` and gated the page with
// `<AccessDenied />` for non-managers).
//
// This is a *routing component*, not a new view — Summaries already
// supports tree view + favorites filtering; we just deep-link into it.

export default function ManagerDashboardRedirect() {
  const navigate = useNavigate();
  const { data: user, isLoading } = useCurrentUser();

  const role = (user?.role ?? '').toLowerCase();
  const allowed = role === 'manager' || role === 'admin';

  useEffect(() => {
    if (!isLoading && allowed) {
      navigate({
        to: '/summaries',
        search: { view: 'tree', favoritesOnly: 'true' },
        replace: true,
      });
    }
  }, [isLoading, allowed, navigate]);

  if (isLoading) {
    return (
      <div className="flex items-center gap-2 text-muted">
        <Spinner /> Loading…
      </div>
    );
  }

  if (!user) {
    // Not signed in. The TopBar's Sign-in button is the right CTA;
    // surface a minimal empty state here so the page isn't blank.
    return (
      <EmptyState
        title="Sign in required"
        description="The Manager Dashboard is only available to authenticated managers and admins."
        action={
          <a
            href="/auth/start"
            className="rounded border border-primary bg-primary px-3 py-1 text-xs text-white hover:bg-primary/90"
          >
            Sign in
          </a>
        }
      />
    );
  }

  if (!allowed) {
    return (
      <EmptyState
        title="Manager access only"
        description={`This view is restricted to manager and admin roles. You're signed in as ${user.role || 'user'}.`}
        action={
          <div className="flex items-center gap-2">
            <Link
              to="/"
              className="rounded border border-border bg-surface px-3 py-1 text-xs text-foreground hover:bg-surface-2"
            >
              Back to Dashboard
            </Link>
            <Link
              to="/summaries"
              search={{ view: undefined, favoritesOnly: undefined }}
              className="rounded border border-border bg-surface px-3 py-1 text-xs text-foreground hover:bg-surface-2"
            >
              Open Summaries
            </Link>
          </div>
        }
      />
    );
  }

  // While the redirect effect is in flight, render a spinner so the
  // page doesn't flash empty content before navigation completes.
  return (
    <div className="flex items-center gap-2 text-muted">
      <Spinner /> Redirecting to Summaries…
    </div>
  );
}
