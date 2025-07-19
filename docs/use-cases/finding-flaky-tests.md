# Finding Flaky Tests

**Identify and manage unreliable tests that pass and fail intermittently**

> ✅ **Fully Implemented**: Fern Platform includes comprehensive flaky test detection with configurable thresholds, tracking, and management capabilities.

## Overview

Flaky tests are tests that exhibit non-deterministic behavior - they pass and fail without any code changes. Fern Platform automatically detects these problematic tests using statistical analysis of your test history.

## How Flaky Test Detection Works

### Detection Algorithm

Fern Platform identifies flaky tests based on:

1. **Failure Rate** - Percentage of runs where the test failed
2. **Minimum Runs** - Requires sufficient data before classification
3. **Time Window** - Analyzes recent test behavior

Default thresholds:
- **Failure Rate**: 10-90% (tests that always fail or always pass aren't flaky)
- **Minimum Runs**: 10 executions
- **Detection Window**: Last 30 days

### Flaky Test States

Tests can be in one of these states:

- **🔴 Active** - Currently exhibiting flaky behavior
- **✅ Resolved** - Previously flaky but now stable
- **🔕 Ignored** - Manually marked to ignore flakiness

## Using the Flaky Test Features

### Web UI

#### View All Flaky Tests
1. Navigate to the **Test Analysis** section
2. Click on **Flaky Tests** tab
3. See all currently flaky tests across projects

#### Filter and Sort
- Filter by project, suite, or status
- Sort by failure rate or occurrence count
- Search by test name

#### Flaky Test Details
Click on any flaky test to see:
- Historical pass/fail pattern
- Failure rate percentage
- Recent failure messages
- First and last occurrence

### REST API

#### List Flaky Tests
```bash
GET /api/v1/analytics/flaky-tests?projectId={projectId}

# Response
{
  "flakyTests": [
    {
      "id": "flaky-123",
      "projectId": "my-project",
      "suiteName": "Integration Tests",
      "specName": "should handle concurrent requests",
      "failureRate": 0.23,
      "occurrenceCount": 45,
      "status": "active",
      "firstOccurrence": "2024-01-15T10:00:00Z",
      "lastOccurrence": "2024-01-20T15:30:00Z"
    }
  ]
}
```

#### Get Flaky Test Details
```bash
GET /api/v1/analytics/flaky-tests/{id}

# Includes full history and recent failures
```

#### Update Flaky Test Status
```bash
PUT /api/v1/analytics/flaky-tests/{id}/status

{
  "status": "ignored",
  "reason": "Known environment issue, working on fix"
}
```

### GraphQL Queries

#### Query Flaky Tests with Filtering
```graphql
query GetFlakyTests($projectId: ID!, $status: FlakyTestStatus) {
  flakyTests(projectId: $projectId, status: $status) {
    tests {
      id
      projectId
      suiteName
      specName
      failureRate
      occurrenceCount
      status
      tags
      recentFailures {
        testRunId
        timestamp
        errorMessage
      }
    }
    statistics {
      totalActive
      totalResolved
      totalIgnored
      averageFailureRate
    }
  }
}
```

## Configuration

### Adjust Detection Thresholds

Configure flaky test detection in your environment:

```yaml
# config/config.yaml
analytics:
  flaky_detection:
    min_failure_rate: 0.1      # 10% failure rate
    max_failure_rate: 0.9      # 90% failure rate
    min_runs: 10               # Minimum test runs
    detection_window: "720h"   # 30 days
```

### Project-Level Overrides

Different projects may have different reliability requirements:

```bash
# Via API
POST /api/v1/projects/{projectId}/flaky-config
{
  "minFailureRate": 0.05,
  "minRuns": 20
}
```

## Managing Flaky Tests

### Best Practices

1. **Regular Review** - Check flaky tests weekly
2. **Prioritize by Impact** - Focus on critical path tests
3. **Track Resolution** - Document fixes in commit messages
4. **Use Ignoring Sparingly** - Only for known environmental issues

### Workflow Example

1. **Identify** - Dashboard shows new flaky test detected
2. **Investigate** - Review failure patterns and error messages
3. **Fix** - Update test to be more deterministic
4. **Verify** - Monitor for stability over next 10+ runs
5. **Resolve** - Test automatically marked as resolved when stable

### Integration with CI/CD

```yaml
# Example: GitHub Actions
- name: Check for flaky tests
  run: |
    FLAKY_COUNT=$(curl -s $FERN_URL/api/v1/analytics/flaky-tests/count?projectId=$PROJECT_ID | jq '.active')
    if [ $FLAKY_COUNT -gt 5 ]; then
      echo "::warning::Project has $FLAKY_COUNT active flaky tests"
    fi
```

## Reports and Analytics

### Flaky Test Report
Access via `/api/v1/analytics/reports/flaky-tests`:
- Trends over time
- Most problematic suites
- Resolution velocity
- Team comparisons

### Export Data
```bash
# CSV export
GET /api/v1/analytics/flaky-tests/export?format=csv&projectId={projectId}

# JSON export for further analysis
GET /api/v1/analytics/flaky-tests/export?format=json&includeHistory=true
```

## Common Patterns in Flaky Tests

Based on the data Fern Platform collects, common causes include:

1. **Race Conditions** - Tests with timing dependencies
2. **Test Order Dependencies** - Tests that depend on execution order
3. **Resource Contention** - Database or file system conflicts
4. **Network Issues** - External service dependencies
5. **Random Data** - Tests using randomized inputs

## Limitations

Current implementation does not include:
- Root cause analysis (coming in AI features)
- Automatic test retry policies
- Flakiness prediction before it occurs
- Integration with test frameworks for auto-retry

## Related Documentation

- [Debugging Test Failures](./debugging-test-failures.md) - Understand why tests fail
- [Test Performance Monitoring](./performance-monitoring.md) - Track test execution times
- [API Reference](../developers/api-reference.md) - Complete API documentation