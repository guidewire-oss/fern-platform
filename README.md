# Fern Platform

[![Go Version](https://img.shields.io/badge/go-1.23+-00ADD8.svg?style=flat-square&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg?style=flat-square)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/guidewire-oss/fern-platform?style=flat-square)](https://goreportcard.com/report/github.com/guidewire-oss/fern-platform)
[![codecov](https://codecov.io/gh/guidewire-oss/fern-platform/branch/main/graph/badge.svg?token=CODECOV_TOKEN)](https://codecov.io/gh/guidewire-oss/fern-platform)
[![CI Status](https://img.shields.io/github/actions/workflow/status/guidewire-oss/fern-platform/ci.yml?branch=main&label=CI&style=flat-square)](https://github.com/guidewire-oss/fern-platform/actions/workflows/ci.yml)

A unified test intelligence platform that transforms fragmented test data into actionable insights.

## What is Fern Platform?

Fern Platform aggregates test results from any CI/CD pipeline and testing framework (Jest, pytest, JUnit, etc.) into a centralized dashboard. It automatically detects flaky tests, tracks performance trends, and provides the visibility engineering teams need to maintain healthy test suites.

Think of it as a specialized analytics platform for your tests - like Datadog or Grafana, but purpose-built for test intelligence.

<img src="docs/images/test-summaries.png" alt="Fern Platform Dashboard" width="800"/>

## Key Features

- **Universal Test Aggregation** - REST API accepts test results from any framework or CI/CD system
- **Flaky Test Detection** - Automatically identifies tests that pass/fail intermittently
- **Performance Monitoring** - Track test execution times and identify slow tests
- **Interactive Visualizations** - Treemap view shows test suite health at a glance
- **Team-Based Access Control** - OAuth/SSO with role-based permissions
- **Rich Querying** - GraphQL API for complex test data analysis

## Quick Start

### Requirements

- Docker with buildx
- [k3d](https://k3d.io/stable/#installation) (lightweight Kubernetes)
- kubectl
- Go 1.21+ (used by Makefile for architecture detection)
- Make
- 8GB RAM minimum

### Installation

```bash
# Clone the repository
git clone https://github.com/guidewire-oss/fern-platform
cd fern-platform

# Add required hosts entries (for OAuth to work)
echo "127.0.0.1 fern-platform.local" | sudo tee -a /etc/hosts
echo "127.0.0.1 keycloak" | sudo tee -a /etc/hosts

# Deploy everything (takes ~15 minutes)
make deploy-all
```

Access the platform at `http://fern-platform.local:8080`

**Default credentials**: `admin@fern.com` / `test123`

### Basic Usage

Submit test results from your CI/CD pipeline:

```bash
# Report a test run
curl -X POST http://fern-platform.local:8080/api/v1/test-runs \
  -H "Content-Type: application/json" \
  -d '{
    "projectId": "my-project",
    "status": "passed",
    "duration": 45000,
    "passedTests": 150,
    "failedTests": 2,
    "gitCommit": "abc123",
    "gitBranch": "main"
  }'
```

View results in the dashboard or query via GraphQL:

```graphql
query {
  testRuns(projectId: "my-project", first: 10) {
    runs {
      id
      status
      duration
      gitCommit
    }
  }
}
```

## Documentation

### Quick Links by Role

**For Users** → [UI Features Guide](docs/user-guide/ui-features.md) • [Workflows](docs/workflows/README.md) • [Use Cases](docs/use-cases/)

**For Developers** → [Integration Guide](docs/developers/integration-guide.md) • [API Reference](docs/developers/api-reference.md) • [GraphQL](docs/graphql-api.md)

**For DevOps** → [Installation](docs/developers/quick-start.md) • [Configuration](docs/configuration/) • [Troubleshooting](docs/troubleshooting/README.md)

**For Contributors** → [Architecture](docs/ARCHITECTURE.md) • [Contributing](CONTRIBUTING.md) • [RFCs](docs/rfc/)

### All Documentation

See [complete documentation index](docs/all-docs.md) or browse [docs/](docs/) directly.

## Use Cases

Fern Platform helps engineering teams:

- **Identify flaky tests** that waste CI time and erode confidence
- **Track test performance** to find and fix slow tests
- **Monitor test health** across multiple projects and teams
- **Debug failures** with historical context and error patterns

See our [use case guides](docs/use-cases/) for detailed examples.

## Integration Examples

### GitHub Actions

```yaml
- name: Run tests and report to Fern
  run: |
    npm test -- --json > results.json
    curl -X POST ${{ secrets.FERN_URL }}/api/v1/test-runs \
      -H "Content-Type: application/json" \
      -d @results.json
```

### Jest Reporter

```javascript
// jest.config.js
module.exports = {
  reporters: ['default', '<rootDir>/fern-reporter.js']
};
```

See [integration guide](docs/developers/integration-guide.md) for more examples.

## Architecture

Fern Platform uses domain-driven design with a hexagonal architecture:

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│   Web UI        │     │   REST API      │     │   GraphQL API   │
└────────┬────────┘     └────────┬────────┘     └────────┬────────┘
         └──────────────────────┬┴──────────────────────────┘
                                │
                    ┌───────────┴────────────┐
                    │   Business Domains     │
                    │  ┌─────┐ ┌─────────┐  │
                    │  │Tests│ │Analytics│  │
                    │  └─────┘ └─────────┘  │
                    └───────────┬────────────┘
                                │
                    ┌───────────┴────────────┐
                    │   Infrastructure       │
                    │  PostgreSQL + Redis    │
                    └────────────────────────┘
```

## Project Status

Fern Platform is under active development. Core features are stable and used in production by several teams.

**Stable**: Test ingestion, flaky detection, OAuth, web dashboard  
**In Progress**: AI-powered insights, webhook integrations  
**Planned**: Real-time subscriptions, native CI/CD plugins

See our [roadmap](https://github.com/guidewire-oss/fern-platform/issues?q=is%3Aissue+is%3Aopen+label%3Aroadmap) for details.

## Contributing

We welcome contributions! See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

Areas where we need help:
- Test framework integrations
- UI/UX improvements
- Documentation
- Bug fixes

## Community

- [GitHub Discussions](https://github.com/guidewire-oss/fern-platform/discussions) - Ask questions and share ideas
- [Issue Tracker](https://github.com/guidewire-oss/fern-platform/issues) - Report bugs or request features

## License

Apache License 2.0 - see [LICENSE](LICENSE) for details.

---

<div align="center">
  <a href="docs/developers/quick-start.md">Get Started</a> •
  <a href="docs/developers/api-reference.md">API Docs</a> •
  <a href="https://github.com/guidewire-oss/fern-platform/issues">Report Issue</a>
</div>