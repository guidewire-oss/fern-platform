# Fern Platform

[![Go Version](https://img.shields.io/badge/go-1.23+-00ADD8.svg?style=flat-square&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg?style=flat-square)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/guidewire-oss/fern-platform?style=flat-square)](https://goreportcard.com/report/github.com/guidewire-oss/fern-platform)
[![codecov](https://codecov.io/gh/guidewire-oss/fern-platform/branch/main/graph/badge.svg)](https://codecov.io/gh/guidewire-oss/fern-platform)
[![CI Status](https://img.shields.io/github/actions/workflow/status/guidewire-oss/fern-platform/ci.yml?branch=main&label=CI&style=flat-square)](https://github.com/guidewire-oss/fern-platform/actions/workflows/ci.yml)

A unified test intelligence platform that transforms fragmented test data into actionable insights.

## What is Fern Platform?

Fern Platform aggregates test results from any CI/CD pipeline and testing framework (Jest, pytest, JUnit, etc.) into a centralized dashboard. It automatically detects flaky tests, tracks performance trends, and provides the visibility engineering teams need to maintain healthy test suites.

Think of it as a specialized analytics platform for your tests - like Datadog or Grafana, but purpose-built for test intelligence. **We're on a mission to make test failures predictable and preventable through AI-powered insights.**

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

Choose based on your installation method:

**For Docker:**
- Docker Engine 20.10+
- PostgreSQL 14+ (external or containerized)
- Redis 6+ (external or containerized)

**For Kubernetes deployment:**
- Docker with buildx
- [k3d](https://k3d.io/stable/#installation) (lightweight Kubernetes)
- kubectl
- Go 1.21+ (used by Makefile for architecture detection)
- Make
- 8GB RAM minimum

### Installation

Choose your preferred installation method:

#### Option 1: Docker (Coming Soon)

Docker images will be available after the v0.1.0 release:
- GitHub Container Registry: `ghcr.io/guidewire-oss/fern-platform:latest`
- Docker Hub: `docker.io/guidewireoss/fern-platform:latest`

```bash
# Future usage (not yet available):
docker run -d \
  --name fern-platform \
  -p 8080:8080 \
  -e DB_HOST=host.docker.internal \
  -e DB_USER=postgres \
  -e DB_PASSWORD=yourpassword \
  -e DB_NAME=fern_platform \
  -e REDIS_HOST=host.docker.internal \
  ghcr.io/guidewire-oss/fern-platform:latest
```

For now, please use Option 2 (Kubernetes deployment) or build from source.

#### Option 2: Kubernetes with OAuth (Full Features)

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

1. **Manager creates a project** in the Fern Platform UI
2. **Developers install a client library** for their test framework:

#### Official Client Libraries

- **Go/Ginkgo**: [fern-ginkgo-client](https://github.com/guidewire-oss/fern-ginkgo-client)
- **Java/JUnit**: [fern-junit-client](https://github.com/guidewire-oss/fern-junit-client) and [Gradle plugin](https://github.com/guidewire-oss/fern-junit-gradle-plugin)
- **JavaScript/Jest**: [fern-jest-client](https://github.com/guidewire-oss/fern-jest-client)

#### Build Your Own Client

Missing your framework? Create your own client library! See our [client development guide](docs/developers/integration-guide.md#building-your-own-client-library) to:
- Build clients for Python, Ruby, PHP, .NET, or any other language
- Integrate with pytest, RSpec, PHPUnit, NUnit, or any test framework
- Contribute back to the community

3. **Configure with your project ID**:

```bash
export FERN_PROJECT_ID=my-project
export FERN_URL=http://fern-platform.local:8080
```

Test results are automatically sent to Fern Platform!

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

### JavaScript/Jest

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

### Java/JUnit with Gradle

```gradle
plugins {
  id 'com.guidewire.fern' version '1.0.0'
}

fern {
  url = System.getenv('FERN_URL')
  projectId = System.getenv('FERN_PROJECT_ID')
}
```

### Go/Ginkgo

```go
import "github.com/guidewire-oss/fern-ginkgo-client/reporter"

var _ = ginkgo.BeforeSuite(func() {
  ginkgo.RunSpecs(t, "My Suite", reporter.NewFernReporter())
})
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

## The Vision: Where We're Heading

While Fern Platform already provides powerful test analytics, we're building towards something bigger: **an AI-powered test intelligence system that predicts failures before they happen**.

### 🚀 Coming Soon

**AI-Powered Intelligence** (In Development)
- Automatic root cause analysis for failures
- Predictive test failure detection
- Smart test selection for faster CI/CD
- Natural language queries: "Why did the auth tests fail last week?"

**Enhanced Integrations** (Q1 2025)
- Native plugins for Jest, pytest, Go, JUnit
- GitHub Actions & GitLab CI apps
- Slack/Teams notifications with insights
- JIRA auto-ticket creation for failures

**Real-Time Features** (Q2 2025)
- Live test execution monitoring
- WebSocket subscriptions for dashboards
- Streaming logs from CI/CD pipelines

See our [RFCs](docs/rfc/) for detailed technical proposals and join the [discussion](https://github.com/guidewire-oss/fern-platform/discussions).

## Project Status

Fern Platform is under active development with core features stable and used in production.

**Ready Now**: Test aggregation • Flaky detection • Performance tracking • OAuth • REST/GraphQL APIs  
**In Progress**: AI insights • Webhook integrations • Enhanced visualizations  
**Exploring**: ML-based test optimization • Distributed tracing for tests

## Contributing

We welcome contributions! See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

Areas where we need help:
- **Client libraries** for new test frameworks (pytest, RSpec, PHPUnit, etc.)
- Test framework integrations
- UI/UX improvements
- Documentation
- Bug fixes

### Creating Client Libraries

Building a client for your favorite test framework? Check our [client development guide](docs/developers/integration-guide.md#building-your-own-client-library) and join our growing ecosystem!

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