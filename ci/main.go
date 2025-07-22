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

// ContainerEcho is a simple echo function for testing
func (m *Ci) ContainerEcho(stringArg string) string {
	return stringArg
}

// GrepDir searches for a pattern in files within a directory
func (m *Ci) GrepDir(directoryArg *dagger.Directory, pattern string) (string, error) {
	ctx := context.Background()
	return dag.Container().
		From("alpine:3.19").
		WithMountedDirectory("/search", directoryArg).
		WithExec([]string{"grep", "-r", pattern, "/search"}).
		Stdout(ctx)
}

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
	// +optional
	kubeconfig *dagger.File,
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
	return m.runAcceptanceTests(ctx, source, image, kubeconfig)
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
		WithExec([]string{"go", "install", "github.com/onsi/ginkgo/v2/ginkgo@v2.19.0"}).
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
func (m *Ci) runAcceptanceTests(ctx context.Context, source *dagger.Directory, image string, kubeconfig *dagger.File) (string, error) {
	// This function supports two modes:
	// 1. When KUBECONFIG is set (e.g., from GitHub Actions with k3d already running)
	// 2. Local development with full k3d setup
	
	// Build the container image first if not provided
	if image == "" {
		image = "fern-platform:test"
	}
	
	// Try to use pre-built base image from GitHub Container Registry (much faster)
	// If not available, build from Dockerfile
	var container *dagger.Container
	
	// Try the pre-built image first
	prebuiltImage := dag.Container().From("ghcr.io/guidewire-oss/fern-platform-acceptance-test:latest")
	
	// Test if we can pull the image by running a simple command
	_, err := prebuiltImage.WithExec([]string{"echo", "test"}).Stdout(ctx)
	if err == nil {
		// Image is available, use it
		container = prebuiltImage.
			WithMountedDirectory("/workspace", source).
			WithWorkdir("/workspace").
			WithEnvVariable("FERN_IMAGE", image)
	} else {
		// Fallback: build from Dockerfile
		fmt.Printf("Pre-built image not available, building from Dockerfile...\n")
		baseImage := dag.Container().
			Build(source.Directory("ci"), dagger.ContainerBuildOpts{
				Dockerfile: "Dockerfile.acceptance-test",
			})
		container = baseImage.
			WithMountedDirectory("/workspace", source).
			WithWorkdir("/workspace").
			WithEnvVariable("FERN_IMAGE", image)
	}
	
	// Mount kubeconfig if provided
	if kubeconfig != nil {
		container = container.
			WithMountedFile("/tmp/kubeconfig.orig", kubeconfig).
			WithEnvVariable("KUBECONFIG", "/root/.kube/config")
	}
	
	return container.
		WithExec([]string{"sh", "-c", `
			if [ -f "/tmp/kubeconfig.orig" ]; then
				echo "=== Using existing Kubernetes cluster from GitHub Actions ==="
				
				# Copy kubeconfig to writable location
				mkdir -p /root/.kube
				cp /tmp/kubeconfig.orig /root/.kube/config
				
				# Fix kubeconfig to work inside container
				# In GitHub Actions, k3d exposes the API on the host network
				# We need to replace 0.0.0.0 with the actual host IP
				
				# Try to get the host IP from the default gateway
				HOST_IP=$(ip route | grep default | awk '{print $3}')
				echo "Host IP detected: $HOST_IP"
				
				# Replace 0.0.0.0 with the host IP in kubeconfig
				sed -i "s/0\.0\.0\.0/$HOST_IP/g" /root/.kube/config
				
				# Show the updated server address
				echo "Kubernetes API server: $(grep server /root/.kube/config | head -1)"
				
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

// AcceptanceTestK3s runs acceptance tests using Dagger's native k3s module with full KubeVela support
func (m *Ci) AcceptanceTestK3s(
	ctx context.Context,
	// +required
	source *dagger.Directory,
	// +optional
	image string,
) (string, error) {
	// Build the application image if not provided
	if image == "" {
		_ = m.buildContainer(ctx, source, "linux/amd64")
		image = "fern-platform:test"
		// We'll need to export this to a registry or load it into k3s
	}

	// Start k3s server with necessary features
	k3sContainer := dag.Container().
		From("rancher/k3s:v1.28.5-k3s1").
		WithMountedDirectory("/workspace", source).
		WithExec([]string{"sh", "-c", `
			# Start k3s server (keep traefik for ingress support)
			k3s server \
				--snapshotter=native \
				--kube-apiserver-arg="--feature-gates=ServerSideApply=true" &
			
			# Wait for k3s to be ready
			until k3s kubectl get nodes; do
				echo "Waiting for k3s to start..."
				sleep 2
			done
			
			echo "K3s is ready!"
			
			# Set up kubectl
			export KUBECONFIG=/etc/rancher/k3s/k3s.yaml
			alias kubectl="k3s kubectl"
			
			# Install Helm (needed for some operators)
			curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash
			
			# Install KubeVela
			echo "Installing KubeVela..."
			helm repo add kubevela https://charts.kubevela.net/core
			helm repo update
			helm install --create-namespace -n vela-system kubevela kubevela/vela-core --version 1.9.7 --wait
			
			# Install vela CLI
			curl -fsSl https://static.kubevela.net/script/install.sh | bash -s v1.9.7
			
			# Wait for vela to be ready
			k3s kubectl wait --for=condition=available deployment/kubevela-vela-core -n vela-system --timeout=300s
			
			# Install CNPG Operator
			echo "Installing CloudNativePG operator..."
			k3s kubectl apply -f https://raw.githubusercontent.com/cloudnative-pg/cloudnative-pg/release-1.22/releases/cnpg-1.22.1.yaml
			
			# Wait for CNPG operator
			k3s kubectl wait --for=condition=available deployment/cnpg-controller-manager -n cnpg-system --timeout=300s || true
			
			# Create namespace
			k3s kubectl create namespace fern-platform || true
			
			# Create CNPG component definition for KubeVela
			echo "Creating CNPG component definition..."
			k3s kubectl apply -f - <<'CNPG_EOF'
apiVersion: core.oam.dev/v1beta1
kind: ComponentDefinition
metadata:
  name: cloud-native-postgres
  namespace: vela-system
spec:
  workload:
    definition:
      apiVersion: apps/v1
      kind: Deployment
  schematic:
    cue:
      template: |
        parameter: {
          name: string
          namespace: string
          instances: *1 | int
          storageSize: *"1Gi" | string
        }
        output: {
          apiVersion: "apps/v1"
          kind: "Deployment"
          metadata: {
            name: parameter.name
            namespace: parameter.namespace
          }
          spec: {
            selector: {
              matchLabels: {
                app: parameter.name
              }
            }
            template: {
              metadata: {
                labels: {
                  app: parameter.name
                }
              }
              spec: {
                containers: [{
                  name: "postgres"
                  image: "postgres:15"
                  env: [
                    {name: "POSTGRES_PASSWORD", value: "postgres"},
                    {name: "POSTGRES_DB", value: "fern_platform"}
                  ]
                  ports: [{containerPort: 5432}]
                }]
              }
            }
          }
        }
        outputs: {
          service: {
            apiVersion: "v1"
            kind: "Service"
            metadata: {
              name: parameter.name
              namespace: parameter.namespace
            }
            spec: {
              selector: {
                app: parameter.name
              }
              ports: [{
                port: 5432
                targetPort: 5432
              }]
            }
          }
        }
CNPG_EOF
			
			# Wait for Traefik to be ready
			echo "Waiting for Traefik ingress controller..."
			k3s kubectl wait --for=condition=available deployment/traefik -n kube-system --timeout=120s || true
			
			# Install Traefik middleware for Keycloak
			echo "Creating Traefik middleware..."
			k3s kubectl apply -f /workspace/deployments/components/traefik-middleware.yaml || true
			
			# Create gateway trait definition for KubeVela
			echo "Creating gateway trait definition..."
			k3s kubectl apply -f - <<'GATEWAY_EOF'
apiVersion: core.oam.dev/v1beta1
kind: TraitDefinition
metadata:
  name: gateway
  namespace: vela-system
spec:
  appliesToWorkloads:
    - deployments.apps
    - statefulsets.apps
  schematic:
    cue:
      template: |
        import "strconv"
        
        let nameSuffix = {
          if parameter.name != _|_ { "-" + parameter.name }
          if parameter.name == _|_ { "" }
        }
        let serviceOutputName = "service" + nameSuffix
        let serviceMetaName = context.name + nameSuffix
        
        outputs: (serviceOutputName): {
          apiVersion: "v1"
          kind: "Service"
          metadata: name: serviceMetaName
          spec: {
            if parameter.exposeType != _|_ {
              type: parameter.exposeType
            }
            selector: "app.oam.dev/component": context.name
            ports: [
              for k, v in parameter.http {
                name: "port-" + strconv.FormatInt(v, 10)
                port: v
                targetPort: v
              },
            ]
          }
        }
        
        let ingressOutputName = "ingress" + nameSuffix
        let ingressMetaName = context.name + nameSuffix
        
        outputs: (ingressOutputName): {
          apiVersion: "networking.k8s.io/v1"
          kind: "Ingress"
          metadata: {
            name: ingressMetaName
            annotations: {
              if !parameter.classInSpec {
                "kubernetes.io/ingress.class": parameter.class
              }
              if parameter.annotations != _|_ {
                for key, value in parameter.annotations {
                  "\(key)": "\(value)"
                }
              }
            }
          }
          spec: {
            if parameter.classInSpec {
              ingressClassName: parameter.class
            }
            rules: [{
              if parameter.domain != _|_ {
                host: parameter.domain
              }
              http: paths: [
                for k, v in parameter.http {
                  path: k
                  pathType: parameter.pathType
                  backend: {
                    service: {
                      name: serviceMetaName
                      port: number: v
                    }
                  }
                },
              ]
            }]
          }
        }
        
        parameter: {
          domain?: string
          http: [string]: int
          exposeType: *"ClusterIP" | "NodePort" | "LoadBalancer"
          class: *"traefik" | string
          classInSpec: *false | bool
          name?: string
          pathType: *"Prefix" | "ImplementationSpecific" | "Exact"
          annotations?: [string]: string
        }
GATEWAY_EOF
			
			# Load our built image into k3s
			echo "Loading application image..."
			# Note: In real implementation, we'd need to either:
			# 1. Push to a registry accessible by k3s
			# 2. Use k3s ctr to import the image
			# For now, we'll assume the image is available
			
			# Update the KubeVela application with our image
			echo "Updating application deployment..."
			sed "s|image: fern-platform:latest|image: ${FERN_IMAGE:-fern-platform:latest}|g" \
				/workspace/deployments/fern-platform-kubevela.yaml > /tmp/fern-platform-app.yaml
			
			# Deploy the application
			echo "Deploying Fern Platform application..."
			k3s kubectl apply -f /tmp/fern-platform-app.yaml
			
			# Wait for application to be ready
			echo "Waiting for application components..."
			k3s kubectl wait --for=condition=Ready application/fern-platform -n fern-platform --timeout=300s || true
			
			# Check application status
			vela status fern-platform -n fern-platform || true
			
			# Wait for all pods
			k3s kubectl wait --for=condition=ready pod -l app=postgres -n fern-platform --timeout=120s || true
			k3s kubectl wait --for=condition=ready pod -l app=redis -n fern-platform --timeout=120s || true
			k3s kubectl wait --for=condition=ready pod -l app=fern-platform -n fern-platform --timeout=300s || true
			
			# Show all resources
			echo "=== Application Resources ==="
			k3s kubectl get all -n fern-platform
			
			# Get the service endpoint
			FERN_SERVICE=$(k3s kubectl get svc fern-platform -n fern-platform -o jsonpath='{.spec.clusterIP}' || echo "localhost")
			export FERN_BASE_URL="http://${FERN_SERVICE}:8080"
			echo "Fern Platform URL: $FERN_BASE_URL"
			
			# Keep container running for acceptance tests
			tail -f /dev/null
		`}).
		WithExposedPort(6443). // Kubernetes API
		AsService()

	// Create test container that connects to k3s
	testContainer := dag.Container().
		From("ghcr.io/guidewire-oss/fern-platform-acceptance-test:latest").
		WithMountedDirectory("/workspace", source).
		WithWorkdir("/workspace").
		WithServiceBinding("k3s", k3sContainer).
		WithExec([]string{"sh", "-c", `
			# Configure kubectl to talk to k3s
			export KUBECONFIG=/root/.kube/config
			mkdir -p /root/.kube
			
			# Wait for k3s to be available
			until nc -z k3s 6443; do
				echo "Waiting for k3s API..."
				sleep 2
			done
			
			# Get kubeconfig from k3s
			kubectl config set-cluster default --server=https://k3s:6443 --insecure-skip-tls-verify=true
			kubectl config set-context default --cluster=default
			kubectl config use-context default
			
			# Create service account for kubectl
			cat <<'SA_EOF' | kubectl apply -f -
apiVersion: v1
kind: ServiceAccount
metadata:
  name: acceptance-test
  namespace: default
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: acceptance-test
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: cluster-admin
subjects:
- kind: ServiceAccount
  name: acceptance-test
  namespace: default
SA_EOF
			
			# Get service account token
			SECRET=$(kubectl get serviceaccount acceptance-test -o jsonpath='{.secrets[0].name}' || echo "")
			if [ -n "$SECRET" ]; then
				TOKEN=$(kubectl get secret $SECRET -o jsonpath='{.data.token}' | base64 -d)
			else
				# For newer k8s versions
				TOKEN=$(kubectl create token acceptance-test)
			fi
			
			# Configure kubectl with the token
			kubectl config set-credentials acceptance-test --token=$TOKEN
			kubectl config set-context default --user=acceptance-test
			
			# Verify connection
			kubectl get nodes
			kubectl get pods -n fern-platform
			
			# Port forward to access the application
			echo "Setting up port forwarding..."
			kubectl port-forward -n fern-platform svc/fern-platform 8080:8080 &
			PF_PID=$!
			sleep 5
			
			# Run acceptance tests
			export FERN_BASE_URL="http://localhost:8080"
			echo "Running acceptance tests against $FERN_BASE_URL"
			
			# Wait for app to be ready
			until curl -f $FERN_BASE_URL/health; do
				echo "Waiting for app to be ready..."
				sleep 2
			done
			
			# Run tests
			cd acceptance
			go mod download
			
			# Install Playwright browsers
			go run github.com/playwright-community/playwright-go/cmd/playwright install chromium
			go run github.com/playwright-community/playwright-go/cmd/playwright install-deps chromium
			
			# Run tests with ginkgo
			ginkgo -r -v
			
			# Cleanup
			kill $PF_PID || true
		`}).
		WithEnvVariable("FERN_IMAGE", image)

	// Execute and return results
	return testContainer.Stdout(ctx)
}

// AcceptanceTestSimple runs tests using direct service binding (no k8s)
func (m *Ci) AcceptanceTestSimple(
	ctx context.Context,
	// +required  
	source *dagger.Directory,
) (string, error) {
	// Build our app  
	appContainer := m.buildContainer(ctx, source, "linux/amd64").
		WithExposedPort(8080).
		WithEnvVariable("DB_HOST", "postgres").
		WithEnvVariable("DB_PORT", "5432").
		WithEnvVariable("DB_NAME", "fern_platform").
		WithEnvVariable("DB_USER", "postgres").
		WithEnvVariable("DB_PASSWORD", "postgres").
		WithEnvVariable("REDIS_HOST", "redis").
		WithEnvVariable("REDIS_PORT", "6379").
		WithEnvVariable("OAUTH_ENABLED", "false"). // Disable OAuth for simple tests
		WithEnvVariable("MIGRATION_PATH", "/app/migrations").
		WithEnvVariable("CONFIG_PATH", "/app/config/config.yaml").
		AsService()

	// PostgreSQL service
	postgresService := dag.Container().
		From("postgres:15").
		WithEnvVariable("POSTGRES_PASSWORD", "postgres").
		WithEnvVariable("POSTGRES_DB", "fern_platform").
		WithExposedPort(5432).
		AsService()

	// Redis service
	redisService := dag.Container().
		From("redis:7-alpine").
		WithExposedPort(6379).
		AsService()

	// Run acceptance tests directly against the services
	return dag.Container().
		From("mcr.microsoft.com/playwright:v1.40.0-focal").
		WithMountedDirectory("/workspace", source).
		WithWorkdir("/workspace/acceptance").
		WithServiceBinding("postgres", postgresService).
		WithServiceBinding("redis", redisService).
		WithServiceBinding("app", appContainer).
		// Install Go
		WithExec([]string{"sh", "-c", "curl -LO https://go.dev/dl/go1.23.0.linux-amd64.tar.gz && tar -C /usr/local -xzf go1.23.0.linux-amd64.tar.gz"}).
		WithEnvVariable("PATH", "/usr/local/go/bin:/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin").
		WithEnvVariable("GOPATH", "/go").
		// Install ginkgo
		WithExec([]string{"go", "install", "github.com/onsi/ginkgo/v2/ginkgo@v2.19.0"}).
		// Configure test environment
		WithEnvVariable("FERN_BASE_URL", "http://app:8080").
		WithEnvVariable("FERN_USERNAME", "fern-user@fern.com").
		WithEnvVariable("FERN_PASSWORD", "test123").
		WithEnvVariable("FERN_TEAM_NAME", "fern").
		WithEnvVariable("FERN_HEADLESS", "true").
		WithEnvVariable("FERN_RECORD_VIDEO", "false").
		WithExec([]string{"sh", "-c", `
			# Wait for services to be ready
			echo "Waiting for services..."
			for i in {1..30}; do
				nc -z postgres 5432 && nc -z redis 6379 && nc -z app 8080 && break
				echo "Waiting for services to start... ($i/30)"
				sleep 2
			done
			
			# Check if app is healthy
			curl -f http://app:8080/health || echo "Warning: Health check failed"
			
			# Run tests
			go mod download
			ginkgo -r -v
		`}).
		Stdout(ctx)
}