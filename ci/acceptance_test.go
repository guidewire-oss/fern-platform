package main

import (
	"context"
	"fmt"
	"time"
	
	"dagger/ci/internal/dagger"
)

// AcceptanceTestAdvanced provides more control over acceptance testing
type AcceptanceTestAdvanced struct {
	// Test configuration
	ClusterName string
	Namespace   string
	Timeout     time.Duration
}

// RunWithK3d runs acceptance tests using k3d in a more controlled manner
func (m *Ci) RunWithK3d(
	ctx context.Context,
	// +required
	source *dagger.Directory,
	// +optional
	// +default="test-cluster"
	clusterName string,
	// +optional
	// +default="fern-platform"
	namespace string,
) (string, error) {
	// Build the application image first
	appImage := m.buildContainer(ctx, source, "linux/amd64")
	
	// Create a custom test runner that includes everything needed
	testRunner := dag.Container().
		From("alpine:3.19").
		// Install Docker CLI and dependencies
		WithExec([]string{"apk", "add", "--no-cache",
			"docker-cli", "docker-cli-compose", "bash", "curl", "make", "go", "nodejs", "npm", "git",
		}).
		// Install k3d
		WithExec([]string{"sh", "-c", `
			curl -s https://raw.githubusercontent.com/k3d-io/k3d/main/install.sh | TAG=v5.6.0 bash
		`}).
		// Install kubectl
		WithExec([]string{"sh", "-c", `
			curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl" &&
			chmod +x kubectl &&
			mv kubectl /usr/local/bin/
		`}).
		// Install Helm
		WithExec([]string{"sh", "-c", `
			curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash
		`}).
		// Install KubeVela CLI
		WithExec([]string{"sh", "-c", `
			curl -fsSl https://static.kubevela.net/script/install.sh | bash
		`}).
		WithMountedDirectory("/workspace", source).
		WithWorkdir("/workspace")

	// Use Docker service from Dagger
	dockerd := dag.Container().
		From("docker:24-dind").
		WithMountedCache("/var/lib/docker", dag.CacheVolume("docker-lib")).
		WithExposedPort(2375).
		WithExec([]string{"dockerd", 
			"--host=tcp://0.0.0.0:2375",
			"--host=unix:///var/run/docker.sock",
			"--storage-driver=overlay2",
		}).
		AsService()

	// Load the application image into Docker
	imageArchive := appImage.AsTarball()
	
	// Run the acceptance tests
	result, err := testRunner.
		WithServiceBinding("docker", dockerd).
		WithEnvVariable("DOCKER_HOST", "tcp://docker:2375").
		WithFile("/tmp/app-image.tar", imageArchive).
		// Create test script
		WithNewFile("/workspace/run-acceptance-tests.sh", fmt.Sprintf(`#!/bin/bash
set -e

echo "Waiting for Docker daemon..."
for i in {1..30}; do
    if docker version >/dev/null 2>&1; then
        echo "Docker is ready!"
        break
    fi
    echo "Waiting for Docker... ($i/30)"
    sleep 1
done

echo "Loading application image..."
docker load -i /tmp/app-image.tar
docker tag $(docker images -q | head -n 1) fern-platform:test

echo "Creating k3d cluster..."
k3d cluster create %s \
    --api-port 6550 \
    --servers 1 \
    --agents 2 \
    --port "8080:80@loadbalancer" \
    --wait

echo "Importing image to k3d..."
k3d image import fern-platform:test -c %s

echo "Creating namespace..."
kubectl create namespace %s || true

echo "Deploying PostgreSQL..."
kubectl apply -n %s -f - <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: postgres-app
type: Opaque
stringData:
  host: postgres
  port: "5432"
  username: postgres
  password: postgres
  dbname: fern_platform
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: postgres
spec:
  selector:
    matchLabels:
      app: postgres
  template:
    metadata:
      labels:
        app: postgres
    spec:
      containers:
      - name: postgres
        image: postgres:15-alpine
        env:
        - name: POSTGRES_PASSWORD
          value: postgres
        - name: POSTGRES_DB
          value: fern_platform
        ports:
        - containerPort: 5432
---
apiVersion: v1
kind: Service
metadata:
  name: postgres
spec:
  selector:
    app: postgres
  ports:
  - port: 5432
EOF

echo "Deploying Redis..."
kubectl apply -n %s -f - <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: redis
spec:
  selector:
    matchLabels:
      app: redis
  template:
    metadata:
      labels:
        app: redis
    spec:
      containers:
      - name: redis
        image: redis:7-alpine
        ports:
        - containerPort: 6379
---
apiVersion: v1
kind: Service
metadata:
  name: redis
spec:
  selector:
    app: redis
  ports:
  - port: 6379
EOF

echo "Waiting for dependencies..."
kubectl wait --for=condition=available --timeout=60s deployment/postgres -n %s
kubectl wait --for=condition=available --timeout=60s deployment/redis -n %s

echo "Deploying Fern Platform..."
kubectl apply -n %s -f - <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: fern-platform
spec:
  selector:
    matchLabels:
      app: fern-platform
  template:
    metadata:
      labels:
        app: fern-platform
    spec:
      containers:
      - name: fern-platform
        image: fern-platform:test
        imagePullPolicy: Never
        env:
        - name: LOG_LEVEL
          value: debug
        - name: REDIS_HOST
          value: redis
        - name: REDIS_PORT
          value: "6379"
        - name: AUTH_ENABLED
          value: "false"
        - name: DB_HOST
          valueFrom:
            secretKeyRef:
              name: postgres-app
              key: host
        - name: DB_PORT
          valueFrom:
            secretKeyRef:
              name: postgres-app
              key: port
        - name: DB_USER
          valueFrom:
            secretKeyRef:
              name: postgres-app
              key: username
        - name: DB_PASSWORD
          valueFrom:
            secretKeyRef:
              name: postgres-app
              key: password
        - name: DB_NAME
          valueFrom:
            secretKeyRef:
              name: postgres-app
              key: dbname
        ports:
        - containerPort: 8080
---
apiVersion: v1
kind: Service
metadata:
  name: fern-platform
spec:
  selector:
    app: fern-platform
  ports:
  - port: 8080
    targetPort: 8080
EOF

echo "Waiting for Fern Platform to be ready..."
kubectl wait --for=condition=available --timeout=120s deployment/fern-platform -n %s

echo "Setting up port forward..."
kubectl port-forward -n %s service/fern-platform 8080:8080 &
PF_PID=$!

echo "Waiting for service to be accessible..."
for i in {1..30}; do
    if curl -f http://localhost:8080/health >/dev/null 2>&1; then
        echo "Service is ready!"
        break
    fi
    echo "Waiting for service... ($i/30)"
    sleep 1
done

echo "Running acceptance tests..."
cd /workspace
if [ -f "acceptance/run_tests.sh" ]; then
    ./acceptance/run_tests.sh
else
    echo "Running Go acceptance tests..."
    go test -v ./acceptance/...
fi

# Cleanup
kill $PF_PID || true
`, clusterName, clusterName, namespace, namespace, namespace, namespace, namespace, namespace, namespace, namespace)).
		WithExec([]string{"bash", "/workspace/run-acceptance-tests.sh"}).
		Stdout(ctx)

	if err != nil {
		return "", fmt.Errorf("acceptance tests failed: %w", err)
	}

	return result, nil
}