# Test Performance Monitoring

**Track test execution times and identify performance bottlenecks in your test suite**

> ✅ **Implemented**: Fern Platform tracks comprehensive timing data at the test, suite, and run levels with historical tracking and visualization.

## Overview

Fast test suites enable rapid development cycles. Fern Platform helps you monitor test performance by tracking execution times, identifying slow tests, and visualizing performance trends over time.

## What Performance Data is Collected

### Timing Hierarchy

Fern Platform captures timing at three levels:

1. **Test Run Level** - Total execution time for entire test run
2. **Suite Level** - Time for each test suite/file
3. **Spec Level** - Individual test execution time

### Calculated Metrics

- **Average Duration** - Mean execution time over multiple runs
- **Duration Trends** - Performance changes over time
- **Cumulative Time** - Total time spent on each test/suite
- **Performance Distribution** - Visualization of fast vs slow tests

## Viewing Performance Data

### Web UI

#### Performance Dashboard
The main dashboard shows:
- Average test run duration trend
- Slowest test suites
- Performance distribution treemap

#### Treemap Visualization
The interactive treemap displays:
- **Size** = Number of tests
- **Color** = Performance (green=fast, yellow=medium, red=slow)
- **Hover** = Detailed timing information

Navigate by clicking on rectangles to drill down into suites and tests.

#### Test Run Details
For any test run:
1. Click on the run in the Test Runs page
2. View total duration and timing breakdown
3. Sort tests by duration to find slowest ones
4. See duration distribution chart

### REST API

#### Get Performance Summary
```bash
GET /api/v1/test-runs/{runId}

# Response includes timing data
{
  "id": "run-123",
  "duration": 125000,  // Total run time in ms
  "suites": [{
    "name": "User API Tests",
    "duration": 45000,
    "specs": [{
      "name": "should create user",
      "duration": 1200,
      "status": "passed"
    }]
  }]
}
```

#### Query Historical Performance
```bash
GET /api/v1/projects/{projectId}/performance-trends?days=30

# Returns performance data over time
{
  "trends": [{
    "date": "2024-01-20",
    "avgDuration": 120000,
    "runCount": 15
  }]
}
```

### GraphQL Queries

#### Get Performance Analytics
```graphql
query GetTestPerformance($projectId: ID!, $days: Int) {
  project(id: $projectId) {
    testPerformance(days: $days) {
      averageDuration
      totalRuns
      slowestTests {
        suiteName
        specName
        averageDuration
        runCount
      }
      performanceTrend {
        date
        averageDuration
        p95Duration
      }
    }
  }
}
```

#### Treemap Data Query
```graphql
query GetPerformanceTreemap($projectId: ID!) {
  treemapData(projectId: $projectId) {
    name
    value  # number of tests
    duration  # average duration
    color  # performance indicator
    children {
      name
      value
      duration
      color
    }
  }
}
```

## Performance Analysis Features

### Identify Slow Tests

Find tests that are slowing down your suite:

```bash
# Get slowest tests
GET /api/v1/analytics/slow-tests?projectId={projectId}&limit=20

{
  "slowTests": [{
    "suiteName": "Integration Tests",
    "specName": "full system test",
    "averageDuration": 30000,
    "runCount": 50,
    "trend": "increasing"  // getting slower
  }]
}
```

### Performance Trends

Track how test performance changes over time:
- Daily/weekly averages
- Performance after deployments
- Correlation with code changes

### Suite-Level Analysis

Understand which test suites consume the most time:
- Cumulative time per suite
- Number of tests per suite
- Average test duration by suite

## Best Practices

### Setting Performance Budgets

While not enforced by the platform, you can track against targets:

```javascript
// Example: Check performance in CI
const performance = await fetch(`${FERN_URL}/api/v1/projects/${PROJECT_ID}/performance-trends?days=1`);
const avgDuration = performance.trends[0].avgDuration;

if (avgDuration > 300000) { // 5 minute budget
  console.warn(`Test suite too slow: ${avgDuration}ms`);
  process.exit(1);
}
```

### Optimizing Test Performance

Based on Fern Platform data, optimize by:

1. **Parallelize Slow Suites** - Run independent suites concurrently
2. **Split Large Tests** - Break down tests taking >10s
3. **Optimize Setup/Teardown** - Reduce repeated expensive operations
4. **Mock External Services** - Eliminate network latency
5. **Profile Resource Usage** - Find CPU/Memory bottlenecks

### Monitoring Degradation

Set up alerts for performance regression:

```yaml
# Example: GitHub Actions
- name: Check test performance
  run: |
    CURRENT=$(curl -s $FERN_URL/api/v1/projects/$PROJECT_ID/performance | jq '.averageDuration')
    BASELINE=$(curl -s $FERN_URL/api/v1/projects/$PROJECT_ID/performance?days=7 | jq '.averageDuration')
    
    # Check for division by zero
    if [ "$BASELINE" -eq 0 ]; then
      echo "::warning::No baseline performance data available"
      exit 0
    fi
    
    INCREASE=$(( ($CURRENT - $BASELINE) * 100 / $BASELINE ))
    if [ $INCREASE -gt 20 ]; then
      echo "::error::Test performance degraded by ${INCREASE}%"
      exit 1
    fi
```

## Current Limitations

Performance monitoring currently tracks duration only. It does not include:

- ❌ CPU/Memory usage per test
- ❌ Network I/O metrics
- ❌ Database query counts
- ❌ Parallel execution tracking
- ❌ Resource contention analysis
- ❌ Automatic performance regression detection

## Integration Examples

### Send Detailed Timing Data

```javascript
// Jest example with timing
const results = {
  projectId: "my-project",
  duration: 125000,
  suites: [{
    name: "user.test.js",
    duration: 45000,
    specs: testResults.map(test => ({
      name: test.title,
      duration: test.duration,
      status: test.status
    }))
  }]
};

await fetch(`${FERN_URL}/api/v1/test-runs`, {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify(results)
});
```

### Query Performance for Reporting

```python
# Python example for performance report
import requests
import matplotlib.pyplot as plt

# Get performance trends
response = requests.get(f"{FERN_URL}/api/v1/projects/{PROJECT_ID}/performance-trends?days=30")
trends = response.json()['trends']

# Plot the trend
dates = [t['date'] for t in trends]
durations = [t['avgDuration']/1000 for t in trends]  # Convert to seconds

plt.plot(dates, durations)
plt.title('Test Suite Performance Trend')
plt.ylabel('Duration (seconds)')
plt.xlabel('Date')
plt.show()
```

## Related Documentation

- [Finding Flaky Tests](./finding-flaky-tests.md) - Flaky tests often have variable performance
- [Integration Guide](../developers/integration-guide.md) - Send timing data from your CI/CD
- [GraphQL API](../graphql-api.md) - Advanced performance queries