# API Reference

Fern Platform provides both REST and GraphQL APIs for integration with CI/CD systems, custom tools, and third-party applications.

> 📌 **Recommended Approach**: Use our [official client libraries](./integration-guide.md) for your test framework instead of calling the API directly. This documentation is for:
> - Developers building client libraries for new test frameworks/languages
> - Teams needing custom integrations beyond standard test reporting
> - Contributors wanting to understand the API for creating new clients

## Overview

The platform offers two API styles:

1. **REST API** - Traditional RESTful endpoints for CRUD operations
2. **GraphQL API** - Modern query language for efficient data fetching

Both APIs use the same authentication mechanism (OAuth 2.0) and are designed to work together.

### Official Client Libraries

Before using the raw API, check if we have a client for your framework:

- **JavaScript/Jest**: [@guidewire/fern-jest-client](https://github.com/guidewire-oss/fern-jest-client)
- **Java/JUnit**: [fern-junit-client](https://github.com/guidewire-oss/fern-junit-client)
- **Go/Ginkgo**: [fern-ginkgo-client](https://github.com/guidewire-oss/fern-ginkgo-client)
- **Gradle Plugin**: [fern-junit-gradle-plugin](https://github.com/guidewire-oss/fern-junit-gradle-plugin)

## Authentication

All API requests require authentication using OAuth 2.0. See the [Authentication Setup](authentication.md) guide for details.

### For Web Clients

Web clients should use the OAuth flow and include session cookies:

```javascript
fetch('/api/v1/projects', {
    credentials: 'include'
});
```

### For CI/CD Clients

CI/CD systems and automated tools can use API keys (coming soon) or service accounts.

## REST API

The REST API provides traditional endpoints for all platform operations.

### Base URL

```
https://your-domain/api/v1
```

### Endpoints

#### Health Check

```http
GET /health
```

Returns the service health status. This is the only public endpoint.

**Response:**
```json
{
    "status": "healthy",
    "service": "fern-platform",
    "timestamp": "2025-06-25T10:30:00Z",
    "version": "1.0.0"
}
```

#### Projects

##### List Projects

```http
GET /api/v1/projects
```

Returns all projects the user has access to.

**Response:**
```json
{
    "data": [
        {
            "id": "uuid",
            "project_id": "my-project",
            "name": "My Project",
            "description": "Project description",
            "repository": "https://github.com/org/repo",
            "default_branch": "main",
            "is_active": true,
            "created_at": "2025-06-01T10:00:00Z",
            "updated_at": "2025-06-25T10:00:00Z"
        }
    ],
    "total": 1
}
```

##### Get Project

```http
GET /api/v1/projects/:projectId
```

Returns a single project by ID.

##### Create Project

```http
POST /api/v1/projects
```

Creates a new project. **Requires manager or admin privileges.**

**Request Body:**
```json
{
    "project_id": "my-project",
    "name": "My Project",
    "description": "Project description",
    "repository": "https://github.com/org/repo",
    "default_branch": "main"
}
```

#### Test Runs

##### List Test Runs

```http
GET /api/v1/test-runs
```

Returns test runs with optional filtering.

**Query Parameters:**
- `project_id` - Filter by project
- `branch` - Filter by branch
- `status` - Filter by status (passed, failed, running)
- `start_time` - Filter by start time (ISO 8601)
- `limit` - Number of results (default: 50)

**Response:**
```json
{
    "data": [
        {
            "id": "uuid",
            "run_id": "run-123",
            "project_id": "my-project",
            "branch": "main",
            "commit_sha": "abc123",
            "status": "passed",
            "start_time": "2025-06-25T10:00:00Z",
            "end_time": "2025-06-25T10:05:00Z",
            "duration": 300,
            "total_tests": 100,
            "passed_tests": 95,
            "failed_tests": 3,
            "skipped_tests": 2,
            "environment": "production",
            "metadata": {
                "ci_provider": "github-actions",
                "triggered_by": "push"
            }
        }
    ],
    "total": 1
}
```

##### Create Test Run

```http
POST /api/v1/test-runs
```

Creates a new test run. Used by test reporters.

**Request Body:**
```json
{
    "project_id": "my-project",
    "run_id": "run-123",
    "branch": "main",
    "commit_sha": "abc123",
    "environment": "ci",
    "metadata": {
        "ci_provider": "github-actions"
    }
}
```

##### Complete Test Run

```http
POST /api/v1/test-runs/complete
```

Completes a test run and updates its final status.

**Request Body:**
```json
{
    "runId": "run-123",
    "status": "passed",
    "endTime": "2025-06-25T10:05:00Z",
    "totalTests": 100,
    "passedTests": 95,
    "failedTests": 3,
    "skippedTests": 2
}
```

##### Update Test Run

```http
PUT /api/v1/test-runs/:id
```

Updates a test run by its database ID.

**Request Body:**
```json
{
    "status": "passed",
    "end_time": "2025-06-25T10:05:00Z",
    "total_tests": 100,
    "passed_tests": 95,
    "failed_tests": 3,
    "skipped_tests": 2
}
```

#### Test Results

##### Create Suite Run

```http
POST /api/v1/suite-runs
```

Creates a suite run within a test run.

**Request Body:**
```json
{
    "testRunId": "run-123",
    "suiteName": "API Tests",
    "status": "passed",
    "startTime": "2025-06-25T10:00:00Z",
    "endTime": "2025-06-25T10:02:30Z",
    "duration": 150000,
    "totalSpecs": 10,
    "passedSpecs": 9,
    "failedSpecs": 1
}
```

##### Create Spec Run

```http
POST /api/v1/spec-runs
```

Creates a spec run within a suite run.

**Request Body:**
```json
{
    "suiteRunId": 123,
    "specName": "should authenticate user",
    "status": "passed",
    "startTime": "2025-06-25T10:00:00Z",
    "endTime": "2025-06-25T10:00:50Z",
    "duration": 50000,
    "errorMessage": null,
    "stackTrace": null,
    "stdout": "",
    "stderr": "",
    "retries": 0
}
```

## GraphQL API

The GraphQL API provides a more efficient way to fetch data, especially for the UI.

### Endpoint

```
POST /query
```

### GraphQL Playground

An interactive GraphQL playground is available at `/graphql` when authenticated.

### Key Queries

#### Dashboard Data

Fetch all dashboard data in a single query:

```graphql
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
        overallPassRate
    }
    
    projects(first: 10) {
        edges {
            node {
                id
                projectId
                name
                isActive
            }
        }
    }
    
    recentTestRuns(limit: 10) {
        id
        runId
        projectId
        status
        startTime
    }
}
```

See the [GraphQL API Documentation](../graphql-api.md) for complete schema and examples.

## Error Handling

Both APIs use standard HTTP status codes:

- `200` - Success
- `201` - Created
- `400` - Bad Request
- `401` - Unauthorized
- `403` - Forbidden
- `404` - Not Found
- `500` - Internal Server Error

Error responses include a message:

```json
{
    "error": "Project not found",
    "code": "NOT_FOUND"
}
```

## Rate Limiting

API requests are rate limited to prevent abuse:

- **Authenticated users**: 1000 requests per hour
- **Unauthenticated requests**: 100 requests per hour

Rate limit headers are included in responses:

```
X-RateLimit-Limit: 1000
X-RateLimit-Remaining: 999
X-RateLimit-Reset: 1719316800
```

## SDK Support

Official SDKs are planned for:

- Go (fern-ginkgo-client)
- JavaScript/TypeScript
- Python
- Java

## Webhooks

Webhook support is planned for future releases to enable real-time notifications for:

- Test run completion
- Flaky test detection
- Build failures
- Performance regressions

## Best Practices

1. **Use GraphQL for UI**: The GraphQL API is optimized for UI data fetching
2. **Use REST for CI/CD**: The REST API is simpler for automated tools
3. **Batch Operations**: Use bulk endpoints when available
4. **Handle Pagination**: Always check for additional pages of results
5. **Error Handling**: Implement proper error handling and retries
6. **Caching**: Respect cache headers to reduce server load

## API Versioning

The API uses URL versioning:

- Current version: `/api/v1`
- Deprecated versions are supported for 6 months
- Breaking changes require a new version
- Non-breaking changes are added to the current version

## Need Help?

- Check the [Quick Start Guide](quick-start.md) for examples
- See [Authentication Setup](authentication.md) for auth configuration
- Review [GraphQL Documentation](../graphql-api.md) for GraphQL details
- Open an issue for bugs or feature requests