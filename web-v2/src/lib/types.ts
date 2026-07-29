// Hand-written API types for now. When the GraphQL Codegen pipeline
// is wired up these get replaced by generated.ts; until then we
// maintain a small surface here so the SPA stays type-safe.

// --- GraphQL: projects query ----------------------------------------

export interface ProjectStats {
  totalTestRuns: number;
  successRate: number;
  averageDuration: number;
  lastRunTime: string | null;
}

export interface Project {
  id: string;
  projectId: string;
  name: string;
  description: string;
  isActive: boolean;
  team: string;
  canManage: boolean;
  stats: ProjectStats | null;
}

export interface ProjectsConnection {
  totalCount: number;
  edges: { cursor?: string; node: Project }[];
  pageInfo?: {
    hasNextPage: boolean;
    endCursor: string | null;
  };
}

// --- REST v2: GET /api/v2/test-runs ---------------------------------

export interface TestRunNode {
  id: number;
  run_id: string;
  project_id: string;
  // Display name resolved from project_details. Optional: absent on an
  // older backend, empty when the project has no name on record — in
  // both cases the UI falls back to project_id.
  project_name?: string;
  branch: string;
  git_branch: string;
  git_commit: string;
  status: string;
  start_time: string;
  end_time: string | null;
  total_tests: number;
  passed_tests: number;
  failed_tests: number;
  skipped_tests: number;
  // Wall-clock run time as recorded by the server. Preferred over
  // end_time - start_time, which cannot be computed for a run with no
  // end_time. Optional: absent on an older backend.
  duration_ms?: number;
  environment: string;
}

export interface TestRunEdge {
  cursor: string;
  node: TestRunNode;
}

export interface FacetCount {
  value: string;
  count: number;
  // Human-readable rendering of `value`, sent only for the project
  // facet. `value` stays the filterable id in every case.
  label?: string;
}

export interface TestRunFacets {
  byStatus: FacetCount[];
  byBranch: FacetCount[];
  byTag: FacetCount[];
  byProject: FacetCount[];
}

export interface TestRunConnection {
  edges: TestRunEdge[];
  pageInfo: { hasNextPage: boolean; endCursor: string };
  totalCount: number;
  totalCountIsEstimate: boolean;
  facets: TestRunFacets;
}

// --- REST v2: saved views -------------------------------------------

export interface SavedView {
  id: number;
  page: string;
  name: string;
  filter: Record<string, unknown>;
}

export interface SavedViewList {
  views: SavedView[];
  totalCount: number;
  limit: number;
  offset: number;
}
