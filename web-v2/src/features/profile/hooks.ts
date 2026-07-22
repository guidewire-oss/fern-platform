import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { graphqlFetch } from '@/lib/api';

const GET_PREFS = /* GraphQL */ `
  query MyPrefs {
    userPreferences {
      theme
      timezone
      language
      favorites
    }
  }
`;

const TOGGLE = /* GraphQL */ `
  mutation Toggle($projectId: String!) {
    toggleProjectFavorite(projectId: $projectId) {
      favorites
    }
  }
`;

interface PrefsResp {
  userPreferences: {
    theme?: string;
    timezone?: string;
    language?: string;
    favorites: string[];
  } | null;
}

const QK = ['user-preferences'];

export function useUserPreferences() {
  return useQuery({
    queryKey: QK,
    queryFn: () => graphqlFetch<PrefsResp>(GET_PREFS),
    staleTime: 60_000,
  });
}

export function useToggleFavorite() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (projectId: string) =>
      graphqlFetch<{ toggleProjectFavorite: { favorites: string[] } }>(
        TOGGLE,
        { projectId },
      ),
    // Optimistically flip the favorite locally so the star animates
    // instantly. Roll back on error.
    onMutate: async (projectId) => {
      await qc.cancelQueries({ queryKey: QK });
      const previous = qc.getQueryData<PrefsResp>(QK);
      if (previous?.userPreferences) {
        const favs = new Set(previous.userPreferences.favorites);
        if (favs.has(projectId)) favs.delete(projectId);
        else favs.add(projectId);
        qc.setQueryData<PrefsResp>(QK, {
          userPreferences: {
            ...previous.userPreferences,
            favorites: Array.from(favs),
          },
        });
      }
      return { previous };
    },
    onError: (_err, _projectId, ctx) => {
      if (ctx?.previous) qc.setQueryData(QK, ctx.previous);
    },
    onSettled: () => qc.invalidateQueries({ queryKey: QK }),
  });
}
