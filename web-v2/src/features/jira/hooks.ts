import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { restFetch, ApiError } from '@/lib/api';

export interface JiraConnection {
  id: string;
  projectId: string;
  name: string;
  jiraUrl: string;
  authenticationType: 'api_token' | 'oauth' | 'personal_access_token';
  projectKey: string;
  username: string;
  status: 'pending' | 'connected' | 'failed';
  isActive: boolean;
  lastTestedAt?: string;
  versionFilter?: string;
}

const qk = (projectId: string) => ['jira-connections', projectId];

// The REST handler returns a bare JSON array of connections
// (c.JSON(200, []JiraConnectionResponse)), NOT a { connections: [...] }
// envelope — consume it as an array.
export function useJiraConnections(projectId: string) {
  return useQuery({
    queryKey: qk(projectId),
    queryFn: () =>
      restFetch<JiraConnection[]>(
        `/api/v1/projects/${encodeURIComponent(projectId)}/integrations/jira/connections`,
      ),
    enabled: !!projectId,
    staleTime: 60_000,
  });
}

export interface NewJiraConnection {
  name: string;
  jiraUrl: string;
  authenticationType: JiraConnection['authenticationType'];
  projectKey: string;
  username: string;
  credential: string;
}

export function useCreateJiraConnection(projectId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: NewJiraConnection) =>
      restFetch(
        `/api/v1/projects/${encodeURIComponent(projectId)}/integrations/jira/connections`,
        {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(input),
        },
      ),
    onSuccess: () => qc.invalidateQueries({ queryKey: qk(projectId) }),
  });
}

export interface UpdateJiraConnectionInput {
  name: string;
  jiraUrl: string;
  projectKey: string;
  versionFilter: string;
}

// Updates the non-secret fields (PUT …/connections/:id).
export function useUpdateJiraConnection(projectId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, input }: { id: string; input: UpdateJiraConnectionInput }) =>
      restFetch(
        `/api/v1/projects/${encodeURIComponent(projectId)}/integrations/jira/connections/${encodeURIComponent(id)}`,
        { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(input) },
      ),
    onSettled: () => qc.invalidateQueries({ queryKey: qk(projectId) }),
  });
}

export interface UpdateJiraCredentialsInput {
  authenticationType: JiraConnection['authenticationType'];
  username: string;
  credential: string;
}

// Rotates the credential (PUT …/connections/:id/credentials). Separate
// from the field update because the server keeps the secret write-only.
export function useUpdateJiraCredentials(projectId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, input }: { id: string; input: UpdateJiraCredentialsInput }) =>
      restFetch(
        `/api/v1/projects/${encodeURIComponent(projectId)}/integrations/jira/connections/${encodeURIComponent(id)}/credentials`,
        { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(input) },
      ),
    onSettled: () => qc.invalidateQueries({ queryKey: qk(projectId) }),
  });
}

export function useTestJiraConnection(projectId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (connectionId: string) => {
      // The endpoint signals success with HTTP 200 (body: { message }) and
      // failure with a 4xx that restFetch throws. So a resolved call means
      // the test passed — synthesize the success flag the UI renders on
      // (the response has no `success` field).
      const resp = await restFetch<{ message?: string }>(
        `/api/v1/projects/${encodeURIComponent(projectId)}/integrations/jira/connections/${encodeURIComponent(connectionId)}/test`,
        { method: 'POST' },
      );
      return { success: true, message: resp?.message ?? 'Connection test successful' };
    },
    // A test updates the stored status / last_tested_at, so refresh the
    // list to reflect the new "connected" badge.
    onSettled: () => qc.invalidateQueries({ queryKey: qk(projectId) }),
  });
}

export function useDeleteJiraConnection(projectId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (connectionId: string) => {
      try {
        await restFetch(
          `/api/v1/projects/${encodeURIComponent(projectId)}/integrations/jira/connections/${encodeURIComponent(connectionId)}`,
          { method: 'DELETE' },
        );
      } catch (e) {
        // Delete is idempotent: a 404 means the connection is already
        // gone (e.g. a double-click, or the list was stale) — that's the
        // desired end state, so treat it as success rather than surfacing
        // a scary error and leaving the row on screen.
        if (e instanceof ApiError && e.status === 404) return;
        throw e;
      }
    },
    // Always refresh the list, success or error, so a deleted connection
    // leaves the UI even if the mutation resolved oddly.
    onSettled: () => qc.invalidateQueries({ queryKey: qk(projectId) }),
  });
}
