# Debugging Test Failures

**Use Fern Platform to understand why tests fail and track failure patterns**

> 📌 **Current Capabilities**: Fern Platform provides basic test failure tracking and visualization. Advanced failure analysis and root cause detection are planned for future releases.

## What You Can Do Today

### 1. View Test Failure Details

When a test fails, Fern Platform captures:
- **Error messages** - The actual error that caused the failure
- **Stack traces** - Full stack trace when available
- **Test metadata** - Duration, retry count, test location
- **Historical context** - Previous runs of the same test

### 2. Track Failure Patterns

The platform helps you identify:
- **Failure frequency** - How often a test fails over time
- **Failure timing** - When failures occur (by commit, time of day)
- **Related failures** - Other tests that failed in the same run

### 3. Navigate Failure Data

#### From the Dashboard
1. Click on any failed test run (shown in red)
2. Drill down to see all failed tests in that run
3. Click on a specific test to see error details

#### From Test History
1. Search for a specific test by name
2. View its complete history
3. Filter to show only failed runs
4. Compare error messages across failures

## Accessing Failure Data

### Via Web UI

Navigate to a failed test to see:

```
Test: should process payment successfully
Status: FAILED
Duration: 1.2s
Error: Connection timeout to payment service
Stack: 
  at PaymentService.process (payment.js:45)
  at test (payment.spec.js:23)
```

### Via REST API

Get detailed failure information:

```bash
# Get test run with failure details
curl -X GET http://fern-platform.local:8080/api/v1/test-runs/{runId} \
  -H "Authorization: Bearer $TOKEN"

# Response includes:
{
  "id": "run-123",
  "status": "failed",
  "failedTests": 3,
  "suites": [{
    "name": "Payment Suite",
    "specs": [{
      "name": "should process payment successfully",
      "status": "failed",
      "error": "Connection timeout to payment service",
      "stackTrace": "at PaymentService.process...",
      "duration": 1200
    }]
  }]
}
```

### Via GraphQL

Query for failure patterns:

```graphql
query GetFailureDetails($projectId: ID!, $status: TestStatus!) {
  testRuns(projectId: $projectId, status: $status) {
    runs {
      id
      gitCommit
      gitBranch
      status
      suites {
        name
        specs {
          name
          status
          error
          stackTrace
          duration
        }
      }
    }
  }
}
```

## Understanding Failure Context

### Timeline View
1. Go to Test Runs page
2. Filter by "Failed" status
3. Sort by date to see failure progression
4. Look for patterns in timing

### Cross-Reference with Changes
- Note the git commit of failed runs
- Check if failures correlate with specific commits
- Look for environment-specific failures (branch patterns)

## Current Limitations

Fern Platform currently provides basic failure tracking. It does **not** yet support:

- ❌ Intelligent failure grouping (similar errors grouped together)
- ❌ Root cause analysis
- ❌ Failure predictions
- ❌ Auto-correlation with code changes
- ❌ Screenshot or video capture
- ❌ Log aggregation from test runs

## Tips for Effective Debugging

1. **Use meaningful test names** - Makes failures easier to find and understand
2. **Include context in errors** - Your test framework should provide detailed error messages
3. **Track flaky tests** - Use the [flaky test detection](./finding-flaky-tests.md) to identify unreliable tests
4. **Monitor trends** - Regular failures might indicate systemic issues

## API Integration Example

Send rich failure data from your test runner:

```javascript
// Example: Jest reporter
class FernReporter {
  onTestResult(test, testResult) {
    if (testResult.status === 'failed') {
      const failure = {
        projectId: process.env.PROJECT_ID,
        suite: testResult.testFilePath,
        spec: testResult.title,
        status: 'failed',
        error: testResult.failureMessage,
        stackTrace: testResult.failureDetails,
        duration: testResult.duration
      };
      
      // Send to Fern Platform
      fetch(`${FERN_URL}/api/v1/test-runs`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(failure)
      });
    }
  }
}
```

## Future Enhancements

We're planning to add:
- 🎯 Smart failure grouping using ML
- 🔍 Root cause analysis
- 📸 Screenshot/video capture integration
- 📊 Failure prediction based on code changes
- 🔗 Integration with error tracking tools

See our [RFCs](../rfc/) for detailed plans on AI-powered failure analysis.

## Related Documentation

- [Finding Flaky Tests](./finding-flaky-tests.md) - Identify unreliable tests
- [API Reference](../developers/api-reference.md) - Complete API documentation
- [Integration Guide](../developers/integration-guide.md) - Connect your CI/CD pipeline