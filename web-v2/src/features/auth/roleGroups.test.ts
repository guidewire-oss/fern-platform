import { describe, it, expect } from 'vitest';
import { deriveUserTeams, isAdminUser, DEFAULT_ROLE_GROUPS } from './roleGroups';

const RG = { adminGroup: 'admin', managerGroup: 'manager', userGroup: 'user' };

describe('deriveUserTeams', () => {
  it('returns groups minus the three role groups', () => {
    expect(deriveUserTeams(['admin', 'manager', 'user', 'team-a', 'team-b'], RG)).toEqual([
      'team-a',
      'team-b',
    ]);
  });

  it('preserves order and non-role groups only', () => {
    expect(deriveUserTeams(['team-x', 'user', 'team-y'], RG)).toEqual(['team-x', 'team-y']);
  });

  it('returns [] for null/empty groups', () => {
    expect(deriveUserTeams(null, RG)).toEqual([]);
    expect(deriveUserTeams(undefined, RG)).toEqual([]);
    expect(deriveUserTeams([], RG)).toEqual([]);
  });

  it('returns [] when roleGroups is unavailable (cannot safely filter)', () => {
    expect(deriveUserTeams(['team-a'], null)).toEqual([]);
  });

  it('uses the provided role-group names, not hardcoded ones', () => {
    const custom = { adminGroup: 'g-admin', managerGroup: 'g-mgr', userGroup: 'g-user' };
    expect(deriveUserTeams(['g-admin', 'g-user', 'squad-1'], custom)).toEqual(['squad-1']);
  });
});

describe('isAdminUser', () => {
  it('is true when role is admin', () => {
    expect(isAdminUser({ role: 'admin', groups: [] }, RG)).toBe(true);
  });

  it('is true when groups include the admin group', () => {
    expect(isAdminUser({ role: 'user', groups: ['team-a', 'admin'] }, RG)).toBe(true);
  });

  it('is false for a plain user', () => {
    expect(isAdminUser({ role: 'user', groups: ['team-a'] }, RG)).toBe(false);
  });

  it('is false for null user or null roleGroups', () => {
    expect(isAdminUser(null, RG)).toBe(false);
    expect(isAdminUser({ role: 'user', groups: ['admin'] }, null)).toBe(false);
  });
});

describe('DEFAULT_ROLE_GROUPS', () => {
  it('matches the v1 fallback', () => {
    expect(DEFAULT_ROLE_GROUPS).toEqual({
      adminGroup: 'admin',
      managerGroup: 'manager',
      userGroup: 'user',
    });
  });
});
