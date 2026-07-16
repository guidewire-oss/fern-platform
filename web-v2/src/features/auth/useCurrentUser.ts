import { useQuery } from '@tanstack/react-query';
import { graphqlFetch } from '@/lib/api';
import { ApiError } from '@/lib/api';

// Minimal shape needed by the TopBar / AuthGate. Profile.tsx queries a
// wider projection — kept separate so the page-chrome request stays
// small and cacheable.
export interface CurrentUser {
  id: string;
  userId: string;
  email: string;
  name: string;
  firstName?: string;
  lastName?: string;
  role: string;
  profileUrl?: string;
  groups?: string[];
}

const GET_ME = /* GraphQL */ `
  query TopBarMe {
    currentUser {
      id
      userId
      email
      name
      firstName
      lastName
      role
      profileUrl
      groups
    }
  }
`;

interface MeResp {
  currentUser: CurrentUser | null;
}

export function useCurrentUser() {
  return useQuery({
    queryKey: ['current-user'],
    queryFn: async () => {
      try {
        const data = await graphqlFetch<MeResp>(GET_ME);
        return data.currentUser;
      } catch (e) {
        // 401 is expected before SSO — treat as "no user", not as a hard
        // error, so the chrome can show a Sign-in link instead of red.
        if (e instanceof ApiError && (e.status === 401 || e.status === 403)) {
          return null;
        }
        throw e;
      }
    },
    staleTime: 60_000,
    retry: false,
  });
}

export function initialsOf(u: Pick<CurrentUser, 'name' | 'firstName' | 'lastName' | 'email'>): string {
  const pickInitial = (s: string | undefined) => (s ? s.trim().charAt(0).toUpperCase() : '');
  if (u.firstName || u.lastName) {
    return (pickInitial(u.firstName) + pickInitial(u.lastName)) || '?';
  }
  if (u.name) {
    const parts = u.name.trim().split(/\s+/);
    if (parts.length >= 2) return (pickInitial(parts[0]) + pickInitial(parts[parts.length - 1])) || '?';
    return pickInitial(parts[0]) || '?';
  }
  return pickInitial(u.email) || '?';
}
