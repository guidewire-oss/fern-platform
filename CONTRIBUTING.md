# Contributing to Fern Platform

Thank you for your interest in contributing to Fern Platform! This guide will help you get started with development, testing, and submitting contributions.

## 🚀 Quick Start for Contributors

**Want to start contributing right away?** Here's the fastest path:

```bash
# 1. Fork and clone the repository
git clone https://github.com/YOUR_USERNAME/fern-platform.git
cd fern-platform

# 2. Deploy everything with one command
make deploy-all

# 3. Start developing! The app will be running at http://localhost:8080
```

**That's it!** The `make deploy-all` command handles everything:
- ✅ Creates k3d cluster
- ✅ Installs all prerequisites  
- ✅ Builds and deploys the application
- ✅ Opens your browser automatically

For detailed instructions, continue reading below.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Environment Setup](#development-environment-setup)
- [Local Deployment with KubeVela](#local-deployment-with-kubevela)
- [Testing](#testing)
- [Development Workflow](#development-workflow)
- [Pull Request Process](#pull-request-process)
- [Style Guidelines](#style-guidelines)
- [Architecture Guidelines](#architecture-guidelines)

## Code of Conduct

By participating in this project, you agree to abide by our Code of Conduct. Please treat all contributors with respect and create a welcoming environment for everyone.

## Getting Started

### Prerequisites

Ensure you have the following tools installed:

- **Go 1.21+** - [Installation guide](https://golang.org/doc/install)
- **Docker** - [Installation guide](https://docs.docker.com/get-docker/)
- **kubectl** - [Installation guide](https://kubernetes.io/docs/tasks/tools/)
- **k3d** - [Installation guide](https://k3d.io/v5.4.6/#installation)
- **Helm** - [Installation guide](https://helm.sh/docs/intro/install/)
- **KubeVela CLI** - [Installation guide](https://kubevela.io/docs/installation/kubernetes#install-vela-cli)

### Fork and Clone

1. **Fork** the repository on GitHub
2. **Clone** your fork locally:
   ```bash
   git clone https://github.com/YOUR_USERNAME/fern-platform.git
   cd fern-platform
   ```
3. **Add upstream** remote:
   ```bash
   git remote add upstream https://github.com/guidewire-oss/fern-platform.git
   ```

## Development Environment Setup

## Local Deployment with KubeVela

### Quick Start (Recommended)

The fastest way to get Fern Platform running locally is using our automated deployment:

```bash
# One command to deploy everything
make deploy-all
```

This single command will:
1. Check/create k3d cluster with proper configuration
2. Install prerequisites (KubeVela, CloudNativePG) if needed
3. Build and import Docker image
4. Deploy the complete application stack
5. Start port forwarding and open browser automatically

**That's it!** The application will open in your browser at http://localhost:8080.

> **Note**: If you plan to use OAuth authentication with Keycloak, you'll need to configure DNS entries. See [Networking and DNS Configuration](docs/developers/networking-and-dns.md) for details.

#### Quick Start Commands

```bash
# Complete automated deployment
make deploy-all

# Quick deployment (assumes cluster and prerequisites exist)
make deploy-quick

# Stop port forwarding when done
make stop-port-forward

# Clean up everything
make k8s-delete    # Delete application
make k3d-delete    # Delete cluster
```

### Manual Step-by-Step Setup (Alternative)

If you prefer manual control or need to troubleshoot, you can follow these detailed steps:

#### 1. Create k3d Cluster

```bash
# Create cluster with proper port mapping for HTTP traffic
k3d cluster create fern-platform --port "8080:80@loadbalancer" --agents 2

# Verify cluster is running
kubectl cluster-info
```

#### 2. Install KubeVela Core

```bash
# Add KubeVela Helm repository
helm repo add kubevela https://kubevela.github.io/charts
helm repo update

# Install KubeVela with proper timeout
helm install --create-namespace -n vela-system kubevela kubevela/vela-core --wait --timeout=10m

# Verify installation
kubectl get pods -n vela-system
```

#### 3. Install CloudNativePG Operator

```bash
# Add CloudNativePG Helm repository
helm repo add cnpg https://cloudnative-pg.github.io/charts
helm repo update

# Install the operator
helm upgrade --install cnpg --namespace cnpg-system --create-namespace cnpg/cloudnative-pg --wait --timeout=10m

# Verify installation
kubectl get pods -n cnpg-system
```

#### 4. Deploy Custom Component Definitions

```bash
# Apply PostgreSQL component definition
vela def apply deployments/components/cnpg.cue

# Apply Gateway component definition (optional)
vela def apply deployments/components/gateway.cue

# Verify component definitions are installed
vela def list
```

#### 5. Build and Import Docker Image

```bash
# Build the Docker image
make docker-build

# Import the image into k3d cluster (required for local deployment)
k3d image import fern-platform:latest -c fern-platform
```

#### 6. Deploy Fern Platform Application

```bash
# Create namespace
kubectl create namespace fern-platform

# Deploy the application
kubectl apply -f deployments/fern-platform-kubevela.yaml

# Check initial status (will be suspending)
vela status fern-platform -n fern-platform
```

#### 7. Resume Workflow and Monitor Deployment

```bash
# Resume the workflow (it suspends after infrastructure deployment)
vela workflow resume fern-platform -n fern-platform

# Monitor deployment progress
watch kubectl get pods -n fern-platform

# Check detailed application status
vela status fern-platform -n fern-platform

# View application logs
kubectl logs -n fern-platform deployment/fern-platform -f
```

#### 8. Access the Application

```bash
# Port forward to access the application
kubectl port-forward -n fern-platform svc/fern-platform 8080:8080

# Access in browser
open http://localhost:8080

# OR use the automated port forwarding
make start-port-forward-and-open
```

##### OAuth Authentication Setup (Optional)

If you're deploying with OAuth/Keycloak authentication:

1. **Add DNS entries** to `/etc/hosts`:
   ```bash
   sudo echo "127.0.0.1 keycloak" >> /etc/hosts
   sudo echo "127.0.0.1 fern-platform.local" >> /etc/hosts
   ```

2. **Access via configured domains**:
   - Application: http://fern-platform.local:8080
   - Keycloak Admin: http://keycloak:8080

3. **Test credentials**:
   - Admin: admin@fern.com / admin123
   - User: user@fern.com / user123

For detailed explanation, see [Networking and DNS Configuration](docs/developers/networking-and-dns.md).

### Verify Deployment

Check that all components are running:

```bash
# Check all pods are running
kubectl get pods -n fern-platform

# Expected output:
# NAME                             READY   STATUS    RESTARTS   AGE
# postgres-1                       1/1     Running   0          5m
# redis-xxxx-xxxxx                 1/1     Running   0          5m
# fern-platform-xxxx-xxxxx         1/1     Running   0          3m

# Check PostgreSQL cluster health
kubectl get clusters -n fern-platform

# Check services
kubectl get services -n fern-platform

# Test health endpoint
curl http://localhost:8080/health
```

### Troubleshooting Common Issues

#### Automated Deployment Issues

```bash
# If deployment fails, check individual components
make k8s-status                    # Check application status
kubectl get pods -A               # Check all pods
vela status fern-platform -n fern-platform  # Check KubeVela status

# Restart deployment if needed
make k8s-delete && make deploy-all

# Check logs
kubectl logs -n fern-platform deployment/fern-platform
kubectl logs -n vela-system deployment/kubevela-vela-core
kubectl logs -n cnpg-system deployment/cnpg-controller-manager

# Force cluster recreation
make k3d-delete && make deploy-all
```

#### Port Forwarding Issues

```bash
# Check if port forwarding is running
ps aux | grep "kubectl.*port-forward"

# Stop and restart port forwarding
make stop-port-forward
make start-port-forward-and-open

# Check if port 8080 is in use
lsof -i :8080
```

#### Application Not Starting

```bash
# Check pod logs
kubectl logs -n fern-platform deployment/fern-platform

# Check pod events
kubectl describe pod -n fern-platform $(kubectl get pods -n fern-platform | grep fern-platform | awk '{print $1}')

# Check application status
vela status fern-platform -n fern-platform
```

#### Database Connection Issues

```bash
# Check PostgreSQL cluster status
kubectl describe cluster postgres -n fern-platform

# Check database secrets
kubectl get secrets -n fern-platform | grep postgres

# Test database connectivity
kubectl exec -it postgres-1 -n fern-platform -- psql -U fern_user -d fern_platform -c "SELECT version();"
```

#### Workflow Stuck

```bash
# Check workflow status
vela workflow status fern-platform -n fern-platform

# Resume if suspended
vela workflow resume fern-platform -n fern-platform

# Restart workflow if needed
vela workflow restart fern-platform -n fern-platform
```

### Cleanup

```bash
# Quick cleanup using make commands
make k8s-delete     # Delete the application
make k3d-delete     # Delete the cluster

# OR manual cleanup
kubectl delete -f deployments/fern-platform-kubevela.yaml
k3d cluster delete fern-platform

# Stop port forwarding if running
make stop-port-forward
```

## Testing

### Unit Tests

```bash
# Run all unit tests
make test

# Run tests with coverage
make test-coverage

# Run tests for specific packages
go test ./internal/reporter/...
go test ./pkg/...
```

### Acceptance Tests

The project includes comprehensive acceptance tests using Ginkgo. These tests can run against either an existing deployed platform or create isolated test environments.

#### Prerequisites for Acceptance Tests

```bash
# Navigate to acceptance test directory
cd acceptance-go

# Install Go dependencies
go mod download

# Install Ginkgo CLI (if not already installed)
go install github.com/onsi/ginkgo/v2/ginkgo@latest
```

#### Fast Mode: Test Against Existing Platform

```bash
# Run all tests against existing platform (recommended for development)
make test-existing

# Run specific test suites
make test-api-existing          # API tests only
make test-integration-existing  # Integration tests only
make test-ui-existing          # UI tests only

# Run with verbose output
make test-existing VERBOSE=true
```

#### Isolated Mode: Full k3d Environment

```bash
# Run tests with complete isolation (creates fresh k3d cluster)
make test-isolated

# Run specific test suites in isolated mode
make test-api                   # API tests with k3d
make test-integration          # Integration tests with k3d
make test-ui                   # UI tests with k3d
```

#### Understanding Test Configuration

The acceptance tests support two execution modes controlled by the `USE_EXISTING_PLATFORM` flag:

- **Fast Mode** (`USE_EXISTING_PLATFORM=true`): Tests run against your locally deployed platform
- **Isolated Mode** (`USE_EXISTING_PLATFORM=false`): Tests create and manage their own k3d environment

### Test Organization

```
acceptance-go/
├── specs/
│   ├── api/          # REST and GraphQL API tests
│   ├── integration/  # End-to-end workflow tests
│   └── ui/           # Web interface tests
├── pkg/
│   ├── clients/      # API client implementations
│   ├── fixtures/     # Test data management
│   └── k8s/          # Kubernetes test utilities
└── test-results/     # Test output and reports
```

### Writing New Tests

When adding new features, include tests at appropriate levels:

1. **Unit Tests**: Test individual functions and methods
2. **Integration Tests**: Test component interactions
3. **Acceptance Tests**: Test complete user workflows

Example acceptance test structure:

```go
var _ = Describe("New Feature", func() {
    var (
        ctx context.Context
        client *reporter.Client
    )

    BeforeEach(func() {
        ctx = GetTestContext()
        client = GetReporterClient()
    })

    It("should perform expected behavior", func() {
        // Test implementation
        // Use Ginkgo/Gomega assertions
        Expect(result).To(Equal(expected))
    })
})
```

## Development Workflow

### Quick Development Setup

For rapid development iteration:

```bash
# Initial setup
make deploy-all

# Make code changes, then quick rebuild and deploy
make deploy-quick

# Stop when done
make stop-port-forward
```

### Branch Management

1. **Create feature branch** from `main`:
   ```bash
   git checkout main
   git pull upstream main
   git checkout -b feature/your-feature-name
   ```

2. **Set up development environment**:
   ```bash
   make deploy-all  # Complete setup with one command
   ```

3. **Make your changes** following the style guidelines

4. **Test thoroughly**:
   ```bash
   make test
   cd acceptance-go && make test-existing
   ```

5. **Test deployment** (optional):
   ```bash
   make deploy-quick  # Quick test of changes
   ```

6. **Commit with conventional commits**:
   ```bash
   git add .
   git commit -m "feat(component): add new feature description"
   ```

7. **Push and create PR**:
   ```bash
   git push origin feature/your-feature-name
   ```

### Conventional Commits

We use [Conventional Commits](https://www.conventionalcommits.org/) for clear project history:

- `feat(scope): add new feature`
- `fix(scope): fix bug`
- `docs(scope): update documentation`
- `test(scope): add or update tests`
- `refactor(scope): refactor code`
- `chore(scope): maintenance tasks`

Examples:
```bash
git commit -m "feat(ui): add treemap visualization component"
git commit -m "fix(api): resolve database connection timeout"
git commit -m "docs(readme): update deployment instructions"
```

### Code Quality

Before submitting:

```bash
# Format code
make fmt

# Run linter
make lint

# Run all tests
make test
cd acceptance-go && make test-existing

# Build to ensure no compilation errors
make build
```

## Pull Request Process

### Before Opening a PR

1. **Rebase** on latest main:
   ```bash
   git fetch upstream
   git rebase upstream/main
   ```

2. **Run full test suite**:
   ```bash
   make test
   cd acceptance-go && make test-existing
   ```

3. **Verify deployment** works:
   ```bash
   # Test automated deployment
   make deploy-all

   # Or quick test if cluster exists
   make deploy-quick

   # Clean up when done
   make stop-port-forward
   ```

### PR Requirements

- **Clear description** of changes and motivation
- **Tests included** for new functionality
- **Documentation updated** if needed
- **Conventional commit** format
- **No breaking changes** without major version bump
- **All CI checks** must pass

### PR Template

```markdown
## Description
Brief description of changes

## Type of Change
- [ ] Bug fix
- [ ] New feature
- [ ] Documentation update
- [ ] Refactoring
- [ ] Performance improvement

## Testing
- [ ] Unit tests added/updated
- [ ] Acceptance tests added/updated
- [ ] Manual testing completed

## Checklist
- [ ] Code follows style guidelines
- [ ] Self-review completed
- [ ] Documentation updated
- [ ] Tests pass locally
```

## Style Guidelines

### Go Code Style

- Follow [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- Use `gofmt` and `golint`
- Write clear, descriptive variable and function names
- Include godoc comments for public APIs
- Use structured logging with appropriate levels

### File Organization

```
internal/
├── reporter/           # Reporter module
│   ├── api/           # HTTP handlers
│   ├── service/       # Business logic
│   └── repository/    # Data access
├── mycelium/          # AI module
└── ui/                # UI components

pkg/                   # Shared utilities
├── config/            # Configuration
├── database/          # Database utilities
├── middleware/        # HTTP middleware
└── logging/           # Logging utilities
```

### Testing Standards

- Use Ginkgo/Gomega for Go tests
- Test files should end with `_test.go`
- Include both positive and negative test cases
- Use table-driven tests for multiple similar cases
- Mock external dependencies appropriately

## Architecture Guidelines

### Design Principles

1. **Modular Design**: Clear separation of concerns between modules
2. **Dependency Injection**: Use interfaces for testability
3. **Configuration Management**: Environment-based configuration
4. **Error Handling**: Structured error handling with context
5. **Observability**: Comprehensive logging and metrics

### Adding New Features

When adding new features:

1. **Design First**: Consider how it fits into existing architecture
2. **Interface Definition**: Define clear interfaces for new components
3. **Database Changes**: Use migrations for schema changes
4. **API Design**: Follow REST conventions and GraphQL best practices
5. **Documentation**: Update API docs and user guides

### Database Migrations

```bash
# Create new migration
migrate create -ext sql -dir migrations -seq add_new_feature

# Apply migrations
migrate -path migrations -database "postgres://..." up

# Rollback if needed
migrate -path migrations -database "postgres://..." down 1
```

### Adding API Endpoints

1. **Define routes** in appropriate handler files
2. **Add business logic** in service layer
3. **Update OpenAPI** documentation
4. **Add integration tests** for new endpoints
5. **Update GraphQL schema** if needed

## Getting Help

- **GitHub Issues**: For bug reports and feature requests
- **GitHub Discussions**: For questions and general discussion
- **Documentation**: Check existing docs in `/docs` directory

## Recognition

Contributors will be recognized in:
- GitHub contributors list
- Release notes for significant contributions
- Project documentation for major features

Thank you for contributing to Fern Platform! 🌿