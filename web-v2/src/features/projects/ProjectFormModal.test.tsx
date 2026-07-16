import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup, waitFor } from '@testing-library/react';

// Capture what the mutations receive without a real react-query client.
const createMut = { mutateAsync: vi.fn().mockResolvedValue({}), isPending: false, error: null };
const updateMut = { mutateAsync: vi.fn().mockResolvedValue({}), isPending: false, error: null };
vi.mock('./mutations', () => ({
  useCreateProject: () => createMut,
  useUpdateProject: () => updateMut,
}));

// Controllable auth data. roleGroups keeps its real pure helpers.
const mockUser = vi.fn();
vi.mock('@/features/auth/useCurrentUser', () => ({ useCurrentUser: () => mockUser() }));

const mockRoleGroups = vi.fn();
vi.mock('@/features/auth/roleGroups', async () => {
  const actual =
    await vi.importActual<typeof import('@/features/auth/roleGroups')>('@/features/auth/roleGroups');
  return { ...actual, useRoleGroups: () => mockRoleGroups() };
});

// Existing projects for the duplicate-name guard.
const mockProjects = vi.fn();
vi.mock('./hooks', () => ({ useProjects: () => mockProjects() }));

import { ProjectFormModal } from './ProjectFormModal';

const RG = { adminGroup: 'admin', managerGroup: 'manager', userGroup: 'user' };

beforeEach(() => {
  createMut.mutateAsync.mockClear();
  updateMut.mutateAsync.mockClear();
  mockRoleGroups.mockReturnValue({ data: RG });
  mockUser.mockReturnValue({ data: { role: 'user', groups: ['team-a', 'team-b', 'user'] } });
  mockProjects.mockReturnValue({ data: { projects: [] } });
});
afterEach(() => cleanup());

const noop = () => {};

describe('ProjectFormModal — create (v1 parity)', () => {
  it('does not render a Project ID input', () => {
    render(<ProjectFormModal open onClose={noop} />);
    // No Project ID input (placeholder) and none of its former hint copy.
    expect(screen.queryByPlaceholderText('my-service')).toBeNull();
    expect(screen.queryByText(/FERN_PROJECT_ID/i)).toBeNull();
  });

  it('renders Team as a dropdown of the user’s teams (role groups excluded)', () => {
    render(<ProjectFormModal open onClose={noop} />);
    const select = screen.getByRole('combobox');
    const options = Array.from(select.querySelectorAll('option')).map((o) => o.textContent);
    expect(options).toEqual(['team-a', 'team-b']);
    expect(screen.queryByPlaceholderText('team-a')).toBeNull(); // not a text input
  });

  it('submits an empty projectId (backend auto-generates) and the default first team', async () => {
    render(<ProjectFormModal open onClose={noop} />);
    fireEvent.change(screen.getByPlaceholderText('My Service'), { target: { value: 'My Svc' } });
    fireEvent.click(screen.getByRole('button', { name: /create project/i }));
    await waitFor(() => expect(createMut.mutateAsync).toHaveBeenCalledTimes(1));
    expect(createMut.mutateAsync).toHaveBeenCalledWith(
      expect.objectContaining({ projectId: '', name: 'My Svc', team: 'team-a', defaultBranch: 'main' }),
    );
  });

  it('gates submission on name only', () => {
    render(<ProjectFormModal open onClose={noop} />);
    const submit = screen.getByRole('button', { name: /create project/i });
    expect(submit).toBeDisabled();
    fireEvent.change(screen.getByPlaceholderText('My Service'), { target: { value: 'X' } });
    expect(submit).toBeEnabled();
  });

  it('blocks a duplicate name (case-insensitive) and shows a message', () => {
    mockProjects.mockReturnValue({ data: { projects: [{ projectId: 'other', name: 'Payments' }] } });
    render(<ProjectFormModal open onClose={noop} />);
    fireEvent.change(screen.getByPlaceholderText('My Service'), { target: { value: '  payments ' } });
    expect(screen.getByText(/already exists/i)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /create project/i })).toBeDisabled();
  });

  it('allows a unique name when others exist', () => {
    mockProjects.mockReturnValue({ data: { projects: [{ projectId: 'other', name: 'Payments' }] } });
    render(<ProjectFormModal open onClose={noop} />);
    fireEvent.change(screen.getByPlaceholderText('My Service'), { target: { value: 'Billing' } });
    expect(screen.queryByText(/already exists/i)).toBeNull();
    expect(screen.getByRole('button', { name: /create project/i })).toBeEnabled();
  });

  it('admins get a "No team" option defaulted to empty', async () => {
    mockUser.mockReturnValue({ data: { role: 'admin', groups: ['admin', 'team-a'] } });
    render(<ProjectFormModal open onClose={noop} />);
    const options = Array.from(screen.getByRole('combobox').querySelectorAll('option')).map(
      (o) => o.textContent,
    );
    expect(options).toEqual(['No team', 'team-a']);
    fireEvent.change(screen.getByPlaceholderText('My Service'), { target: { value: 'Svc' } });
    fireEvent.click(screen.getByRole('button', { name: /create project/i }));
    await waitFor(() => expect(createMut.mutateAsync).toHaveBeenCalled());
    expect(createMut.mutateAsync).toHaveBeenCalledWith(expect.objectContaining({ team: undefined }));
  });

  it('non-admin with no teams submits an empty team', async () => {
    mockUser.mockReturnValue({ data: { role: 'user', groups: ['user'] } });
    render(<ProjectFormModal open onClose={noop} />);
    fireEvent.change(screen.getByPlaceholderText('My Service'), { target: { value: 'Svc' } });
    fireEvent.click(screen.getByRole('button', { name: /create project/i }));
    await waitFor(() => expect(createMut.mutateAsync).toHaveBeenCalled());
    expect(createMut.mutateAsync).toHaveBeenCalledWith(expect.objectContaining({ team: undefined }));
  });
});

describe('ProjectFormModal — edit keeps own name', () => {
  it('does not flag the project’s own current name as a duplicate', () => {
    mockProjects.mockReturnValue({ data: { projects: [{ projectId: 'proj-x', name: 'X' }] } });
    render(
      <ProjectFormModal
        open
        onClose={noop}
        initial={{ dbId: '99', projectId: 'proj-x', name: 'X', team: 'team-a', defaultBranch: 'main' }}
      />,
    );
    expect(screen.queryByText(/already exists/i)).toBeNull();
    expect(screen.getByRole('button', { name: /save changes/i })).toBeEnabled();
  });
});

describe('ProjectFormModal — edit (unchanged)', () => {
  const initial = { dbId: '99', projectId: 'proj-x', name: 'X', team: 'team-a', defaultBranch: 'main' };

  it('shows the immutable Project ID as context and never sends projectId', async () => {
    render(<ProjectFormModal open onClose={noop} initial={initial} />);
    expect(screen.getByText(/proj-x/)).toBeInTheDocument();
    expect(screen.queryByPlaceholderText('my-service')).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: /save changes/i }));
    await waitFor(() => expect(updateMut.mutateAsync).toHaveBeenCalledTimes(1));
    const arg = updateMut.mutateAsync.mock.calls[0][0];
    expect(arg.id).toBe('99');
    expect(arg.input).not.toHaveProperty('projectId');
  });
});
