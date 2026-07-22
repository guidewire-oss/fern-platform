import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { graphqlFetch } from '@/lib/api';
import type { FernField, FieldMappingEntry, ReductionStrategy } from './fieldMapping';

// --- GraphQL response shapes (mirror schema.graphql) -----------------

export interface JiraFieldMapping {
  projectId: string;
  entries: FieldMappingEntry[];
  updatedBy?: string | null;
  updatedAt?: string | null;
}

export interface JiraFieldOption {
  id: string;
  name: string;
  custom: boolean;
  multiValue: boolean;
}

const MAPPING_FIELDS = `
  projectId
  updatedBy
  updatedAt
  entries { fernField jiraFieldId jiraFieldIsMultiValue reductionStrategy }
`;

const mappingKey = (projectId: string) => ['jira-field-mapping', projectId];
const fieldsKey = (connectionId: string) => ['jira-fields', connectionId];

export function useJiraFieldMapping(projectId: string) {
  return useQuery({
    queryKey: mappingKey(projectId),
    queryFn: () =>
      graphqlFetch<{ jiraFieldMapping: JiraFieldMapping }>(
        `query FieldMapping($p: String!) { jiraFieldMapping(projectId: $p) { ${MAPPING_FIELDS} } }`,
        { p: projectId },
      ).then((d) => d.jiraFieldMapping),
    enabled: !!projectId,
    staleTime: 60_000,
  });
}

// Available JIRA fields for the mapping dropdowns. Needs a connection id;
// callers pass '' (and rely on `enabled`) when the project has none.
export function useJiraFields(connectionId: string | undefined) {
  return useQuery({
    queryKey: fieldsKey(connectionId ?? ''),
    queryFn: () =>
      graphqlFetch<{ jiraFields: JiraFieldOption[] }>(
        `query JiraFields($c: ID!) { jiraFields(connectionId: $c) { id name custom multiValue } }`,
        { c: connectionId },
      ).then((d) => d.jiraFields),
    enabled: !!connectionId,
    staleTime: 300_000,
  });
}

export interface SaveMappingInput {
  fernField: FernField;
  jiraFieldId: string;
  jiraFieldIsMultiValue: boolean;
  reductionStrategy?: ReductionStrategy | null;
}

export function useSaveJiraFieldMapping(projectId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (entries: SaveMappingInput[]) =>
      graphqlFetch<{ saveJiraFieldMapping: JiraFieldMapping }>(
        `mutation SaveMapping($input: SaveJiraFieldMappingInput!) {
           saveJiraFieldMapping(input: $input) { ${MAPPING_FIELDS} }
         }`,
        { input: { projectId, entries } },
      ).then((d) => d.saveJiraFieldMapping),
    onSuccess: (data) => {
      qc.setQueryData(mappingKey(projectId), data);
      qc.invalidateQueries({ queryKey: mappingKey(projectId) });
    },
  });
}

export function useResetJiraFieldMapping(projectId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () =>
      graphqlFetch<{ resetJiraFieldMapping: JiraFieldMapping }>(
        `mutation ResetMapping($p: String!) { resetJiraFieldMapping(projectId: $p) { ${MAPPING_FIELDS} } }`,
        { p: projectId },
      ).then((d) => d.resetJiraFieldMapping),
    onSuccess: (data) => {
      qc.setQueryData(mappingKey(projectId), data);
      qc.invalidateQueries({ queryKey: mappingKey(projectId) });
    },
  });
}
