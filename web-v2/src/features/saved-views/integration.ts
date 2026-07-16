// Shared hooks for saving / listing saved views. Imported by the
// saved-views management page AND the test-runs page (for the inline
// 'Save view' / 'Views ▾' affordances).

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { ApiError, restFetch } from '@/lib/api';
import type { SavedView, SavedViewList } from '@/lib/types';

export const TEST_RUNS_PAGE = 'test-runs';

// ONE query holds all of the user's saved views. Both the manage page and
// the test-runs Views bar read from this single key (the bar filters to
// its page client-side). A single key means one fetch and one
// invalidation that updates every surface at once — the previous
// two-query design (a page-scoped ['saved-views','test-runs'] alongside
// ['saved-views']) left the bar stale because invalidating one key didn't
// reliably reach the other.
export const SAVED_VIEWS_ROOT = ['saved-views'] as const;

function invalidateSavedViews(qc: ReturnType<typeof useQueryClient>) {
  return qc.invalidateQueries({ queryKey: SAVED_VIEWS_ROOT, exact: true, refetchType: 'all' });
}

function updateSavedViews(
  qc: ReturnType<typeof useQueryClient>,
  fn: (list: SavedViewList) => SavedViewList,
) {
  qc.setQueryData<SavedViewList>(SAVED_VIEWS_ROOT, (old) => (old ? fn(old) : old));
}

// deleteSavedView is idempotent: a 404 means the row is already gone
// (deleted in another tab, or a double-click after a successful delete),
// which is success for the caller.
export async function deleteSavedView(id: number): Promise<void> {
  try {
    await restFetch(`/api/v2/me/saved-views/${id}`, { method: 'DELETE' });
  } catch (err) {
    if (err instanceof ApiError && err.status === 404) return;
    throw err;
  }
}

// useSavedViews fetches every saved view for the current user (all pages).
// Consumers filter by page as needed.
export function useSavedViews() {
  return useQuery({
    queryKey: SAVED_VIEWS_ROOT,
    queryFn: () => restFetch<SavedViewList>('/api/v2/me/saved-views'),
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
    onSuccess: (created) => {
      // Insert immediately so every surface shows it at once, then
      // reconcile with the server (ordering, other tabs).
      updateSavedViews(qc, (list) =>
        list.views.some((v) => v.id === created.id)
          ? list
          : { ...list, views: [...list.views, created], totalCount: list.totalCount + 1 },
      );
      void invalidateSavedViews(qc);
    },
  });
}

export function useDeleteSavedView() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: deleteSavedView,
    // Mirror the create path (which reliably updates the UI): drop the row
    // from the cache in onSuccess, then reconcile. deleteSavedView swallows
    // 404, so onSuccess also runs when the row was already gone — the
    // filter is then a harmless no-op. (The earlier onMutate/cancelQueries
    // approach left the row on screen and skipped the refetch.)
    onSuccess: (_data, id) => {
      updateSavedViews(qc, (list) => ({
        ...list,
        views: list.views.filter((v) => v.id !== id),
        totalCount: Math.max(0, list.totalCount - 1),
      }));
      void invalidateSavedViews(qc);
    },
  });
}
