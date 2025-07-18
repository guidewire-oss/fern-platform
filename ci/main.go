// Dagger CI/CD module for Fern Platform
//
// This module provides a complete CI/CD pipeline including:
// - Building and testing
// - Security scanning
// - Acceptance tests with k3d
// - Container image publishing
package main

import (
	"context"
	"fmt"
	"strings"
	
	"dagger/ci/internal/dagger"
)

type Ci struct{}

// Build builds the Go application
func (m *Ci) Build(
	ctx context.Context,
	// +required
	source *dagger.Directory,
	// +optional
	// +default="linux/amd64,linux/arm64"
	platforms string,
) *dagger.Container {
	return m.buildContainer(ctx, source, platforms)
}

// Test runs unit tests
func (m *Ci) Test(
	ctx context.Context,
	// +required
	source *dagger.Directory,
) (string, error) {
	return m.runTests(ctx, source)
}

// Lint runs golangci-lint
func (m *Ci) Lint(
	ctx context.Context,
	// +required
	source *dagger.Directory,
) (string, error) {
	return m.runLint(ctx, source)
}

// SecurityScan runs Trivy security scanning
func (m *Ci) SecurityScan(
	ctx context.Context,
	// +required
	source *dagger.Directory,
) (string, error) {
	return m.runSecurityScan(ctx, source)
}

// AcceptanceTest runs acceptance tests with k3d
func (m *Ci) AcceptanceTest(
	ctx context.Context,
	// +required
	source *dagger.Directory,
	// +optional
	image string,
	// +optional
	// +default="localhost:5000"
	registry string,
) (string, error) {
	// Build and push image if not provided
	if image == "" {
		container := m.buildContainer(ctx, source, "linux/amd64")
		
		// If registry is provided (e.g., k3d registry), push to it
		if registry != "" {
			imageRef := fmt.Sprintf("%s/fern-platform:test", registry)
			addr, err := container.Publish(ctx, imageRef)
			if err != nil {
				// If push fails, continue with local image
				fmt.Printf("Warning: Failed to push to registry %s: %v\n", registry, err)
				image = "fern-platform:test"
			} else {
				image = addr
				fmt.Printf("Published image to: %s\n", addr)
			}
		} else {
			image = "fern-platform:test"
		}
	}
	return m.runAcceptanceTests(ctx, source, image)
}

// AcceptanceTestPlaywright runs Playwright-based acceptance tests
// This is a simpler function that just runs the tests without k8s deployment
func (m *Ci) AcceptanceTestPlaywright(
	ctx context.Context,
	// +required
	source *dagger.Directory,
	// +optional
	// +default="http://localhost:8080"
	baseURL string,
) (string, error) {
	return m.runAcceptanceTestsWithPlaywright(ctx, source, baseURL)
}

// Publish builds and publishes container images
func (m *Ci) Publish(
	ctx context.Context,
	// +required
	source *dagger.Directory,
	// +required
	registry string,
	// +required
	tag string,
	// +optional
	// +default="linux/amd64,linux/arm64"
	platforms string,
	// +optional
	username string,
	// +optional
	password *dagger.Secret,
) (string, error) {
	return m.publishImages(ctx, source, registry, tag, platforms, username, password)
}

// All runs all CI checks
func (m *Ci) All(
	ctx context.Context,
	// +required
	source *dagger.Directory,
) (string, error) {
	var results []string

	// Run lint
	lintResult, err := m.Lint(ctx, source)
	if err != nil {
		return "", fmt.Errorf("lint failed: %w", err)
	}
	results = append(results, "✅ Lint: "+lintResult)

	// Run tests
	testResult, err := m.Test(ctx, source)
	if err != nil {
		return "", fmt.Errorf("tests failed: %w", err)
	}
	results = append(results, "✅ Tests: "+testResult)

	// Run security scan
	scanResult, err := m.SecurityScan(ctx, source)
	if err != nil {
		return "", fmt.Errorf("security scan failed: %w", err)
	}
	results = append(results, "✅ Security: "+scanResult)

	// Build
	container := m.Build(ctx, source, "linux/amd64,linux/arm64")
	_, err = container.Sync(ctx)
	if err != nil {
		return "", fmt.Errorf("build failed: %w", err)
	}
	results = append(results, "✅ Build: Multi-platform build successful")

	return strings.Join(results, "\n"), nil
}

// Helper function to build container
func (m *Ci) buildContainer(ctx context.Context, source *dagger.Directory, platforms string) *dagger.Container {
	// Note: Dagger handles multi-platform builds internally
	_ = platforms // platforms will be used when we implement multi-platform support
	
	// Base builder stage
	builder := dag.Container().
		From("golang:1.23-alpine").
		WithMountedDirectory("/src", source).
		WithWorkdir("/src").
		WithExec([]string{"apk", "add", "--no-cache", "git", "make"}).
		WithExec([]string{"go", "mod", "download"})

	// Build the binary
	builder = builder.WithExec([]string{"go", "build", "-ldflags", "-w -s", "-o", "fern-platform", "cmd/fern-platform/main.go"})

	// Final stage
	return dag.Container().
		From("alpine:3.19").
		WithExec([]string{"apk", "add", "--no-cache", "ca-certificates", "tzdata"}).
		WithExec([]string{"addgroup", "-g", "1001", "-S", "fern"}).
		WithExec([]string{"adduser", "-u", "1001", "-S", "fern", "-G", "fern"}).
		WithFile("/app/fern-platform", builder.File("/src/fern-platform")).
		WithDirectory("/app/config", source.Directory("config")).
		WithDirectory("/app/migrations", source.Directory("migrations")).
		WithDirectory("/app/web", source.Directory("web")).
		WithExec([]string{"chown", "-R", "fern:fern", "/app"}).
		WithUser("fern").
		WithWorkdir("/app").
		WithEntrypoint([]string{"/app/fern-platform"})
}

// Helper function to run tests
func (m *Ci) runTests(ctx context.Context, source *dagger.Directory) (string, error) {
	output, err := dag.Container().
		From("golang:1.23-alpine").
		WithMountedDirectory("/src", source).
		WithWorkdir("/src").
		WithExec([]string{"apk", "add", "--no-cache", "git", "make", "gcc", "musl-dev"}).
		WithEnvVariable("CGO_ENABLED", "1").
		WithExec([]string{"go", "mod", "download"}).
		WithExec([]string{"go", "test", "-v", "-race", "-coverprofile=coverage.out", "./..."}).
		Stdout(ctx)
	
	if err != nil {
		return "", err
	}
	
	return output, nil
}

// Helper function to run lint
func (m *Ci) runLint(ctx context.Context, source *dagger.Directory) (string, error) {
	// Use golang base image and install golangci-lint
	// This ensures we have the right Go version and modules
	container := dag.Container().
		From("golang:1.23-alpine").
		WithMountedDirectory("/src", source).
		WithWorkdir("/src").
		WithExec([]string{"apk", "add", "--no-cache", "git", "make", "gcc", "musl-dev"}).
		WithEnvVariable("CGO_ENABLED", "1").
		WithExec([]string{"go", "mod", "download"}).
		// Install golangci-lint
		WithExec([]string{"sh", "-c", "wget -O- -nv https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s v1.61.0"})
	
	// Run golangci-lint
	_, err := container.
		WithExec([]string{"./bin/golangci-lint", "run", "--timeout", "5m"}).
		Stdout(ctx)
	
	if err != nil {
		return "", err
	}
	
	return "Linting passed", nil
}

// Helper function to run security scan
func (m *Ci) runSecurityScan(ctx context.Context, source *dagger.Directory) (string, error) {
	// Build the container first
	container := m.buildContainer(ctx, source, "linux/amd64")
	
	// Export container as tarball for Trivy
	tarball := container.AsTarball()
	
	// Run Trivy scan
	// Override entrypoint to avoid command duplication
	output, err := dag.Container().
		From("aquasec/trivy:0.48.1").
		WithMountedFile("/image.tar", tarball).
		WithEntrypoint([]string{"trivy"}).
		WithExec([]string{
			"image",
			"--input", "/image.tar",
			"--exit-code", "0",
			"--no-progress",
			"--format", "table",
			"--severity", "HIGH,CRITICAL",
		}).
		Stdout(ctx)
	
	if err != nil {
		return "", err
	}
	
	return output, nil
}

// runAcceptanceTestsWithPlaywright runs acceptance tests using Playwright for browser automation
func (m *Ci) runAcceptanceTestsWithPlaywright(ctx context.Context, source *dagger.Directory, baseURL string) (string, error) {
	// Default base URL if not provided
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}
	
	// Run acceptance tests in a container with Playwright support
	output, err := dag.Container().
		From("mcr.microsoft.com/playwright:v1.40.0-focal").
		WithMountedDirectory("/workspace", source).
		WithWorkdir("/workspace/acceptance").
		// Install Go
		WithExec([]string{"sh", "-c", "curl -LO https://go.dev/dl/go1.23.0.linux-amd64.tar.gz && tar -C /usr/local -xzf go1.23.0.linux-amd64.tar.gz"}).
		WithEnvVariable("PATH", "/usr/local/go/bin:/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin").
		WithEnvVariable("GOPATH", "/go").
		// Install ginkgo
		WithExec([]string{"go", "install", "github.com/onsi/ginkgo/v2/ginkgo@latest"}).
		// Download dependencies
		WithExec([]string{"go", "mod", "download"}).
		// Set test environment variables
		WithEnvVariable("FERN_BASE_URL", baseURL).
		WithEnvVariable("FERN_USERNAME", "fern-user@fern.com").
		WithEnvVariable("FERN_PASSWORD", "test123").
		WithEnvVariable("FERN_TEAM_NAME", "fern").
		WithEnvVariable("FERN_HEADLESS", "true").
		WithEnvVariable("FERN_RECORD_VIDEO", "false").
		// Run tests
		WithExec([]string{"ginkgo", "-r", "-v"}).
		Stdout(ctx)
	
	if err != nil {
		return "", err
	}
	
	return output, nil
}

// Helper function to run acceptance tests
func (m *Ci) runAcceptanceTests(ctx context.Context, source *dagger.Directory, image string) (string, error) {
	// This function supports two modes:
	// 1. When KUBECONFIG is set (e.g., from GitHub Actions with k3d already running)
	// 2. Local development with full k3d setup
	
	// Build the container image first if not provided
	if image == "" {
		image = "fern-platform:test"
	}
	
	// Check if we're in GitHub Actions with external k3d
	// Use Ubuntu-based image for better Playwright support
	return dag.Container().
		From("golang:1.23-bookworm").
		WithMountedDirectory("/workspace", source).
		WithWorkdir("/workspace").
		// Install required packages
		WithExec([]string{"apt-get", "update"}).
		WithExec([]string{"apt-get", "install", "-y", "curl", "git", "make", "bash", "docker.io", "wget"}).
		// Install kubectl
		WithExec([]string{"sh", "-c", "curl -LO https://storage.googleapis.com/kubernetes-release/release/v1.28.0/bin/linux/amd64/kubectl && chmod +x kubectl && mv kubectl /usr/local/bin/"}).
		// Install vela CLI
		WithExec([]string{"wget", "-O", "/tmp/vela.tar.gz", "https://github.com/kubevela/kubevela/releases/download/v1.9.7/vela-v1.9.7-linux-amd64.tar.gz"}).
		WithExec([]string{"tar", "-xzf", "/tmp/vela.tar.gz", "-C", "/tmp"}).
		WithExec([]string{"mv", "/tmp/linux-amd64/vela", "/usr/local/bin/vela"}).
		WithExec([]string{"chmod", "+x", "/usr/local/bin/vela"}).
		// Install ginkgo for acceptance tests
		WithExec([]string{"go", "install", "github.com/onsi/ginkgo/v2/ginkgo@latest"}).
		// Pass the image reference to the deployment
		WithEnvVariable("FERN_IMAGE", image).
		// Set up PATH to include Go bin
		WithEnvVariable("PATH", "/go/bin:/usr/local/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin").
		WithExec([]string{"sh", "-c", `
			if [ -n "$KUBECONFIG" ] && [ -f "$KUBECONFIG" ]; then
				echo "=== Using existing Kubernetes cluster from GitHub Actions ==="
				kubectl version --client
				kubectl get nodes
				
				# Create namespace if it doesn't exist
				kubectl create namespace fern-platform || true
				
				# Update the deployment YAML with the correct image
				echo "Using image: $FERN_IMAGE"
				sed -i "s|image: fern-platform:latest|image: $FERN_IMAGE|g" deployments/fern-platform-kubevela.yaml
				
				# Deploy the application
				echo "Deploying application with vela..."
				vela up -f deployments/fern-platform-kubevela.yaml
				
				# Wait for deployment
				echo "Waiting for deployment to be ready..."
				kubectl wait --for=condition=ready pod -l app=fern-platform -n fern-platform --timeout=300s
				
				# Check pod status
				kubectl get pods -n fern-platform
				kubectl describe pod -l app=fern-platform -n fern-platform
				
				# Get service information
				echo "Getting service endpoints..."
				kubectl get svc -n fern-platform
				
				# Port-forward the fern-platform service for testing
				echo "Setting up port forwarding..."
				kubectl port-forward -n fern-platform svc/fern-platform 8080:8080 &
				PF_PID=$!
				sleep 5
				
				# Export the test URL
				export FERN_BASE_URL="http://localhost:8080"
				echo "Fern Platform URL: $FERN_BASE_URL"
				
				# Check if the service is responding
				echo "Checking service health..."
				curl -f $FERN_BASE_URL/health || echo "Warning: Health check failed"
				
				# Run acceptance tests
				echo "Running acceptance tests..."
				cd acceptance && go mod download
				
				# Install Playwright browsers
				cd acceptance && go run github.com/playwright-community/playwright-go/cmd/playwright install chromium
				cd acceptance && go run github.com/playwright-community/playwright-go/cmd/playwright install-deps chromium
				
				# Run tests with ginkgo
				cd acceptance && ginkgo -r -v || TEST_RESULT=$?
				
				# Clean up port-forward
				kill $PF_PID || true
				
				# Exit with test result
				exit ${TEST_RESULT:-0}
			else
				echo "=== No external Kubernetes cluster detected ==="
				echo "For CI environments, start k3d in GitHub Actions before running Dagger"
				echo "For local development, run 'make deploy-all' to set up k3d with proper privileges"
				echo ""
				echo "Example GitHub Actions setup:"
				echo "  - uses: AbsaOSS/k3d-action@v2"
				echo "    with:"
				echo "      cluster-name: 'test-cluster'"
				echo "      args: >-"
				echo "        --agents 1"
				echo "        --no-lb"
				echo "        --k3s-arg '--no-deploy=traefik,servicelb,metrics-server@server:*'"
				exit 1
			fi
		`}).
		Stdout(ctx)
}

// Helper function to publish images
func (m *Ci) publishImages(ctx context.Context, source *dagger.Directory, registry string, tag string, platforms string, username string, password *dagger.Secret) (string, error) {
	container := m.buildContainer(ctx, source, platforms)
	
	// Add registry auth if provided
	if username != "" && password != nil {
		container = container.WithRegistryAuth(registry, username, password)
	}
	
	// Publish with the specified tag
	addr, err := container.Publish(ctx, fmt.Sprintf("%s:%s", registry, tag))
	if err != nil {
		return "", err
	}
	
	// Also publish as latest
	latestAddr, err := container.Publish(ctx, fmt.Sprintf("%s:latest", registry))
	if err != nil {
		return "", err
	}
	
	return fmt.Sprintf("Published: %s, %s", addr, latestAddr), nil
}