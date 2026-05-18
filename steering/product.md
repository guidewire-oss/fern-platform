# Product Overview

## Purpose
Fern Platform aggregates test results from any CI/CD pipeline and framework (Jest, pytest, JUnit, Ginkgo, etc.) into a single dashboard. It detects flaky tests, tracks performance trends, and gives engineering teams visibility into test suite health.

## Target Users
- **Engineering managers** — track test health, flaky tests, and coverage across teams and projects.
- **Developers** — submit test results from their framework, drill into failures, and link tests to requirements.
- **QA / platform engineers** — operate the platform, configure projects, and integrate with PM tools.

## Key Features
- Universal test aggregation via REST API across frameworks and CI systems
- Flaky test detection (intermittent pass/fail patterns)
- Performance monitoring of test execution times
- Interactive visualizations (treemap for suite health)
- OAuth/SSO with team-based access control and role-based permissions
- GraphQL API for rich querying
- Project management connectors (in progress) — JIRA integration to map external requirements onto Fern test data

## Constraints
- Apache 2.0 licensed, open source — designs and specs are public
- Go 1.24+ backend, PostgreSQL 14+, Redis 6+
- Deployable via Kubernetes (KubeVela); Docker deployment planned
- OAuth/SSO required for non-trivial deployments
