import { describe, it, expect } from 'vitest';
import { sortProjects } from './sortProjects';
import type { Project } from '@/lib/types';

function p(over: Partial<Project> & { projectId: string; name: string }): Project {
  return {
    id: over.projectId,
    description: '',
    isActive: true,
    team: '',
    canManage: false,
    stats: over.stats ?? { totalTestRuns: 0, successRate: 0, averageDuration: 0, lastRunTime: null },
    ...over,
  };
}

const st = (totalTestRuns: number, successRate = 0, lastRunTime: string | null = null) => ({
  totalTestRuns, successRate, averageDuration: 0, lastRunTime,
});

describe('sortProjects', () => {
  it('sorts by name asc/desc', () => {
    const list = [p({ projectId: 'b', name: 'Beta' }), p({ projectId: 'a', name: 'Alpha' })];
    expect(sortProjects(list, 'name', 'asc').map((x) => x.name)).toEqual(['Alpha', 'Beta']);
    expect(sortProjects(list, 'name', 'desc').map((x) => x.name)).toEqual(['Beta', 'Alpha']);
  });

  it('sorts by runs and reverses with direction', () => {
    const list = [
      p({ projectId: 'a', name: 'A', stats: st(10) }),
      p({ projectId: 'b', name: 'B', stats: st(50) }),
      p({ projectId: 'c', name: 'C', stats: st(30) }),
    ];
    expect(sortProjects(list, 'runs', 'asc').map((x) => x.name)).toEqual(['A', 'C', 'B']);
    expect(sortProjects(list, 'runs', 'desc').map((x) => x.name)).toEqual(['B', 'C', 'A']);
  });

  it('prefers the per-window runsByProject override when given', () => {
    // All-time stats are identical (the "runs does nothing" case) but the
    // window runs differ → order must follow the window numbers.
    const list = [
      p({ projectId: 'a', name: 'A', stats: st(450) }),
      p({ projectId: 'b', name: 'B', stats: st(450) }),
    ];
    const byWindow = { a: 5, b: 40 };
    expect(sortProjects(list, 'runs', 'desc', byWindow).map((x) => x.name)).toEqual(['B', 'A']);
  });

  it('breaks ties by name ascending regardless of direction', () => {
    const list = [
      p({ projectId: 'z', name: 'Zeta', stats: st(10) }),
      p({ projectId: 'a', name: 'Alpha', stats: st(10) }),
    ];
    // Equal runs → tiebreak name asc, in both directions.
    expect(sortProjects(list, 'runs', 'asc').map((x) => x.name)).toEqual(['Alpha', 'Zeta']);
    expect(sortProjects(list, 'runs', 'desc').map((x) => x.name)).toEqual(['Alpha', 'Zeta']);
  });

  it('sorts by pass rate and last activity', () => {
    const list = [
      p({ projectId: 'a', name: 'A', stats: st(1, 0.5, '2026-01-01T00:00:00Z') }),
      p({ projectId: 'b', name: 'B', stats: st(1, 0.9, '2026-03-01T00:00:00Z') }),
    ];
    expect(sortProjects(list, 'rate', 'desc').map((x) => x.name)).toEqual(['B', 'A']);
    expect(sortProjects(list, 'last', 'asc').map((x) => x.name)).toEqual(['A', 'B']);
  });

  it('sinks no-run projects to the bottom for pass rate in both directions', () => {
    const list = [
      p({ projectId: 'empty', name: 'Empty', stats: st(0, 0) }),
      p({ projectId: 'low', name: 'Low', stats: st(5, 0.4) }),
      p({ projectId: 'high', name: 'High', stats: st(5, 0.95) }),
    ];
    // Ascending: real rates ascend, the no-run project is last (not first).
    expect(sortProjects(list, 'rate', 'asc').map((x) => x.name)).toEqual(['Low', 'High', 'Empty']);
    // Descending: real rates descend, the no-run project still last.
    expect(sortProjects(list, 'rate', 'desc').map((x) => x.name)).toEqual(['High', 'Low', 'Empty']);
  });

  it('honors the window run count when deciding empty (Summaries)', () => {
    // All-time stats have runs, but the selected window has none for "b".
    const list = [
      p({ projectId: 'a', name: 'A', stats: st(100, 0.3) }),
      p({ projectId: 'b', name: 'B', stats: st(100, 0.99) }),
    ];
    const byWindow = { a: 10, b: 0 };
    // "b" has 0 runs in the window → its window pass rate is N/A → last,
    // even though its all-time successRate is the highest.
    expect(sortProjects(list, 'rate', 'asc', byWindow).map((x) => x.name)).toEqual(['A', 'B']);
    expect(sortProjects(list, 'rate', 'desc', byWindow).map((x) => x.name)).toEqual(['A', 'B']);
  });

  it('sinks no-activity projects to the bottom for last activity', () => {
    const list = [
      p({ projectId: 'none', name: 'None', stats: st(0, 0, null) }),
      p({ projectId: 'old', name: 'Old', stats: st(3, 0.5, '2026-01-01T00:00:00Z') }),
      p({ projectId: 'new', name: 'New', stats: st(3, 0.5, '2026-03-01T00:00:00Z') }),
    ];
    expect(sortProjects(list, 'last', 'asc').map((x) => x.name)).toEqual(['Old', 'New', 'None']);
    expect(sortProjects(list, 'last', 'desc').map((x) => x.name)).toEqual(['New', 'Old', 'None']);
  });
});
