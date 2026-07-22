import { useQuery } from '@tanstack/react-query';
import { graphqlFetch } from '@/lib/api';
import type { JiraRelease, RequirementCoverageTree } from './coverage';

const STORY_FIELDS = `
  issue { key summary statusName issueType }
  covered
  testRunCoverage { total passed failed skipped lastRunAt }
`;

// GraphQL can't express unbounded recursion, so sub-tasks are fetched to
// a fixed depth. Two levels covers the epic → story → sub-task hierarchy
// the resolver produces; deeper levels simply arrive as empty arrays.
const STORY_SELECTION = `
  ${STORY_FIELDS}
  subTasks {
    ${STORY_FIELDS}
    subTasks { ${STORY_FIELDS} }
  }
`;

const COVERAGE_QUERY = `
  query RequirementCoverage($p: ID!, $v: String!) {
    requirementCoverage(projectId: $p, fixVersionName: $v) {
      fixVersion { id name released releaseDate }
      epics {
        issue { key summary statusName issueType }
        coveredCount
        totalCount
        stories { ${STORY_SELECTION} }
      }
      unassigned { ${STORY_SELECTION} }
    }
  }
`;

export function useJiraFixVersions(projectId: string) {
  return useQuery({
    queryKey: ['jira-fix-versions', projectId],
    queryFn: () =>
      graphqlFetch<{ jiraFixVersions: JiraRelease[] }>(
        `query FixVersions($p: ID!) { jiraFixVersions(projectId: $p) { id name released releaseDate } }`,
        { p: projectId },
      ).then((d) => d.jiraFixVersions),
    enabled: !!projectId,
    staleTime: 300_000,
  });
}

export function useRequirementCoverage(projectId: string, fixVersionName: string | undefined) {
  return useQuery({
    queryKey: ['requirement-coverage', projectId, fixVersionName],
    queryFn: () =>
      graphqlFetch<{ requirementCoverage: RequirementCoverageTree }>(COVERAGE_QUERY, {
        p: projectId,
        v: fixVersionName,
      }).then((d) => d.requirementCoverage),
    enabled: !!projectId && !!fixVersionName,
    staleTime: 60_000,
  });
}
