<div align="center">
  <img src="https://github.com/guidewire-oss/fern-platform/blob/main/docs/images/logo-color.png" alt="Fern Platform" width="200"/>
  
  # 🌿 Fern Platform
  
  <!-- Note: Some badges require initial setup:
       - OpenSSF Best Practices: Register at https://www.bestpractices.dev/
       - Codecov: Set up at https://codecov.io/ and add CODECOV_TOKEN secret
       - Docker Hub: Update pulls badge if using different registry
  -->

  **Transform your test chaos into intelligent insights with AI-powered test analysis**

  *Stop drowning in test data. Start understanding what your tests are telling you.*

  [![Go Version](https://img.shields.io/badge/go-1.23+-00ADD8.svg?style=flat-square&logo=go)](https://golang.org)
  [![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg?style=flat-square)](LICENSE)
  [![Go Report Card](https://goreportcard.com/badge/github.com/guidewire-oss/fern-platform?style=flat-square)](https://goreportcard.com/report/github.com/guidewire-oss/fern-platform)
  [![codecov](https://codecov.io/gh/guidewire-oss/fern-platform/branch/main/graph/badge.svg?token=CODECOV_TOKEN)](https://codecov.io/gh/guidewire-oss/fern-platform)
  
  [![OpenSSF Scorecard](https://api.securityscorecards.dev/projects/github.com/guidewire-oss/fern-platform/badge)](https://securityscorecards.dev/viewer/?uri=github.com/guidewire-oss/fern-platform)
  [![OpenSSF Best Practices](https://www.bestpractices.dev/projects/9999/badge)](https://www.bestpractices.dev/en/projects/9999)
  [![SLSA 3](https://slsa.dev/images/gh-badge-level3.svg)](https://slsa.dev)
  
  [![CI Status](https://img.shields.io/github/actions/workflow/status/guidewire-oss/fern-platform/ci.yml?branch=main&label=CI&style=flat-square)](https://github.com/guidewire-oss/fern-platform/actions/workflows/ci.yml)
  [![Release](https://img.shields.io/github/v/release/guidewire-oss/fern-platform?style=flat-square&logo=github)](https://github.com/guidewire-oss/fern-platform/releases)
  [![Docker Pulls](https://img.shields.io/docker/pulls/guidewire/fern-platform?style=flat-square&logo=docker)](https://hub.docker.com/r/guidewire/fern-platform)
  [![Dependency Status](https://img.shields.io/librariesio/github/guidewire-oss/fern-platform?style=flat-square)](https://libraries.io/github/guidewire-oss/fern-platform)
  
  [![Development Status](https://img.shields.io/badge/status-active%20development-orange.svg?style=flat-square)](#-project-status)
  [![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg?style=flat-square)](CONTRIBUTING.md)
  [![GitHub Stars](https://img.shields.io/github/stars/guidewire-oss/fern-platform?style=social)](https://github.com/guidewire-oss/fern-platform/stargazers)

  <p align="center">
    <a href="docs/developers/quick-start.md">🚀 Quick Start</a> •
    <a href="docs/workflows/README.md">📖 How to Use</a> •
    <a href="docs/README.md">📚 Documentation</a> •
    <a href="docs/product/overview.md">🎯 Why Fern?</a> •
    <a href="#-demo">🎬 Demo</a> •
    <a href="#-community">💬 Community</a>
  </p>
</div>

## ⚠️ Project Status

**Fern Platform is under active development.** While the core functionality is working and stable, we're continuously adding features and improvements.

### 🔨 **Current Status**
- ✅ **Core features stable**: Test data ingestion, OAuth authentication, web dashboard
- ✅ **Production ready**: Used by teams for test reporting and analysis
- 🚧 **Active development**: Regular updates, new features, and improvements
- 🚧 **API evolution**: APIs may change as we add capabilities

### 🚀 **Production Usage**
- **✅ Recommended for**: Development teams, staging environments, non-critical test reporting
- **⚠️ Use with care for**: Mission-critical production systems
- **📊 Monitor**: Keep backups, test upgrades in staging first

### 💬 **We Need Your Feedback!**
As an actively developed project, your feedback is invaluable:
- 🐛 **Found a bug?** [Report it](../../issues/new?labels=bug)
- 💡 **Have ideas?** [Share them](../../discussions/new?category=ideas)
- 🤝 **Want to contribute?** [Join us](CONTRIBUTING.md)
- 📈 **Using in production?** [Tell us about it](../../discussions/new?category=show-and-tell)

## 🎯 Why Choose Fern Platform?

Every engineering team struggles with the same problems:
- 🔥 **Flaky tests** that waste CI time and developer confidence
- 📊 **Test data scattered** across multiple tools and dashboards  
- 🤔 **No visibility** into test trends, failures, or team productivity
- 🔍 **Manual debugging** of test failures without context

**Fern Platform solves this** by providing a unified test intelligence platform that consolidates your test data into actionable insights.

### ✨ What Makes Fern Special

| 📊 **Unified Intelligence** | 🔧 **Developer-First** | 🏢 **Enterprise-Ready** |
|-------------------|------------------------|-------------------------|
| Multi-framework test consolidation | 15-minute k3d setup | OAuth/SSO with any provider |
| Rich test metadata and trends | Multi-framework support | Role-based access control |
| Interactive data visualization | Rich APIs (REST + GraphQL) | Production-grade security |

```bash
# Get started in 3 commands (requires k3d + kubectl)
git clone https://github.com/guidewire-oss/fern-platform
cd fern-platform
make deploy-all  # ← You'll have a running platform in 15 minutes!
```

## 🎬 Demo

<div align="center">
  <img src="docs/images/fern-platform-demo.gif" alt="Fern Platform Demo" width="800"/>
  
  *See Fern Platform in action: From test chaos to intelligent insights in minutes*
</div>

### 🌟 Key Features Available Now

- **🎯 Interactive Treemap**: Visualize all your projects' test health at a glance
- **📊 Real-time Dashboards**: Live test statistics and trends
- **🔍 Deep Drill-Down**: From high-level overview to individual test details
- **👥 Team Collaboration**: Role-based access and project management
- **🔐 OAuth Integration**: Secure authentication with any OAuth 2.0 provider

### 🚧 Planned AI Features (Coming Soon)

- **🤖 Flaky Test Detection**: Statistical analysis to identify unreliable tests
- **📈 Failure Pattern Analysis**: Automatic categorization of test failures
- **💡 Smart Recommendations**: AI-powered suggestions for test improvements

## 🚀 Quick Start

Choose your setup path based on your environment:

### 🔥 **15 Minutes** - Local Development Setup
```bash
# Prerequisites: Docker, k3d, kubectl, helm
# Complete setup with OAuth, database, and test data
git clone https://github.com/guidewire-oss/fern-platform
cd fern-platform
make deploy-all  # Installs k3d cluster, deploys everything
# Visit http://fern-platform.local:8080

# Test users and credentials are available after deployment
# See docs/configuration/test-users.md for login details

# Note: You'll be prompted to add entries to /etc/hosts for OAuth to work
```

### 🏢 **30 Minutes** - Production Kubernetes Deployment
```bash
# Deploy to your existing Kubernetes cluster
kubectl apply -f deployments/fern-platform-kubevela.yaml
# See docs/operations/production-setup.md for details
```

### 💻 Cross-Platform Support

Fern Platform supports multiple operating systems and architectures:

- **Linux**: AMD64, ARM64
- **macOS**: Intel (AMD64), Apple Silicon (ARM64)  
- **Windows**: AMD64, ARM64

```bash
# Build for all platforms
make build-all

# Build multi-arch Docker images
make docker-build-multi

# Build for specific platform
GOOS=linux GOARCH=arm64 make build
```

**[📖 Detailed setup guides for all scenarios →](docs/developers/quick-start.md)**

## 🛠️ What Can You Build?

Fern Platform is designed for extensibility. Here are some examples of what teams have built:

### 🔌 **Current Integrations**
- **CI/CD Pipelines**: Compatible with any CI/CD system that can make HTTP requests
- **Test Frameworks**: Ginkgo ([client available](https://github.com/guidewire-oss/fern-ginkgo-client)), JUnit ([client](https://github.com/guidewire-oss/fern-junit-client) & [Gradle plugin](https://github.com/guidewire-oss/fern-junit-gradle-plugin)), Jest ([client available](https://github.com/guidewire-oss/fern-jest-client))
- **Monitoring**: API endpoints for integration with monitoring tools
- **OAuth Providers**: Any OAuth 2.0/OpenID Connect provider

### 🤖 **Planned AI Features** (Roadmap)
- **Smart Notifications**: AI-filtered alerts for critical failures only
- **Failure Categorization**: Automatic grouping of similar test failures
- **Test Optimization**: Suggestions for improving test reliability
- **Predictive Analysis**: Identify tests likely to become flaky

### 📊 **Custom Analytics**
- **Team Dashboards**: Per-team test health and productivity metrics
- **Executive Reports**: High-level quality trends and business impact
- **Performance Analysis**: Test execution time trends and bottlenecks
- **Coverage Insights**: Visual test coverage gaps and improvements

```go
// Example: Custom test analyzer plugin
type FlakinessPredictorPlugin struct {
    client *fern.Client
}

func (p *FlakinessPredictorPlugin) Analyze(testRun *TestRun) *Prediction {
    // Your custom AI/ML logic here
    return &Prediction{
        Confidence: 0.85,
        Suggestion: "This test may become flaky due to timing issues",
    }
}
```

**[🚀 See the full API documentation →](docs/developers/api-reference.md)**

## 💬 Community

Join thousands of developers already using Fern Platform:

### 🤝 **Get Involved**
- ⭐ **Star this repo** if you find Fern Platform useful
- 🐛 **Report bugs** via [GitHub Issues](../../issues)
- 💡 **Suggest features** in [GitHub Discussions](../../discussions)
- 🔄 **Contribute code** - see our [Contributing Guide](CONTRIBUTING.md)

### 📞 **Get Help**
- 📖 **Documentation**: [Complete guides](docs/README.md) for all use cases
- 💬 **Community Chat**: [GitHub Discussions](../../discussions) for questions
- 🎬 **Video Tutorials**: Coming soon
- 📧 **Enterprise Support**: Contact for commercial support options

### 🏆 **Who's Using Fern Platform**

Fern Platform is actively used by development teams for:
- **Test result consolidation** across multiple CI/CD pipelines
- **Historical test analysis** and trend tracking
- **Team collaboration** on test quality improvements
- **OAuth-integrated dashboards** for secure test data access

**[📝 Share how you're using Fern Platform →](../../discussions/categories/show-and-tell)**

## 📖 Documentation Hub

Our documentation is organized by your role and needs:

### 🎯 **For Product & Business Teams**
- **[🌟 Product Overview](docs/product/overview.md)** - Business value and use cases
- **[📊 UI Enhancements](docs/UI_ENHANCEMENTS.md)** - Modern dashboard features
- **[🏗️ Architecture](docs/ARCHITECTURE.md)** - Technical design and principles

### 🔧 **For Developers & Engineers**  
- **[🚀 Quick Start Guide](docs/developers/quick-start.md)** - Get running in 15 minutes
- **[🔐 OAuth Configuration](docs/configuration/oauth.md)** - Authentication setup
- **[📊 REST API Reference](docs/developers/api-reference.md)** - RESTful endpoints
- **[📈 GraphQL API](docs/graphql-api.md)** - GraphQL schema and queries
- **[🌐 Networking & DNS](docs/developers/networking-and-dns.md)** - Local DNS setup

### 🏢 **For Platform & Operations Teams**
- **[🐳 Local k3d Installation](docs/installation/local-k3d.md)** - Kubernetes local setup
- **[🏗️ Architecture Overview](docs/ARCHITECTURE.md)** - System design
- **[📋 RFCs](docs/rfc/)** - Design proposals and future plans

**[📚 Browse all documentation →](docs/README.md)**

## 🚀 Technology & Architecture

Fern Platform is built on modern, battle-tested technologies:

### 🛠️ **Core Technologies**
- **Backend**: Go + Gin framework for high performance
- **Frontend**: React + TypeScript for modern UX
- **Database**: PostgreSQL with comprehensive test data models
- **Authentication**: OAuth 2.0/OpenID Connect with any provider
- **Deployment**: Kubernetes-native with KubeVela
- **Future**: AI/ML integration planned (Claude, OpenAI, local models)

### 🏗️ **Architecture Principles**
- **Domain-Driven Design**: Organized around business domains (testing, projects, auth)
- **Unified Monolith**: Single deployment, modular internals
- **API-First**: Rich REST + GraphQL APIs for integration
- **Cloud-Native**: Container-first, Kubernetes-optimized
- **Extensible**: Clean architecture enables easy feature addition

```
┌─────────────────────────────────────────────────────────┐
│                    Fern Platform                        │
├─────────────────┬─────────────────┬─────────────────────┤
│   Test Reporter │   AI Analysis   │    Web Dashboard    │
│   (Data Layer)  │   (ML Layer)    │    (UI Layer)       │
├─────────────────┴─────────────────┴─────────────────────┤
│              Shared Infrastructure                      │
│         (Auth, Config, Database, Logging)               │
└─────────────────────────────────────────────────────────┘
```

**[🏗️ Deep-dive into the architecture →](docs/ARCHITECTURE.md)**

## 🤝 Contributing

We love contributions from the community! Whether you're fixing bugs, adding features, or improving docs.

### 🌟 **Ways to Contribute**
- 🐛 **Fix bugs** - Check our [good first issues](../../issues?q=is%3Aissue+is%3Aopen+label%3A%22good+first+issue%22)
- ✨ **Add features** - See our [roadmap](../../projects) for ideas
- 📝 **Improve docs** - Help make Fern Platform easier to use
- 🧪 **Write tests** - Help us maintain quality
- 🎨 **Design & UX** - Make the platform more beautiful

### 🚀 **Quick Start for Contributors**
```bash
# 1. Fork and clone the repo
git clone https://github.com/YOUR_USERNAME/fern-platform
cd fern-platform

# 2. Set up development environment (15 minutes)
make dev-setup

# 3. Make your changes and test
make test

# 4. Submit a pull request
# See CONTRIBUTING.md for detailed guidelines
```

**[📋 Read the full Contributing Guide →](CONTRIBUTING.md)**

## 📄 License

Fern Platform is licensed under the [Apache License 2.0](LICENSE), meaning you can use it freely in your commercial and open source projects.

---

<div align="center">
  <p><strong>Ready to transform your test intelligence?</strong></p>
  
  <a href="docs/developers/quick-start.md">
    <img src="https://img.shields.io/badge/Get%20Started-15%20minutes-brightgreen?style=for-the-badge" alt="Get Started"/>
  </a>
  
  <p><em>⭐ Star this repo if you find Fern Platform useful!</em></p>
</div>