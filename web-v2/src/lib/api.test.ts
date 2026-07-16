import { describe, it, expect, vi } from 'vitest';
import { restFetch } from './api';

describe('restFetch caching', () => {
  it('sends cache:no-store so API responses are never served from the HTTP cache', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response('{}', { status: 200, headers: { 'content-type': 'application/json' } }));
    vi.stubGlobal('fetch', fetchMock);
    await restFetch('/api/v2/me/saved-views');
    expect(fetchMock).toHaveBeenCalledWith('/api/v2/me/saved-views', expect.objectContaining({ cache: 'no-store' }));
  });
});
