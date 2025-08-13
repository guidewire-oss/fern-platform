// GraphQL client for Fern Platform

class GraphQLClient {
    constructor(endpoint = '/query') {
        this.endpoint = endpoint;
    }

    async query(queryOrOptions, variables = {}) {
        // Handle both string queries and options objects
        let queryString, queryVariables;
        
        if (typeof queryOrOptions === 'string') {
            queryString = queryOrOptions;
            queryVariables = variables;
        } else if (typeof queryOrOptions === 'object' && queryOrOptions.query) {
            queryString = queryOrOptions.query;
            queryVariables = queryOrOptions.variables || {};
        } else {
            throw new Error('Invalid query format');
        }
        
        const endpoint = this.endpoint;
        
        const response = await fetch(endpoint, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'Accept': 'application/json',
            },
            credentials: 'include',
            body: JSON.stringify({
                query: queryString,
                variables: queryVariables
            })
        });

        if (!response.ok) {
            if (response.status === 401) {
                // Redirect to login
                window.location.href = '/auth/login';
                return;
            }
            // Try to get error details
            const errorText = await response.text();
            console.error(`GraphQL request failed with status ${response.status}:`, errorText);
            throw new Error(`GraphQL request failed: ${response.status}`);
        }

        const result = await response.json();
        
        if (result.errors) {
            console.error('GraphQL errors:', result.errors);
            throw new Error(result.errors[0].message);
        }

        return result.data;
    }

    async mutation(mutation, variables = {}) {
        return this.query(mutation, variables);
    }
}

// Export queries
const QUERIES = {
    GET_DASHBOARD_DATA: `
        query GetDashboardData {
            dashboardSummary {
                health {
                    status
                    service
                    timestamp
                }
                projectCount
                activeProjectCount
                totalTestRuns
                recentTestRuns
                overallPassRate
                totalTestsExecuted
                averageTestDuration
            }
            
            projects(first: 100) {
                edges {
                    node {
                        id
                        projectId
                        name
                        description
                        isActive
                        team
                        canManage
                        stats {
                            totalTestRuns
                            successRate
                            averageDuration
                            lastRunTime
                        }
                    }
                }
                totalCount
            }
            
            recentTestRuns(limit: 10) {
                id
                runId
                projectId
                branch
                status
                startTime
                duration
                totalTests
                passedTests
                failedTests
                skippedTests
                suiteRuns {
                    id
                    suiteName
                    status
                    totalSpecs
                    passedSpecs
                    failedSpecs
                    skippedSpecs
                    duration
                    specRuns {
                        id
                        specName
                        status
                        duration
                        startTime
                        endTime
                        errorMessage
                        stackTrace
                        isFlaky
                    }
                }
            }
        }
    `,

    GET_TREEMAP_DATA: `
        query GetTreemapData($projectId: String, $days: Int) {
            treemapData(projectId: $projectId, days: $days) {
                projects {
                    project {
                        id
                        projectId
                        name
                    }
                    suites {
                        suite {
                            id
                            suiteName
                            status
                        }
                        totalDuration
                        totalSpecs
                        passRate
                    }
                    totalDuration
                    totalTests
                    passRate
                    totalRuns
                }
                totalDuration
                totalTests
                overallPassRate
            }
        }
    `,

    GET_CURRENT_USER: `
        query GetCurrentUser {
            currentUser {
                id
                email
                name
                firstName
                lastName
                role
                profileUrl
                groups
            }
        }
    `,

    GET_TEST_RUN_DETAILS: `
        query GetTestRunDetails($runId: String!) {
            testRunByRunId(runId: $runId) {
                id
                runId
                projectId
                branch
                commitSha
                status
                startTime
                endTime
                totalTests
                passedTests
                failedTests
                skippedTests
                duration
                environment
                metadata
                suiteRuns {
                    id
                    suiteName
                    status
                    totalSpecs
                    passedSpecs
                    failedSpecs
                    skippedSpecs
                    duration
                    specRuns {
                        id
                        specName
                        status
                        duration
                        startTime
                        endTime
                        errorMessage
                        stackTrace
                        isFlaky
                    }
                }
            }
        }
    `,

    GET_PROJECT_DETAILS: `
        query GetProjectDetails($projectId: String!) {
            projectByProjectId(projectId: $projectId) {
                id
                projectId
                name
                description
                repository
                defaultBranch
                isActive
                team
                canManage
                stats {
                    totalTestRuns
                    recentTestRuns
                    uniqueBranches
                    successRate
                    averageDuration
                    lastRunTime
                }
                createdAt
                updatedAt
            }
        }
    `,

    CREATE_PROJECT: `
        mutation CreateProject($input: CreateProjectInput!) {
            createProject(input: $input) {
                id
                projectId
                name
                description
                team
                isActive
            }
        }
    `,

    UPDATE_PROJECT: `
        mutation UpdateProject($id: ID!, $input: UpdateProjectInput!) {
            updateProject(id: $id, input: $input) {
                id
                projectId
                name
                description
                team
                isActive
            }
        }
    `,

    DELETE_PROJECT: `
        mutation DeleteProject($id: ID!) {
            deleteProject(id: $id)
        }
    `,
    
    GET_USER_PREFERENCES: `
        query GetUserPreferences {
            userPreferences {
                id
                userId
                theme
                timezone
                language
                favorites
                preferences
            }
        }
    `,
    
    UPDATE_USER_PREFERENCES: `
        mutation UpdateUserPreferences($input: UpdateUserPreferencesInput!) {
            updateUserPreferences(input: $input) {
                id
                userId
                theme
                timezone
                language
                favorites
                preferences
            }
        }
    `,
    
    TOGGLE_PROJECT_FAVORITE: `
        mutation ToggleProjectFavorite($projectId: String!) {
            toggleProjectFavorite(projectId: $projectId) {
                id
                userId
                favorites
            }
        }
    `,
    
    // JIRA Connection queries
    GET_JIRA_CONNECTIONS: `
        query GetJiraConnections($projectId: String!) {
            jiraConnections(projectId: $projectId) {
                id
                projectId
                name
                jiraUrl
                authenticationType
                projectKey
                username
                status
                isActive
                lastTestedAt
                createdAt
                updatedAt
            }
        }
    `,
    
    CREATE_JIRA_CONNECTION: `
        mutation CreateJiraConnection($input: CreateJiraConnectionInput!) {
            createJiraConnection(input: $input) {
                id
                projectId
                name
                jiraUrl
                authenticationType
                projectKey
                username
                status
                isActive
                lastTestedAt
                createdAt
                updatedAt
            }
        }
    `,
    
    UPDATE_JIRA_CONNECTION: `
        mutation UpdateJiraConnection($id: ID!, $input: UpdateJiraConnectionInput!) {
            updateJiraConnection(id: $id, input: $input) {
                id
                projectId
                name
                jiraUrl
                authenticationType
                projectKey
                username
                status
                isActive
                lastTestedAt
                createdAt
                updatedAt
            }
        }
    `,
    
    UPDATE_JIRA_CREDENTIALS: `
        mutation UpdateJiraCredentials($id: ID!, $input: UpdateJiraCredentialsInput!) {
            updateJiraCredentials(id: $id, input: $input) {
                id
                projectId
                name
                jiraUrl
                authenticationType
                projectKey
                username
                status
                isActive
                lastTestedAt
                createdAt
                updatedAt
            }
        }
    `,
    
    TEST_JIRA_CONNECTION: `
        mutation TestJiraConnection($id: ID!) {
            testJiraConnection(id: $id)
        }
    `,
    
    DELETE_JIRA_CONNECTION: `
        mutation DeleteJiraConnection($id: ID!) {
            deleteJiraConnection(id: $id)
        }
    `
};

// Export singleton instance
const graphqlClient = new GraphQLClient();

// Make it available globally for the existing code
window.graphqlClient = graphqlClient;
window.GRAPHQL_QUERIES = QUERIES;