# Mock JIRA Service

This is a mock JIRA Cloud API server used for local development and acceptance testing of the Fern Platform's JIRA integration features.

## Overview

The mock-jira service simulates essential JIRA Cloud API endpoints to enable testing without requiring a real JIRA instance. It supports:

- Authentication (Bearer token and Basic auth)
- Project management endpoints
- Field definitions
- Issue types
- Server information

## Building and Running

### Local Development (Docker)

1. Build the Docker image:
   ```bash
   cd mock-jira
   docker build -t mock-jira:latest .
   ```

2. Run the container:
   ```bash
   docker run -p 8080:8080 mock-jira:latest
   ```

### Kubernetes (k3d) Deployment

1. Build and load the image into k3d:
   ```bash
   # From the project root
   cd mock-jira
   docker build -t mock-jira:latest .
   k3d image import mock-jira:latest -c fern-platform
   ```

2. Deploy to Kubernetes:
   ```bash
   kubectl apply -f deployments/mock-jira-deployment.yaml
   ```

3. Verify the deployment:
   ```bash
   kubectl get pods -n fern-platform | grep mock-jira
   ```

## API Endpoints

The mock service implements the following JIRA Cloud API endpoints:

- `GET /rest/api/2/myself` - Returns authenticated user information
- `GET /rest/api/2/project` - Lists all projects
- `GET /rest/api/2/project/{key}` - Gets specific project details
- `GET /rest/api/2/field` - Lists all field definitions
- `GET /rest/api/2/issuetype` - Lists all issue types
- `GET /rest/api/2/serverInfo` - Returns server information

## Mock Data

The service provides three mock projects:
- **FERN** - The Fern Platform project
- **TEST** - Generic test project
- **DEMO** - Demo project

## Authentication

The mock service accepts any authentication token for testing purposes:
- Bearer tokens: `Authorization: Bearer <any-token>`
- Basic auth: `Authorization: Basic <base64-encoded-credentials>`

## Usage in Tests

The acceptance tests automatically start and use the mock-jira service. See:
- `/acceptance/helpers/mock_jira_server.go` - Test helper for starting mock server
- `/acceptance/jira_connection_test.go` - Example usage in tests

## Troubleshooting

If you encounter `ImagePullBackOff` errors in k3d:
1. Ensure the image is built locally: `docker images | grep mock-jira`
2. Import the image into k3d: `k3d image import mock-jira:latest -c fern-platform`
3. Delete the failing pod to trigger a new deployment: `kubectl delete pod <pod-name> -n fern-platform`