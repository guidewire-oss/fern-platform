# Fern Platform - Main Makefile
# Unified interface that imports modular Makefiles

# Import all modular Makefiles
include Makefile.core
include Makefile.test  
include Makefile.docker
include Makefile.k8s
include Makefile.ci

# Default target
.DEFAULT_GOAL := help

# Main workflow targets (override .PHONY from includes)
.PHONY: help all quick-start teardown

help: ## Display this help message
	@echo "🌿 Fern Platform Build System"
	@echo ""
	@echo "🚀 Quick Start:"
	@echo "  make setup-local     - Setup development environment"
	@echo "  make deploy-all      - Complete deployment (k3d + app)"
	@echo "  make dev            - Start development mode" 
	@echo "  make test           - Run unit tests"
	@echo ""
	@echo "📋 Available targets:"
	@echo ""
	@echo "🔨 Core Development:"
	@awk 'BEGIN {FS = ":.*?## "; section=""} /^# .* - / {section=$$0; gsub(/^# | - .*$$/, "", section)} /^[a-zA-Z_-]+:.*?## / && ($$0 ~ /deps|build|clean|dev|run|info/) {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}' Makefile.core
	@echo ""
	@echo "🧪 Testing:"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / && ($$0 ~ /test|coverage|fmt|lint|vet/) {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}' Makefile.test
	@echo ""
	@echo "🐳 Docker:"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / && ($$0 ~ /docker/) {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}' Makefile.docker
	@echo ""
	@echo "☸️  Kubernetes:"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / && ($$0 ~ /k3d|k8s|deploy/) {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}' Makefile.k8s
	@echo ""
	@echo "🤖 CI/CD:"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / && ($$0 ~ /ci-|setup-local/) {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}' Makefile.ci
	@echo ""
	@echo "🎯 Workflows:"
	@echo "  \033[36mquick-start\033[0m         Complete setup and deployment"
	@echo "  \033[36mteardown\033[0m            Clean up everything"
	@echo ""
	@echo "💡 Examples:"
	@echo "  make setup-local && make dev                    # Local development"
	@echo "  make deploy-all                                 # Full k3d deployment"
	@echo "  make test && make docker-build                  # Test and build"
	@echo "  make ci-all                                     # Full CI pipeline"
	@echo "  REGISTRY=myregistry.com make docker-push        # Push to registry"

all: ci-all ## Run complete CI pipeline (alias for ci-all)

quick-start: ## Complete setup and deployment workflow
	@echo "🌟 Fern Platform Quick Start"
	@echo "==========================="
	@echo ""
	@echo "This will set up everything you need:"
	@echo "1. Install development tools"
	@echo "2. Set up k3d cluster with prerequisites"
	@echo "3. Build and deploy the application"
	@echo "4. Run basic health checks"
	@echo ""
	@read -p "Continue? [y/N] " confirm && [ "$$confirm" = "y" ] || exit 1
	@echo ""
	@$(MAKE) setup-local
	@$(MAKE) deploy-all
	@echo ""
	@echo "🎉 Quick start completed!"
	@echo ""
	@echo "✅ Next steps:"
	@echo "  - Open http://fern-platform.local:8080 in your browser"
	@echo "  - Run 'make test-acceptance' to verify everything works"
	@echo "  - Run 'make dev' for development mode with live reload"
	@echo "  - Run 'make help' to see all available commands"

teardown: ## Clean up everything (cluster + local artifacts)
	@echo "🧹 Tearing down Fern Platform..."
	@echo ""
	@echo "This will:"
	@echo "1. Delete k3d cluster and all deployments"
	@echo "2. Clean up local build artifacts"
	@echo "3. Remove generated files"
	@echo ""
	@read -p "Continue? [y/N] " confirm && [ "$$confirm" = "y" ] || exit 1
	@echo ""
	@$(MAKE) k8s-delete || true
	@$(MAKE) k3d-delete || true
	@$(MAKE) clean
	@echo ""
	@echo "✅ Teardown completed!"
	@echo ""
	@echo "💡 To start fresh, run: make quick-start"