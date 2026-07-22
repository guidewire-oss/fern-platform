import { describe, it, expect, vi, beforeEach } from 'vitest';
import { ApiError } from '@/lib/api';

// Mock the api module so deleteSavedView exercises its error handling
// against controllable restFetch outcomes.
const restFetch = vi.fn();
vi.mock('@/lib/api', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/lib/api')>();
  return { ...actual, restFetch: (...args: unknown[]) => restFetch(...args) };
});

import { deleteSavedView } from './integration';

describe('deleteSavedView', () => {
  beforeEach(() => restFetch.mockReset());

  it('resolves on a normal delete', async () => {
    restFetch.mockResolvedValueOnce('');
    await expect(deleteSavedView(1)).resolves.toBeUndefined();
    expect(restFetch).toHaveBeenCalledWith('/api/v2/me/saved-views/1', { method: 'DELETE' });
  });

  it('treats 404 (already gone) as success — idempotent', async () => {
    restFetch.mockRejectedValueOnce(new ApiError(404, 'view not found', null));
    await expect(deleteSavedView(35)).resolves.toBeUndefined();
  });

  it('rethrows non-404 errors', async () => {
    restFetch.mockRejectedValueOnce(new ApiError(500, 'delete failed', null));
    await expect(deleteSavedView(2)).rejects.toBeInstanceOf(ApiError);
  });
});
