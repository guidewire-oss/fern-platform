// Shared hooks for saving / listing test-runs saved views. Imported
// by the saved-views management page AND the test-runs page (for the
// inline 'Save view' / 'Views ▾' affordances).

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { restFetch } from '@/lib/api';
import type { SavedView, SavedViewList } from '@/lib/types';

export const TEST_RUNS_PAGE = 'test-runs';
const QK = ['saved-views', TEST_RUNS_PAGE] as const;

export function useTestRunsSavedViews() {
  return useQuery({
    queryKey: QK,
    queryFn: () =>
      restFetch<SavedViewList>(
        `/api/v2/me/saved-views?page=${encodeURIComponent(TEST_RUNS_PAGE)}`,
      ),
    staleTime: 30_000,
  });
}

export function useCreateSavedView() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: { name: string; filter: Record<string, unknown> }) =>
      restFetch<SavedView>('/api/v2/me/saved-views', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          page: TEST_RUNS_PAGE,
          name: body.name,
          filter: body.filter,
        }),
      }),
    onSuccess: () => qc.invalidateQueries({ queryKey: QK }),
  });
}

export function useDeleteSavedView() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: number) =>
      restFetch(`/api/v2/me/saved-views/${id}`, { method: 'DELETE' }),
    onSuccess: () => qc.invalidateQueries({ queryKey: QK }),
  });
}
