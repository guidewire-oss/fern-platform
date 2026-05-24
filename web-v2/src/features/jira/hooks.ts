import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { restFetch } from '@/lib/api';

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
}

interface ListResp {
  connections: JiraConnection[];
}

const qk = (projectId: string) => ['jira-connections', projectId];

export function useJiraConnections(projectId: string) {
  return useQuery({
    queryKey: qk(projectId),
    queryFn: () =>
      restFetch<ListResp>(
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

export function useTestJiraConnection(projectId: string) {
  return useMutation({
    mutationFn: (connectionId: string) =>
      restFetch<{ success: boolean; message?: string }>(
        `/api/v1/projects/${encodeURIComponent(projectId)}/integrations/jira/connections/${encodeURIComponent(connectionId)}/test`,
        { method: 'POST' },
      ),
  });
}

export function useDeleteJiraConnection(projectId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (connectionId: string) =>
      restFetch(
        `/api/v1/projects/${encodeURIComponent(projectId)}/integrations/jira/connections/${encodeURIComponent(connectionId)}`,
        { method: 'DELETE' },
      ),
    onSuccess: () => qc.invalidateQueries({ queryKey: qk(projectId) }),
  });
}
