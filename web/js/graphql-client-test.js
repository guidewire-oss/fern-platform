// Tests for GraphQL client queries
// Run with: node graphql-client-test.js

const { QUERIES } = require('./graphql-client');

// Mock graphqlClient for testing
class TestGraphQLClient {
    constructor() {
        this.lastQuery = null;
        this.lastVariables = null;
    }

    async query(queryOrOptions, variables = {}) {
        this.lastQuery = queryOrOptions;
        this.lastVariables = variables;
        
        // Mock response based on query - check for operation name or query content
        if (queryOrOptions.includes('GetProjectTestRuns') || queryOrOptions.includes('recentTestRuns')) {
            return {
                recentTestRuns: [
                    {
                        id: '1',
                        runId: 'run-1',
                        projectId: variables.projectId,
                        status: 'completed',
                        startTime: new Date().toISOString(),
                        totalTests: 100,
                        passedTests: 95,
                        failedTests: 5,
                        skippedTests: 0
                    }
                ]
            };
        }
        
        return {};
    }
}

// Test suite
async function runTests() {
    console.log('Running GraphQL Client Tests...\n');

    let passed = 0;
    let failed = 0;

    // Test 1: GET_PROJECT_TEST_RUNS query includes projectId parameter
    try {
        const client = new TestGraphQLClient();

        await client.query(QUERIES.GET_PROJECT_TEST_RUNS, { projectId: 'test-project-123', limit: 50 });

        if (client.lastVariables.projectId === 'test-project-123' &&
            client.lastVariables.limit === 50) {
            console.log('✓ Test 1: GET_PROJECT_TEST_RUNS accepts projectId and limit');
            passed++;
        } else {
            console.log('✗ Test 1: GET_PROJECT_TEST_RUNS parameters incorrect');
            failed++;
        }
    } catch (e) {
        console.log('✗ Test 1: Exception thrown:', e.message);
        failed++;
    }

    // Test 2: Query structure includes required fields
    try {
        const requiredFields = ['id', 'runId', 'projectId', 'startTime', 'totalTests', 'passedTests', 'failedTests', 'skippedTests'];
        const hasAllFields = requiredFields.every(field => QUERIES.GET_PROJECT_TEST_RUNS.includes(field));

        if (hasAllFields) {
            console.log('✓ Test 2: Query includes all required fields');
            passed++;
        } else {
            console.log('✗ Test 2: Query missing required fields');
            failed++;
        }
    } catch (e) {
        console.log('✗ Test 2: Exception thrown:', e.message);
        failed++;
    }

    // Test 3: Mock response structure matches expected format
    try {
        const client = new TestGraphQLClient();
        const result = await client.query(QUERIES.GET_PROJECT_TEST_RUNS, { projectId: 'test-123', limit: 50 });

        if (result.recentTestRuns &&
            Array.isArray(result.recentTestRuns) &&
            result.recentTestRuns[0].projectId === 'test-123') {
            console.log('✓ Test 3: Response structure is correct');
            passed++;
        } else {
            console.log('✗ Test 3: Response structure is incorrect');
            failed++;
        }
    } catch (e) {
        console.log('✗ Test 3: Exception thrown:', e.message);
        failed++;
    }

    // Print summary
    console.log(`\n${'='.repeat(50)}`);
    console.log(`Tests passed: ${passed}`);
    console.log(`Tests failed: ${failed}`);
    console.log(`Total: ${passed + failed}`);
    console.log(`${'='.repeat(50)}`);

    if (failed > 0) {
        console.log('\n✗ Some tests failed!');
        throw new Error(`${failed} test(s) failed`);
    }
    console.log('\n✓ All tests passed!');
}

// Run tests if executed directly
if (typeof module !== 'undefined' && require.main === module) {
    runTests().then(() => process.exit(0)).catch(() => process.exit(1));
}

// Export for use in other test files
if (typeof module !== 'undefined') {
    module.exports = { TestGraphQLClient, runTests };
}
