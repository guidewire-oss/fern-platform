import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, cleanup } from '@testing-library/react';

const mockConnections = vi.fn();
vi.mock('./hooks', () => ({ useJiraConnections: () => mockConnections() }));

const mockMapping = vi.fn();
const mockFields = vi.fn();
const saveMut = { mutate: vi.fn(), isPending: false, error: null, isSuccess: false };
const resetMut = { mutate: vi.fn(), isPending: false, error: null };
vi.mock('./fieldMappingHooks', () => ({
  useJiraFieldMapping: () => mockMapping(),
  useJiraFields: () => mockFields(),
  useSaveJiraFieldMapping: () => saveMut,
  useResetJiraFieldMapping: () => resetMut,
}));

import { FieldMappingVisual } from './FieldMappingVisual';

const CONNECTED = { data: [{ id: 'c1', name: 'ACME JIRA' }], isLoading: false, isError: false };
const FIELDS = {
  data: [
    { id: 'summary', name: 'Summary', custom: false, multiValue: false },
    { id: 'labels', name: 'Labels', custom: false, multiValue: true },
    { id: 'customfield_1', name: 'Epic Link', custom: true, multiValue: false },
  ],
  isLoading: false,
  isError: false,
};

beforeEach(() => {
  saveMut.mutate.mockClear();
  resetMut.mutate.mockClear();
  // jsdom has no layout engine; stub getBoundingClientRect so the SVG
  // line math (portCoords) doesn't blow up during render.
  Element.prototype.getBoundingClientRect = vi.fn(() => ({
    x: 0, y: 0, top: 0, left: 0, right: 0, bottom: 0, width: 0, height: 0, toJSON: () => ({}),
  })) as unknown as typeof Element.prototype.getBoundingClientRect;
  global.ResizeObserver = class {
    observe() {}
    unobserve() {}
    disconnect() {}
  } as unknown as typeof ResizeObserver;
  mockConnections.mockReturnValue(CONNECTED);
  mockFields.mockReturnValue(FIELDS);
  mockMapping.mockReturnValue({ data: { projectId: 'p1', entries: [] }, isLoading: false });
});
afterEach(() => cleanup());

describe('FieldMappingVisual', () => {
  it('shows an empty state when the project has no JIRA connection', () => {
    mockConnections.mockReturnValue({ data: [], isLoading: false, isError: false });
    render(<FieldMappingVisual projectId="p1" canManage />);
    expect(screen.getByText(/Connect a JIRA project above/i)).toBeTruthy();
    expect(screen.queryByText(/Save mapping/i)).toBeNull();
  });

  it('renders both columns: Fern fields and JIRA fields', () => {
    render(<FieldMappingVisual projectId="p1" canManage />);
    expect(screen.getByText('Fern fields')).toBeTruthy();
    expect(screen.getByText('JIRA fields')).toBeTruthy();
    expect(screen.getByText('Requirement ID')).toBeTruthy();
    expect(screen.getByText('Summary')).toBeTruthy();
    expect(screen.getByText('Epic Link')).toBeTruthy();
  });

  it('reflects an existing mapping and blocks save while required fields are unmapped', () => {
    mockMapping.mockReturnValue({
      data: {
        projectId: 'p1',
        entries: [
          { fernField: 'REQUIREMENT_ID', jiraFieldId: 'summary', jiraFieldIsMultiValue: false, reductionStrategy: null },
        ],
      },
      isLoading: false,
    });
    render(<FieldMappingVisual projectId="p1" canManage />);
    // REQUIREMENT_TITLE (also required) is still unmapped → validation error, Save disabled.
    expect(screen.getAllByText(/is required and must be mapped/i).length).toBeGreaterThan(0);
    const save = screen.getByRole('button', { name: /Save mapping/i }) as HTMLButtonElement;
    expect(save.disabled).toBe(true);
  });

  it('is read-only for non-managers (no Save/Reset)', () => {
    render(<FieldMappingVisual projectId="p1" canManage={false} />);
    expect(screen.getByText('Fern fields')).toBeTruthy();
    expect(screen.queryByText(/Save mapping/i)).toBeNull();
    expect(screen.queryByText(/Reset/i)).toBeNull();
  });
});
