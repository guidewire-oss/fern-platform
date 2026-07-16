// Tests the TopBar dropdown menu behavior. We can't easily exercise the
// exported `AppShell` (which also renders the Sidebar with many nav
// Links) without a full TanStack router, so we re-import the file with a
// mocked `Link` and drive the surface directly through a small test
// harness that renders just the pill.
//
// FR-24a/b/e are exercised here: toggle, outside-click close, Escape
// close, role-conditional Admin Panel link, dev-admin Sign-out exemption.

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@testing-library/react';

// Stub the router Link so the menu items can render outside a router
// context. The mock has to be in place before AppShell is imported.
vi.mock('@tanstack/react-router', () => ({
  Link: ({
    to,
    children,
    onClick,
    className,
    role,
  }: {
    to: string;
    children: React.ReactNode;
    onClick?: () => void;
    className?: string;
    role?: string;
  }) => (
    <a href={to} onClick={onClick} className={className} role={role} data-to={to}>
      {children}
    </a>
  ),
}));

// useCurrentUser is queried by the TopBar; we don't want react-query
// here, so swap the hook for a controllable stand-in.
const mockUser = vi.fn();
vi.mock('@/features/auth/useCurrentUser', async () => {
  const actual = await vi.importActual<typeof import('@/features/auth/useCurrentUser')>(
    '@/features/auth/useCurrentUser',
  );
  return {
    ...actual,
    useCurrentUser: () => mockUser(),
  };
});

import { AppShell } from './AppShell';

const renderShell = () =>
  render(
    <AppShell>
      <div>page body</div>
    </AppShell>,
  );

beforeEach(() => {
  mockUser.mockReset();
});
afterEach(() => {
  cleanup();
});

const ADMIN = {
  id: '1',
  userId: 'u1',
  email: 'ada@example.com',
  name: 'Ada Lovelace',
  role: 'admin',
};
const USER = { ...ADMIN, userId: 'u2', email: 'beth@example.com', name: 'Beth', role: 'user' };
const DEV   = { ...ADMIN, userId: 'dev-admin', role: 'admin', name: 'Dev Admin' };

describe('TopBar user pill', () => {
  it('renders Sign in when no user', () => {
    mockUser.mockReturnValue({ data: null, isLoading: false });
    renderShell();
    expect(screen.getByText(/sign in/i)).toBeInTheDocument();
    expect(screen.queryByRole('menu')).toBeNull();
  });

  it('renders pill with user name and role; menu is closed by default', () => {
    mockUser.mockReturnValue({ data: ADMIN, isLoading: false });
    renderShell();
    expect(screen.getByRole('button', { expanded: false })).toBeInTheDocument();
    expect(screen.getByText(/ada lovelace/i)).toBeInTheDocument();
    expect(screen.queryByRole('menu')).toBeNull();
  });

  it('opens on click and closes again on second click (toggle)', () => {
    mockUser.mockReturnValue({ data: ADMIN, isLoading: false });
    renderShell();
    const trigger = screen.getByRole('button', { expanded: false });
    fireEvent.click(trigger);
    expect(screen.getByRole('menu')).toBeInTheDocument();
    fireEvent.click(trigger);
    expect(screen.queryByRole('menu')).toBeNull();
  });

  it('closes when clicking outside', () => {
    mockUser.mockReturnValue({ data: ADMIN, isLoading: false });
    renderShell();
    fireEvent.click(screen.getByRole('button', { expanded: false }));
    expect(screen.getByRole('menu')).toBeInTheDocument();
    fireEvent.mouseDown(document.body);
    expect(screen.queryByRole('menu')).toBeNull();
  });

  it('closes on Escape', () => {
    mockUser.mockReturnValue({ data: ADMIN, isLoading: false });
    renderShell();
    fireEvent.click(screen.getByRole('button', { expanded: false }));
    fireEvent.keyDown(document, { key: 'Escape' });
    expect(screen.queryByRole('menu')).toBeNull();
  });

  it('shows Admin Panel link only when role is admin', () => {
    mockUser.mockReturnValue({ data: USER, isLoading: false });
    renderShell();
    fireEvent.click(screen.getByRole('button', { expanded: false }));
    expect(screen.queryByText(/admin panel/i)).toBeNull();
    expect(screen.getByText(/view profile/i)).toBeInTheDocument();
  });

  it('shows Admin Panel link when role is admin', () => {
    mockUser.mockReturnValue({ data: ADMIN, isLoading: false });
    renderShell();
    fireEvent.click(screen.getByRole('button', { expanded: false }));
    expect(screen.getByText(/admin panel/i)).toBeInTheDocument();
  });

  it('hides Sign out for the synthetic dev-admin principal', () => {
    mockUser.mockReturnValue({ data: DEV, isLoading: false });
    renderShell();
    fireEvent.click(screen.getByRole('button', { expanded: false }));
    expect(screen.queryByText(/sign out/i)).toBeNull();
    expect(screen.getByText(/local dev/i)).toBeInTheDocument();
  });

  it('shows Sign out for real users', () => {
    mockUser.mockReturnValue({ data: ADMIN, isLoading: false });
    renderShell();
    fireEvent.click(screen.getByRole('button', { expanded: false }));
    expect(screen.getByText(/sign out/i)).toBeInTheDocument();
  });
});

describe('TopBar nav visibility', () => {
  it('shows Manager dashboard for managers', () => {
    mockUser.mockReturnValue({
      data: { ...USER, role: 'manager' },
      isLoading: false,
    });
    renderShell();
    expect(screen.getByText(/manager dashboard/i)).toBeInTheDocument();
  });

  it('shows Manager dashboard for admins', () => {
    mockUser.mockReturnValue({ data: ADMIN, isLoading: false });
    renderShell();
    expect(screen.getByText(/manager dashboard/i)).toBeInTheDocument();
  });

  it('hides Manager dashboard for plain users', () => {
    mockUser.mockReturnValue({ data: USER, isLoading: false });
    renderShell();
    expect(screen.queryByText(/manager dashboard/i)).toBeNull();
  });

  it('hides admin-only nav items for non-admins', () => {
    mockUser.mockReturnValue({ data: USER, isLoading: false });
    renderShell();
    expect(screen.queryByText(/^users$/i)).toBeNull();
    expect(screen.queryByText(/admin overview/i)).toBeNull();
  });
});
