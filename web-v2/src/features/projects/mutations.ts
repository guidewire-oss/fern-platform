import { useMutation, useQueryClient } from '@tanstack/react-query';
import { graphqlFetch } from '@/lib/api';

export interface CreateProjectInput {
  projectId: string;
  name: string;
  description?: string | undefined;
  repository?: string | undefined;
  defaultBranch?: string | undefined;
  team?: string | undefined;
}

export interface UpdateProjectInput {
  name?: string | undefined;
  description?: string | undefined;
  repository?: string | undefined;
  defaultBranch?: string | undefined;
  team?: string | undefined;
}

const CREATE = /* GraphQL */ `
  mutation CreateProject($input: CreateProjectInput!) {
    createProject(input: $input) {
      id projectId name
    }
  }
`;

const UPDATE = /* GraphQL */ `
  mutation UpdateProject($id: ID!, $input: UpdateProjectInput!) {
    updateProject(id: $id, input: $input) {
      id projectId name description team defaultBranch repository
    }
  }
`;

const DELETE = /* GraphQL */ `
  mutation DeleteProject($id: ID!) {
    deleteProject(id: $id)
  }
`;

export function useCreateProject() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: CreateProjectInput) => graphqlFetch(CREATE, { input }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['projects'] }),
  });
}

export function useUpdateProject() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, input }: { id: string; input: UpdateProjectInput }) =>
      graphqlFetch(UPDATE, { id, input }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['projects'] }),
  });
}

export function useDeleteProject() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => graphqlFetch(DELETE, { id }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['projects'] }),
  });
}
