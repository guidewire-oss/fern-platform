import { useQuery } from '@tanstack/react-query';
import { graphqlFetch, ApiError } from '@/lib/api';

// The three special group names that map to platform roles rather than
// teams. Everything in a user's `groups` that is NOT one of these is a
// team (see deriveUserTeams). Names are configurable server-side and
// surfaced via SystemConfig.roleGroups.
export interface RoleGroups {
  adminGroup: string;
  managerGroup: string;
  userGroup: string;
}

// v1 fallback (web/index.html): used when SystemConfig is unavailable so
// team derivation still works offline / before the config loads.
export const DEFAULT_ROLE_GROUPS: RoleGroups = {
  adminGroup: 'admin',
  managerGroup: 'manager',
  userGroup: 'user',
};

// A user's teams are their groups minus the role groups — mirrors v1's
// filter (web/index.html:2326-2331). Returns [] when we cannot safely
// filter (no roleGroups) so role groups are never shown as teams.
export function deriveUserTeams(
  groups: readonly string[] | null | undefined,
  roleGroups: RoleGroups | null | undefined,
): string[] {
  if (!roleGroups) return [];
  const roles = new Set([roleGroups.adminGroup, roleGroups.managerGroup, roleGroups.userGroup]);
  return (groups ?? []).filter((g) => !roles.has(g));
}

// Admin when the platform role is admin OR the admin group is present —
// mirrors v1 (web/index.html:2321-2323).
export function isAdminUser(
  user: { role?: string; groups?: readonly string[] | null } | null | undefined,
  roleGroups: RoleGroups | null | undefined,
): boolean {
  if (!user) return false;
  if (user.role === 'admin') return true;
  const admin = roleGroups?.adminGroup;
  return !!admin && !!user.groups?.includes(admin);
}

const GET_ROLE_GROUPS = /* GraphQL */ `
  query RoleGroups {
    systemConfig {
      roleGroups {
        adminGroup
        managerGroup
        userGroup
      }
    }
  }
`;

interface RoleGroupsResp {
  systemConfig: { roleGroups: RoleGroups };
}

// Fetches the configured role-group names. On 401/403 (signed-out) it
// resolves to the v1 defaults rather than erroring, so the create form
// can still derive teams.
export function useRoleGroups() {
  return useQuery({
    queryKey: ['role-groups'],
    queryFn: async () => {
      try {
        const data = await graphqlFetch<RoleGroupsResp>(GET_ROLE_GROUPS);
        return data.systemConfig.roleGroups;
      } catch (e) {
        if (e instanceof ApiError && (e.status === 401 || e.status === 403)) {
          return DEFAULT_ROLE_GROUPS;
        }
        throw e;
      }
    },
    staleTime: 300_000,
    retry: false,
  });
}
