# CI/CD Integration Guide

**Connect your continuous integration pipeline to Fern Platform for automatic test reporting**

> 📌 **Integration Method**: Fern Platform uses a REST API for all integrations. Native CI/CD plugins and webhooks are planned for future releases.

## Overview

Fern Platform integrates with any CI/CD system that can make HTTP requests. Your test runner sends results to Fern's REST API after each test run.

## Integration Architecture

```
┌─────────────┐     HTTP POST      ┌──────────────┐
│   CI/CD     │ -----------------> │     Fern     │
│  Pipeline   │   Test Results     │   Platform   │
└─────────────┘                    └──────────────┘
     ↑                                    ↓
     │                              ┌──────────────┐
     └── Test Framework             │  Dashboard   │
         (Jest, pytest, etc.)       │ & Analytics  │
                                    └──────────────┘
```

## Quick Start

### 1. Get Your Project ID

First, register your project in Fern Platform:

```bash
# Via API
curl -X POST http://fern-platform.local:8080/api/v1/projects \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "name": "my-project",
    "description": "My awesome project"
  }'

# Response
{
  "id": "proj_abc123",
  "name": "my-project"
}
```

### 2. Set Environment Variables

```bash
export FERN_URL=http://fern-platform.local:8080
export FERN_PROJECT_ID=proj_abc123
export FERN_API_TOKEN=your-api-token  # If authentication is enabled
```

### 3. Send Test Results

After your tests run, send the results:

```bash
# Basic test run submission
curl -X POST $FERN_URL/api/v1/test-runs \
  -H "Content-Type: application/json" \
  -d '{
    "projectId": "'$FERN_PROJECT_ID'",
    "status": "passed",
    "duration": 125000,
    "passedTests": 95,
    "failedTests": 0,
    "skippedTests": 5,
    "gitCommit": "'$GIT_COMMIT'",
    "gitBranch": "'$GIT_BRANCH'"
  }'
```

## Integration by CI/CD Platform

### GitHub Actions

```yaml
name: Tests
on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - name: Run tests
        run: npm test -- --json --outputFile=test-results.json
        
      - name: Report to Fern Platform
        if: always()
        run: |
          # Parse test results and send to Fern
          node scripts/report-to-fern.js test-results.json
        env:
          FERN_URL: ${{ secrets.FERN_URL }}
          FERN_PROJECT_ID: ${{ secrets.FERN_PROJECT_ID }}
```

### Jenkins Pipeline

```groovy
pipeline {
  agent any
  
  environment {
    FERN_URL = credentials('fern-url')
    FERN_PROJECT_ID = credentials('fern-project-id')
  }
  
  stages {
    stage('Test') {
      steps {
        sh 'npm test'
      }
      post {
        always {
          script {
            def testResults = readJSON file: 'test-results.json'
            def payload = [
              projectId: env.FERN_PROJECT_ID,
              status: currentBuild.result == 'SUCCESS' ? 'passed' : 'failed',
              duration: testResults.duration,
              passedTests: testResults.passed,
              failedTests: testResults.failed,
              gitCommit: env.GIT_COMMIT,
              gitBranch: env.GIT_BRANCH
            ]
            
            httpRequest(
              url: "${env.FERN_URL}/api/v1/test-runs",
              httpMode: 'POST',
              contentType: 'APPLICATION_JSON',
              requestBody: groovy.json.JsonOutput.toJson(payload)
            )
          }
        }
      }
    }
  }
}
```

### GitLab CI

```yaml
test:
  stage: test
  script:
    - npm test -- --reporter json > test-results.json
  after_script:
    - |
      curl -X POST $FERN_URL/api/v1/test-runs \
        -H "Content-Type: application/json" \
        -d @- <<EOF
      {
        "projectId": "$FERN_PROJECT_ID",
        "status": "$CI_JOB_STATUS",
        "duration": $(jq .duration test-results.json),
        "passedTests": $(jq .passed test-results.json),
        "failedTests": $(jq .failed test-results.json),
        "gitCommit": "$CI_COMMIT_SHA",
        "gitBranch": "$CI_COMMIT_REF_NAME"
      }
      EOF
  variables:
    FERN_URL: https://fern.company.com
    FERN_PROJECT_ID: proj_abc123
```

### CircleCI

```yaml
version: 2.1

jobs:
  test:
    docker:
      - image: circleci/node:14
    steps:
      - checkout
      - run:
          name: Run tests and report
          command: |
            # Run tests and capture exit code
            npm test -- --json > test-results.json || TEST_EXIT_CODE=$?
            
            # Report to Fern Platform
            curl -X POST $FERN_URL/api/v1/test-runs \
              -H "Content-Type: application/json" \
              -d "{
                \"projectId\": \"$FERN_PROJECT_ID\",
                \"status\": \"$([[ ${TEST_EXIT_CODE:-0} -eq 0 ]] && echo 'passed' || echo 'failed')\",
                \"gitCommit\": \"$CIRCLE_SHA1\",
                \"gitBranch\": \"$CIRCLE_BRANCH\"
              }"
            
            # Exit with original test exit code
            exit ${TEST_EXIT_CODE:-0}
```

## Test Framework Integration

### Jest (JavaScript)

Create a custom reporter (`fern-reporter.js`):

```javascript
class FernReporter {
  constructor(globalConfig, options) {
    this.fernUrl = process.env.FERN_URL;
    this.projectId = process.env.FERN_PROJECT_ID;
  }

  async onRunComplete(contexts, results) {
    const testRun = {
      projectId: this.projectId,
      status: results.numFailedTests === 0 ? 'passed' : 'failed',
      duration: Date.now() - results.startTime,
      passedTests: results.numPassedTests,
      failedTests: results.numFailedTests,
      skippedTests: results.numPendingTests,
      gitCommit: process.env.GIT_COMMIT || 'unknown',
      gitBranch: process.env.GIT_BRANCH || 'unknown',
      suites: this.formatSuites(results.testResults)
    };

    await fetch(`${this.fernUrl}/api/v1/test-runs`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(testRun)
    });
  }

  formatSuites(testResults) {
    return testResults.map(suite => ({
      name: suite.testFilePath,
      duration: suite.perfStats.runtime,
      specs: suite.testResults.map(test => ({
        name: test.title,
        status: test.status,
        duration: test.duration,
        error: test.failureMessages?.[0],
        stackTrace: test.failureDetails?.[0]?.stack
      }))
    }));
  }
}

module.exports = FernReporter;
```

Use in `jest.config.js`:
```javascript
module.exports = {
  reporters: ['default', '<rootDir>/fern-reporter.js']
};
```

### pytest (Python)

Create a pytest plugin (`pytest_fern.py`):

```python
import pytest
import requests
import os
import time
from datetime import datetime

class FernPlugin:
    def __init__(self):
        self.fern_url = os.environ.get('FERN_URL')
        self.project_id = os.environ.get('FERN_PROJECT_ID')
        self.start_time = None
        self.test_results = []

    def pytest_sessionstart(self):
        self.start_time = time.time()

    def pytest_runtest_logreport(self, report):
        if report.when == 'call':
            self.test_results.append({
                'name': report.nodeid,
                'status': 'passed' if report.passed else 'failed',
                'duration': report.duration * 1000,  # Convert to ms
                'error': str(report.longrepr) if report.failed else None
            })

    def pytest_sessionfinish(self, exitstatus):
        duration = (time.time() - self.start_time) * 1000
        
        test_run = {
            'projectId': self.project_id,
            'status': 'passed' if exitstatus == 0 else 'failed',
            'duration': duration,
            'passedTests': len([t for t in self.test_results if t['status'] == 'passed']),
            'failedTests': len([t for t in self.test_results if t['status'] == 'failed']),
            'gitCommit': os.environ.get('GIT_COMMIT', 'unknown'),
            'gitBranch': os.environ.get('GIT_BRANCH', 'unknown'),
            'suites': [{
                'name': 'pytest',
                'duration': duration,
                'specs': self.test_results
            }]
        }
        
        requests.post(
            f"{self.fern_url}/api/v1/test-runs",
            json=test_run
        )

def pytest_configure(config):
    config.pluginmanager.register(FernPlugin(), 'fern')
```

### Go testing

```go
package main

import (
    "bytes"
    "encoding/json"
    "net/http"
    "os"
    "testing"
    "time"
)

type FernReporter struct {
    URL       string
    ProjectID string
    StartTime time.Time
    Results   []TestResult
}

type TestResult struct {
    Name     string `json:"name"`
    Status   string `json:"status"`
    Duration int64  `json:"duration"`
    Error    string `json:"error,omitempty"`
}

func (r *FernReporter) Report() error {
    testRun := map[string]interface{}{
        "projectId":    r.ProjectID,
        "status":       "passed", // Update based on results
        "duration":     time.Since(r.StartTime).Milliseconds(),
        "passedTests":  len(r.Results),
        "failedTests":  0,
        "gitCommit":    os.Getenv("GIT_COMMIT"),
        "gitBranch":    os.Getenv("GIT_BRANCH"),
    }
    
    data, _ := json.Marshal(testRun)
    _, err := http.Post(r.URL+"/api/v1/test-runs", "application/json", bytes.NewBuffer(data))
    return err
}
```

## Sending Detailed Test Data

### Full Test Hierarchy

Send complete test hierarchy with suites and specs:

```json
POST /api/v1/test-runs/with-suites

{
  "projectId": "proj_abc123",
  "status": "failed",
  "duration": 125000,
  "passedTests": 94,
  "failedTests": 1,
  "skippedTests": 5,
  "gitCommit": "abc123def",
  "gitBranch": "main",
  "suites": [
    {
      "name": "Authentication Tests",
      "duration": 45000,
      "specs": [
        {
          "name": "should login with valid credentials",
          "status": "passed",
          "duration": 1200
        },
        {
          "name": "should reject invalid password",
          "status": "failed",
          "duration": 800,
          "error": "Expected 401 but got 500",
          "stackTrace": "at auth.test.js:45:5\n  at processTicksAndRejections..."
        }
      ]
    }
  ]
}
```

### Batch Upload

For large test suites, upload in batches:

```bash
# Split large test results into chunks
split -l 1000 test-results.json chunk_

# Upload each chunk
for chunk in chunk_*; do
  curl -X POST $FERN_URL/api/v1/test-runs/batch \
    -H "Content-Type: application/json" \
    -d @$chunk
done
```

## Authentication

If your Fern Platform instance requires authentication:

### API Token

```bash
# Include in headers
curl -X POST $FERN_URL/api/v1/test-runs \
  -H "Authorization: Bearer $FERN_API_TOKEN" \
  -H "Content-Type: application/json" \
  -d @test-results.json
```

### OAuth 2.0

```javascript
// Get access token first
const tokenResponse = await fetch(`${FERN_URL}/oauth/token`, {
  method: 'POST',
  headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
  body: new URLSearchParams({
    grant_type: 'client_credentials',
    client_id: process.env.FERN_CLIENT_ID,
    client_secret: process.env.FERN_CLIENT_SECRET
  })
});

const { access_token } = await tokenResponse.json();

// Use token for API calls
await fetch(`${FERN_URL}/api/v1/test-runs`, {
  method: 'POST',
  headers: {
    'Authorization': `Bearer ${access_token}`,
    'Content-Type': 'application/json'
  },
  body: JSON.stringify(testRun)
});
```

## Best Practices

### 1. Include Git Information
Always send git commit and branch for better tracking:
```bash
export GIT_COMMIT=$(git rev-parse HEAD)
export GIT_BRANCH=$(git rev-parse --abbrev-ref HEAD)
```

### 2. Handle Failures Gracefully
Don't fail the build if reporting fails:
```bash
# Report to Fern but don't fail if it's down
curl -X POST $FERN_URL/api/v1/test-runs ... || echo "Failed to report to Fern"
```

### 3. Use Meaningful Suite Names
Organize tests into logical suites:
```json
{
  "suites": [
    { "name": "unit/auth", "specs": [...] },
    { "name": "integration/api", "specs": [...] },
    { "name": "e2e/checkout", "specs": [...] }
  ]
}
```

### 4. Send Complete Error Information
Include full error details for failed tests:
```json
{
  "error": "AssertionError: Expected 200 but got 404",
  "stackTrace": "Full stack trace here...",
  "stdout": "Console output if available",
  "stderr": "Error output if available"
}
```

## Troubleshooting

### Connection Issues
```bash
# Test connectivity
curl -f $FERN_URL/health || echo "Cannot reach Fern Platform"

# Check DNS
nslookup fern-platform.local
```

### Authentication Errors
```bash
# Verify token is valid
curl -H "Authorization: Bearer $FERN_API_TOKEN" \
     $FERN_URL/api/v1/projects
```

### Data Validation Errors
Common issues:
- Missing required fields (projectId, status)
- Invalid status values (use: passed, failed, skipped)
- Duration must be in milliseconds
- Git commit should be full SHA

## Future Enhancements

Coming soon:
- 🔌 Native CI/CD plugins for popular platforms
- 🪝 Webhook support for real-time notifications
- 📦 SDK libraries for major languages
- 🔄 Automatic retry and queuing
- 📊 Streaming test results during execution

## Related Documentation

- [API Reference](./api-reference.md) - Complete REST API documentation
- [Quick Start](./quick-start.md) - Get started with Fern Platform
- [Configuration](../configuration/) - Configure projects and authentication