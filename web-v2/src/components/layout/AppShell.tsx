import { useEffect, useRef, useState, type ReactNode } from 'react';
import { Link } from '@tanstack/react-router';
import {
  LayoutDashboard,
  FolderKanban,
  PlayCircle,
  Bookmark,
  Sparkles,
  CircleUser,
  ExternalLink,
  ChevronDown,
  LogIn,
  LogOut,
  Gauge,
  Tag,
  Users,
  Settings as SettingsIcon,
  ShieldCheck,
  LineChart,
} from 'lucide-react';
import { cn } from '@/lib/cn';
import { useCurrentUser, initialsOf } from '@/features/auth/useCurrentUser';

interface NavItem {
  to: string;
  label: string;
  icon: typeof LayoutDashboard;
  // `requires` gates the item by role. Omitted → visible to everyone
  // who's signed in. Matched case-insensitively against `user.role`.
  requires?: Array<'admin' | 'manager'>;
}

interface NavSection {
  heading: string;
  items: NavItem[];
}

const NAV: NavSection[] = [
  {
    heading: 'Overview',
    items: [
      { to: '/',                  label: 'Dashboard',         icon: LayoutDashboard },
      { to: '/manager-dashboard', label: 'Manager dashboard', icon: Gauge,
        requires: ['admin', 'manager'] },
    ],
  },
  {
    heading: 'Test intelligence',
    items: [
      { to: '/projects',  label: 'Projects',    icon: FolderKanban },
      { to: '/test-runs', label: 'Test runs',   icon: PlayCircle },
      { to: '/summaries', label: 'Summaries',   icon: LineChart },
    ],
  },
  {
    heading: 'Workspace',
    items: [
      { to: '/saved-views', label: 'Saved views', icon: Bookmark },
      { to: '/tags',        label: 'Tags',        icon: Tag },
    ],
  },
  {
    heading: 'Administration',
    items: [
      { to: '/users',    label: 'Users',           icon: Users,        requires: ['admin'] },
      { to: '/settings', label: 'System settings', icon: SettingsIcon },
      { to: '/admin',    label: 'Admin overview',  icon: ShieldCheck,  requires: ['admin'] },
    ],
  },
];

// Exported alongside the AppShell component so the unit tests can exercise
// the visibility predicate without rendering. Splitting it into a separate
// module would force a barrel-style import everywhere the shell is used.
// eslint-disable-next-line react-refresh/only-export-components
export function canSeeNavItem(
  item: NavItem,
  role: string | undefined,
): boolean {
  if (!item.requires || item.requires.length === 0) return true;
  if (!role) return false;
  const r = role.toLowerCase();
  return item.requires.some((needed) => needed === r);
}

export function AppShell({ children }: { children: ReactNode }) {
  return (
    <div className="min-h-screen bg-background text-foreground">
      <div className="grid min-h-screen grid-cols-[240px_1fr]">
        <Sidebar />
        <div className="flex min-w-0 flex-col">
          <TopBar />
          <main className="flex-1 overflow-x-auto px-8 py-8">{children}</main>
        </div>
      </div>
    </div>
  );
}

function Sidebar() {
  const { data: user } = useCurrentUser();
  const role = user?.role;

  // Filter each section's items by role; drop sections that end up empty.
  const visibleSections = NAV
    .map((section) => ({
      heading: section.heading,
      items: section.items.filter((item) => canSeeNavItem(item, role)),
    }))
    .filter((section) => section.items.length > 0);

  return (
    <aside className="fern-sidebar sticky top-0 flex h-screen flex-col">
      <div className="px-5 py-5">
        <div className="flex items-center gap-2.5">
          <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-gradient-primary text-base text-white shadow-md">
            🌿
          </div>
          <div>
            <div className="font-semibold text-white">Fern</div>
            <div className="text-[10px] uppercase tracking-[0.15em] text-slate-400">
              Test Intelligence
            </div>
          </div>
        </div>
      </div>

      <div className="fern-divider mx-3" />

      <nav className="flex-1 overflow-y-auto px-2 py-3" aria-label="Primary">
        {visibleSections.map((section) => (
          <div key={section.heading} className="mb-4">
            <div className="px-3 pb-1 text-[10px] font-semibold uppercase tracking-[0.18em] text-slate-500">
              {section.heading}
            </div>
            <ul className="space-y-0.5">
              {section.items.map(({ to, label, icon: Icon }) => (
                <li key={to}>
                  <Link
                    to={to}
                    activeOptions={{ exact: to === '/' }}
                    className={cn(
                      'group relative flex items-center gap-2.5 rounded-md px-3 py-2 text-sm',
                      'text-sidebar-fg transition-colors duration-150',
                      'hover:bg-white/5 hover:text-white',
                    )}
                    activeProps={{
                      className:
                        '!bg-white/10 !text-white font-medium shadow-inner',
                    }}
                  >
                    <Icon className="h-4 w-4 shrink-0 opacity-80 group-hover:opacity-100" aria-hidden />
                    <span>{label}</span>
                  </Link>
                </li>
              ))}
            </ul>
          </div>
        ))}
      </nav>

      <div className="border-t border-white/5 px-4 py-3">
        <div className="flex items-start gap-2 rounded-lg bg-white/5 p-2.5 text-[11px] leading-snug text-slate-300">
          <Sparkles className="h-3.5 w-3.5 shrink-0 text-amber-300" aria-hidden />
          <div>
            <div className="font-medium text-white">v2 UI preview</div>
            <div className="text-slate-400">
              Edit <code className="text-slate-200">web-v2/src/</code> and rebuild.
            </div>
          </div>
        </div>
      </div>
    </aside>
  );
}

function TopBar() {
  const { data: user, isLoading } = useCurrentUser();
  // The synthetic dev-admin row inserted when AUTH_ENABLED=false has
  // userId=`dev-admin`; surfacing that lets the UI label the env so
  // contributors can tell at a glance they're not in real SSO.
  const isDevAuth = user?.userId === 'dev-admin';

  return (
    <header className="sticky top-0 z-10 border-b border-border bg-surface/80 backdrop-blur">
      <div className="flex h-14 items-center justify-between gap-4 px-8">
        <div className="text-xs text-muted">
          {isDevAuth ? (
            <>
              <span className="rounded-full bg-amber-100 px-2 py-0.5 text-[10px] font-medium uppercase tracking-wider text-amber-800">
                Local dev
              </span>
              <span className="ml-2">Auth disabled — admin user injected</span>
            </>
          ) : null}
          {/* Signed-in identity + role live in the UserPill on the right;
              no need to repeat "Signed in as …" here. */}
        </div>

        <div className="flex items-center gap-3">
          <a
            href="/"
            className="inline-flex items-center gap-1 rounded-md border border-border bg-surface px-2.5 py-1 text-xs text-muted transition-colors hover:border-primary hover:text-primary"
            title="Open legacy UI"
          >
            <ExternalLink className="h-3 w-3" />
            Legacy UI
          </a>
          {isLoading ? (
            <div className="h-6 w-32 animate-pulse rounded-full bg-surface-2" aria-hidden />
          ) : user ? (
            <UserPill user={user} showSignOut={!isDevAuth} />
          ) : (
            <a
              href="/auth/start"
              className="inline-flex items-center gap-1 rounded-md border border-primary bg-primary px-2.5 py-1 text-xs font-medium text-white hover:bg-primary/90"
            >
              <LogIn className="h-3 w-3" /> Sign in
            </a>
          )}
        </div>
      </div>
    </header>
  );
}

function UserPill({
  user,
  showSignOut,
}: {
  user: import('@/features/auth/useCurrentUser').CurrentUser;
  showSignOut: boolean;
}) {
  const [open, setOpen] = useState(false);
  const wrapRef = useRef<HTMLDivElement>(null);

  // Close on outside click and Escape. Both must run only while open so
  // we don't pin global listeners for inactive menus across the page.
  useEffect(() => {
    if (!open) return;
    const onClick = (e: MouseEvent) => {
      if (wrapRef.current && !wrapRef.current.contains(e.target as Node)) {
        setOpen(false);
      }
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setOpen(false);
    };
    document.addEventListener('mousedown', onClick);
    document.addEventListener('keydown', onKey);
    return () => {
      document.removeEventListener('mousedown', onClick);
      document.removeEventListener('keydown', onKey);
    };
  }, [open]);

  const displayName = user.name?.trim() || user.email || 'User';
  const initials = initialsOf(user);
  const isAdmin = (user.role || '').toLowerCase() === 'admin';

  return (
    <div ref={wrapRef} className="relative">
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        aria-haspopup="menu"
        aria-expanded={open}
        title={`${user.email || 'View profile'}${user.role ? ` · ${user.role}` : ''}`}
        className={cn(
          'flex items-center gap-2 rounded-full border border-border bg-surface px-2 py-1 text-xs transition-colors',
          open ? 'border-primary' : 'hover:border-primary',
        )}
      >
        {user.profileUrl ? (
          <img
            src={user.profileUrl}
            alt=""
            className="h-5 w-5 rounded-full object-cover"
            referrerPolicy="no-referrer"
          />
        ) : (
          <div className="flex h-5 w-5 items-center justify-center rounded-full bg-gradient-primary text-[10px] font-bold text-white">
            {initials}
          </div>
        )}
        <span className="max-w-[14ch] truncate text-foreground">{displayName}</span>
        {user.role && <RoleBadge role={user.role} />}
        <ChevronDown
          className={cn(
            'h-3 w-3 text-muted transition-transform',
            open && 'rotate-180',
          )}
        />
      </button>

      {open && (
        <div
          role="menu"
          className="absolute right-0 z-20 mt-1 w-64 overflow-hidden rounded-lg border border-border bg-surface text-xs shadow-lg"
        >
          <div className="border-b border-border bg-surface-2 px-3 py-2.5">
            <div className="truncate text-sm font-semibold text-foreground">
              {displayName}
            </div>
            {user.email && (
              <div className="truncate text-muted">{user.email}</div>
            )}
            {user.role && (
              <div className="mt-1.5 flex items-center gap-1.5">
                <span className="text-muted">Role:</span>
                <RoleBadge role={user.role} />
              </div>
            )}
          </div>

          <div className="py-1">
            <MenuLink to="/profile" icon={CircleUser} onClick={() => setOpen(false)}>
              View profile &amp; preferences
            </MenuLink>
            {isAdmin && (
              <MenuLink to="/admin" icon={ShieldCheck} onClick={() => setOpen(false)}>
                Admin panel
              </MenuLink>
            )}
            {showSignOut && (
              <>
                <div className="my-1 border-t border-border" />
                <SignOutMenuItem />
              </>
            )}
          </div>
        </div>
      )}
    </div>
  );
}

function MenuLink({
  to,
  icon: Icon,
  onClick,
  children,
}: {
  to: string;
  icon: typeof CircleUser;
  onClick: () => void;
  children: ReactNode;
}) {
  return (
    <Link
      to={to}
      role="menuitem"
      onClick={onClick}
      className="flex items-center gap-2.5 px-3 py-2 text-foreground hover:bg-surface-2"
    >
      <Icon className="h-3.5 w-3.5 text-muted" />
      <span>{children}</span>
    </Link>
  );
}

function SignOutMenuItem() {
  return (
    <button
      type="button"
      role="menuitem"
      onClick={() => void signOutFlow()}
      className="flex w-full items-center gap-2.5 px-3 py-2 text-left text-status-failed-fg hover:bg-status-failed-bg"
    >
      <LogOut className="h-3.5 w-3.5" />
      <span>Sign out</span>
    </button>
  );
}

async function signOutFlow() {
  // Same protocol as v1: POST /auth/logout returns { logout_url } and
  // the SPA navigates to the IdP's end-session endpoint so the
  // provider session is killed too. If the POST fails (network blip,
  // 5xx) we still send the user to /auth/login so they don't end up
  // stranded inside the app with a stale cookie.
  try {
    const res = await fetch('/auth/logout', {
      method: 'POST',
      credentials: 'same-origin',
      headers: {
        'Content-Type': 'application/json',
        'X-Requested-With': 'XMLHttpRequest',
      },
    });
    if (res.ok) {
      const body = (await res.json().catch(() => null)) as { logout_url?: string } | null;
      window.location.href = body?.logout_url || '/auth/login';
      return;
    }
  } catch {
    // fall through
  }
  window.location.href = '/auth/login';
}

function RoleBadge({ role }: { role: string }) {
  const r = role.toLowerCase();
  return (
    <span
      className={cn(
        'rounded-full px-1.5 py-0.5 text-[9px] font-semibold uppercase tracking-wider',
        r === 'admin'
          ? 'bg-gradient-primary text-white'
          : r === 'manager'
          ? 'bg-indigo-100 text-indigo-800'
          : 'bg-slate-100 text-slate-700',
      )}
    >
      {role}
    </span>
  );
}
