import { type ReactNode } from 'react';
import { Link } from '@tanstack/react-router';
import { Spinner } from '@/components/ui/Spinner';
import { EmptyState } from '@/components/ui/EmptyState';
import { useCurrentUser } from './useCurrentUser';

// Client-side role guard for v2 routes. The server is the source of
// truth for access control — every protected API call gates correctly
// even if a non-admin manages to reach an admin route. But without
// this guard, the user briefly sees the page chrome, the data-fetch
// 401s, and they land on an unhelpful EmptyState. The guard skips
// that flash by checking role *before* the route component mounts.
//
// Allowed roles are matched case-insensitively. Admin satisfies any
// list that contains 'admin' or 'manager' (admin > manager).
export function RoleGuard({
  allow,
  children,
}: {
  allow: Array<'admin' | 'manager'>;
  children: ReactNode;
}) {
  const { data: user, isLoading } = useCurrentUser();

  if (isLoading) {
    return (
      <div className="flex items-center gap-2 text-muted">
        <Spinner /> Checking access…
      </div>
    );
  }

  if (!user) {
    return (
      <EmptyState
        title="Sign in required"
        description="This page is only available to authenticated users."
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

  const role = (user.role ?? '').toLowerCase();
  const allowed = allow.some((needed) => needed === role)
    // Admin satisfies any allow-list that includes manager (admin > manager).
    || (role === 'admin' && allow.includes('manager'));

  if (!allowed) {
    return (
      <EmptyState
        title={`${allow.join(' or ')} access only`}
        description={`This page is restricted to the ${allow.join(' or ')} role. You're signed in as ${user.role || 'user'}.`}
        action={
          <Link
            to="/"
            className="rounded border border-border bg-surface px-3 py-1 text-xs text-foreground hover:bg-surface-2"
          >
            Back to Dashboard
          </Link>
        }
      />
    );
  }

  return <>{children}</>;
}
