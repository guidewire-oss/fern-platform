# CI/CD Integration Guide

**Connect your test framework to Fern Platform for automatic test reporting**

> 📌 **Recommended Method**: Use our official client libraries for seamless integration with your test framework.

## Overview

Fern Platform provides client libraries for major test frameworks that automatically report test results. For custom frameworks or advanced use cases, you can also use our REST API directly.

## Integration Architecture

```
┌─────────────┐     Client Library     ┌──────────────┐
│   Test      │ -------------------> │     Fern     │
│  Framework  │   Automatic Upload   │   Platform   │
└─────────────┘                      └──────────────┘
     ↑                                      ↓
     │                                ┌──────────────┐
     └── Fern Client Libraries        │  Dashboard   │
         (Jest, JUnit, Ginkgo)        │ & Analytics  │
                                      └──────────────┘
```

## Official Client Libraries

### Available Now

- **Go/Ginkgo**: [fern-ginkgo-client](https://github.com/guidewire-oss/fern-ginkgo-client)
- **Java/JUnit**: [fern-junit-client](https://github.com/guidewire-oss/fern-junit-client)
- **Gradle Plugin**: [fern-junit-gradle-plugin](https://github.com/guidewire-oss/fern-junit-gradle-plugin)
- **JavaScript/Jest**: [fern-jest-client](https://github.com/guidewire-oss/fern-jest-client)

### Coming Soon

- Python/pytest
- Ruby/RSpec
- .NET/NUnit

For other frameworks, you can use the REST API directly or create your own client library.

## Quick Start

### Prerequisites

**Your team manager must first create a project:**
1. Log in to Fern Platform with manager privileges
2. Navigate to **Projects** → **Create New Project**
3. Share the project ID with your development team

### 1. Install a Client Library

Choose the appropriate client for your test framework:

#### JavaScript/Jest
```bash
npm install --save-dev @guidewire/fern-jest-client
```

#### Java/JUnit
```xml
<dependency>
    <groupId>com.guidewire.fern</groupId>
    <artifactId>fern-junit-client</artifactId>
    <version>1.0.0</version>
    <scope>test</scope>
</dependency>
```

#### Go/Ginkgo
```bash
go get github.com/guidewire-oss/fern-ginkgo-client
```

### 2. Set Environment Variables

```bash
export FERN_URL=http://fern-platform.local:8080
export FERN_PROJECT_ID=proj_abc123  # Get this from your manager
```

### 3. Configure Your Test Framework

#### Jest Configuration

```javascript
// jest.config.js
module.exports = {
  reporters: [
    'default',
    ['@guidewire/fern-jest-client', {
      url: process.env.FERN_URL,
      projectId: process.env.FERN_PROJECT_ID
    }]
  ]
};
```

#### JUnit Configuration

```java
// Add to your test runner
@RunWith(FernJUnitRunner.class)
@FernConfig(
    url = "${FERN_URL}",
    projectId = "${FERN_PROJECT_ID}"
)
public class MyTestSuite {
    // Your tests
}
```

#### Ginkgo Configuration

```go
import "github.com/guidewire-oss/fern-ginkgo-client/reporter"

func TestMySuite(t *testing.T) {
    RegisterFailHandler(Fail)
    RunSpecsWithDefaultAndCustomReporters(t, "My Suite",
        []Reporter{reporter.NewFernReporter()})
}
```

Your test results will now be automatically sent to Fern Platform!

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
      - image: circleci/node:18
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

## Using Client Libraries

We strongly recommend using our official client libraries for seamless integration:

### Jest (JavaScript)

Install the client:
```bash
npm install --save-dev @guidewire/fern-jest-client
```

Configure in `jest.config.js`:
```javascript
module.exports = {
  reporters: [
    'default',
    ['@guidewire/fern-jest-client', {
      url: process.env.FERN_URL,
      projectId: process.env.FERN_PROJECT_ID
    }]
  ]
};
```

### JUnit (Java)

Add to your `pom.xml`:
```xml
<dependency>
    <groupId>com.guidewire.fern</groupId>
    <artifactId>fern-junit-client</artifactId>
    <version>1.0.0</version>
    <scope>test</scope>
</dependency>
```

Or for Gradle projects, use our plugin:
```gradle
plugins {
  id 'com.guidewire.fern' version '1.0.0'
}

fern {
  url = System.getenv('FERN_URL')
  projectId = System.getenv('FERN_PROJECT_ID')
}
```

### Ginkgo (Go)

Install the client:
```bash
go get github.com/guidewire-oss/fern-ginkgo-client
```

Use in your test suite:
```go
import "github.com/guidewire-oss/fern-ginkgo-client/reporter"

func TestMySuite(t *testing.T) {
    RegisterFailHandler(Fail)
    RunSpecsWithDefaultAndCustomReporters(t, "My Suite",
        []Reporter{reporter.NewFernReporter()})
}
```

### Other Frameworks

For frameworks without official clients, you can:
1. Create your own client library using our REST/GraphQL APIs
2. Contribute your client library to the community
3. Request official support by opening an issue

## Building Your Own Client Library

Want to create a client for a new test framework or language? Here's how:

### Client Library Requirements

A good Fern client library should:

1. **Auto-detect Configuration** - Read from environment variables
2. **Handle Test Lifecycle** - Hook into framework's test events
3. **Batch Results** - Collect and send results efficiently
4. **Error Handling** - Gracefully handle network/API failures
5. **Zero Config** - Work with minimal setup

### Implementation Guide

#### 1. Configuration
```javascript
// Example: Reading configuration
const config = {
  url: process.env.FERN_URL || 'http://localhost:8080',
  projectId: process.env.FERN_PROJECT_ID,
  apiToken: process.env.FERN_API_TOKEN, // Optional
  batchSize: 100, // Send results in batches
  enabled: process.env.FERN_ENABLED !== 'false'
};
```

#### 2. Test Result Collection
```typescript
interface TestResult {
  name: string;
  status: 'passed' | 'failed' | 'skipped';
  duration: number; // milliseconds
  error?: string;
  stackTrace?: string;
}

interface TestSuite {
  name: string;
  specs: TestResult[];
  duration: number;
}
```

#### 3. API Integration
Use either REST or GraphQL:

**REST Example:**
```python
def send_results(test_run):
    response = requests.post(
        f"{config['url']}/api/v1/test-runs",
        json={
            'projectId': config['projectId'],
            'status': test_run['status'],
            'duration': test_run['duration'],
            'passedTests': test_run['passed'],
            'failedTests': test_run['failed'],
            'suites': test_run['suites']
        },
        headers={'Authorization': f"Bearer {config['token']}"}
    )
    return response.json()
```

**GraphQL Example:**
```javascript
const mutation = `
  mutation CreateTestRun($input: TestRunInput!) {
    createTestRun(input: $input) {
      id
      status
      createdAt
    }
  }
`;

const result = await graphqlClient.request(mutation, {
  input: testRunData
});
```

#### 4. Framework Integration Examples

**Python/pytest:**
```python
# Hook into pytest's plugin system
def pytest_runtest_makereport(item, call):
    # Collect test results
    
def pytest_sessionfinish(session, exitstatus):
    # Send all results to Fern
```

**Ruby/RSpec:**
```ruby
RSpec.configure do |config|
  config.reporter.register_listener(
    FernReporter.new,
    :example_passed,
    :example_failed,
    :example_pending,
    :stop
  )
end
```

**PHP/PHPUnit:**
```php
class FernTestListener implements TestListener {
    public function endTest(Test $test, float $time): void {
        // Collect result
    }
    
    public function endTestSuite(TestSuite $suite): void {
        // Send to Fern
    }
}
```

### Reference Implementations

Study our existing clients for best practices:

- **[Jest Client](https://github.com/guidewire-oss/fern-jest-client)** - Good example of hooking into reporter system
- **[JUnit Client](https://github.com/guidewire-oss/fern-junit-client)** - Shows test listener pattern
- **[Ginkgo Client](https://github.com/guidewire-oss/fern-ginkgo-client)** - Demonstrates Go's testing integration

### Publishing Your Client

1. **Naming Convention**: Use `fern-{framework}-client` or `{language}-fern-client`
2. **Documentation**: Include clear setup instructions
3. **Examples**: Provide working examples
4. **Tests**: Test against multiple framework versions
5. **Package Registry**: Publish to appropriate registry (npm, PyPI, RubyGems, etc.)

### Contributing Back

We welcome community contributions! To get your client listed as official:

1. Open an issue describing your client
2. Ensure it follows our patterns
3. Add comprehensive tests
4. Submit a PR to add it to our documentation

## Custom Integration (REST API)

If you need to create a custom integration, use our REST API directly:

### Basic Test Run Submission

```bash
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

### Creating Your Own Client

When building a custom client:
1. Collect test results from your framework
2. Transform to Fern's format
3. Send via HTTP POST to `/api/v1/test-runs`
4. Handle authentication if required

See our existing clients for reference implementations:
- [Jest Client Source](https://github.com/guidewire-oss/fern-jest-client)
- [JUnit Client Source](https://github.com/guidewire-oss/fern-junit-client)
- [Ginkgo Client Source](https://github.com/guidewire-oss/fern-ginkgo-client)

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