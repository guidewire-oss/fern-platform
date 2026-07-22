// Thin fetch helpers for the v2 REST surface and the GraphQL endpoint.
// All requests are same-origin in production. The Vite dev server
// proxies them to :8080 when running standalone (`pnpm dev`).

export class ApiError extends Error {
  status: number;
  body: unknown;
  constructor(status: number, message: string, body: unknown) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.body = body;
  }
}

// Auth-gate: if any request comes back 401 we treat the session as
// expired and bounce the user to /auth/start. The redirect is a
// browser navigation (not a router push) so OAuth picks up where it
// left off. Guarded with a sentinel so concurrent failing requests
// don't all attempt the redirect simultaneously.
//
// The chrome's `currentUser` query catches 401 in its own try/catch
// (see features/auth/useCurrentUser.ts) and returns null instead of
// letting the error propagate here — that's why loading the app
// while signed-out doesn't immediately bounce to the IdP. Every
// *other* protected call hitting 401 means the session went bad
// after sign-in (expired token, server restart, etc.) and a redirect
// to re-auth is the right user experience.
let didRedirectOn401 = false;
function maybeRedirectOn401(status: number) {
  if (status !== 401 || didRedirectOn401) return;
  if (typeof window === 'undefined') return;
  didRedirectOn401 = true;
  window.location.href = '/auth/start';
}

async function parseBody(res: Response): Promise<unknown> {
  const ct = res.headers.get('content-type') ?? '';
  if (ct.includes('application/json')) {
    try {
      return await res.json();
    } catch {
      return null;
    }
  }
  return res.text();
}

export async function restFetch<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    credentials: 'same-origin',
    // Never let the browser's HTTP cache serve an API response. These
    // endpoints carry no Cache-Control, so the browser heuristically
    // disk-caches GETs — which made a refetch after a DELETE return the
    // stale pre-delete list ("200 OK (from disk cache)"), re-adding the
    // just-removed row. A POST to the same URL invalidates that cache
    // entry (so create looked fine) but a DELETE to a sub-path does not,
    // which is why only delete appeared broken. no-store fixes both.
    cache: 'no-store',
    headers: { Accept: 'application/json', ...(init?.headers ?? {}) },
    ...init,
  });
  const body = await parseBody(res);
  if (!res.ok) {
    maybeRedirectOn401(res.status);
    const msg = typeof body === 'object' && body && 'error' in body
      ? String((body as { error: unknown }).error)
      : `HTTP ${res.status}`;
    throw new ApiError(res.status, msg, body);
  }
  return body as T;
}

export async function graphqlFetch<T>(
  query: string,
  variables?: Record<string, unknown>,
): Promise<T> {
  const res = await fetch('/query', {
    method: 'POST',
    credentials: 'same-origin',
    headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
    body: JSON.stringify({ query, variables }),
  });
  const body = (await parseBody(res)) as { data?: T; errors?: Array<{ message: string }> };
  if (!res.ok) {
    maybeRedirectOn401(res.status);
    throw new ApiError(res.status, `GraphQL HTTP ${res.status}`, body);
  }
  if (body.errors?.length) {
    throw new ApiError(200, body.errors.map((e) => e.message).join('; '), body);
  }
  if (!body.data) {
    throw new ApiError(200, 'GraphQL response missing data', body);
  }
  return body.data;
}
