# Local Installation with k3d

This guide walks you through setting up Fern Platform locally using k3d for development and testing.

## Prerequisites

- Docker Desktop running
- Required tools:
  - `k3d` - Local Kubernetes clusters
  - `kubectl` - Kubernetes CLI
  - `helm` - Package manager for Kubernetes
  - `vela` - KubeVela CLI for application deployment
- Go 1.23+ (if building from source)

### Installing Prerequisites (macOS)
```bash
# Install via Homebrew
brew install k3d kubectl helm

# Install vela CLI
curl -fsSl https://kubevela.io/script/install.sh | bash
```

### Configure DNS Resolution
```bash
# Required for OAuth authentication to work properly
sudo sh -c 'echo "127.0.0.1 fern-platform.local" >> /etc/hosts'
sudo sh -c 'echo "127.0.0.1 keycloak" >> /etc/hosts'

# Verify entries
cat /etc/hosts | grep -E "fern-platform|keycloak"
```

## Environment Configuration

Before deploying, optionally set up JIRA integration (if using JIRA):

```bash
# Generate JIRA encryption key (only needed if using JIRA integration)
export JIRA_ENCRYPTION_KEY=$(openssl rand -hex 32)

# For production, store this securely and provide it to your deployment
```

## Quick Start

### 1. Setup Script

For the fastest setup, use our automated script:

```bash
make deploy-all
```

This will:
- Create a k3d cluster
- Install all dependencies
- Build and deploy Fern Platform
- Configure networking
- Set up JIRA integration (if JIRA_ENCRYPTION_KEY is configured)

### 2. Manual Setup

If you prefer manual control or the script fails:

#### Create Cluster
```bash
k3d cluster create fern-platform --port "8080:80@loadbalancer" --agents 2
```

#### Install Dependencies
```bash
# KubeVela
helm repo add kubevela https://charts.kubevela.net/core
helm upgrade --install --create-namespace -n vela-system kubevela kubevela/vela-core --wait

# CloudNativePG
helm repo add cnpg https://cloudnative-pg.io/charts
helm upgrade --install cnpg --namespace cnpg-system --create-namespace cnpg/cloudnative-pg --wait

# Component Definitions  
vela def apply cnpg.cue
vela def apply gateway.cue
```

#### Deploy Application
```bash
# Build and load image
make docker-build
k3d image import fern-platform:latest -c fern-platform

# Deploy using vela
kubectl create namespace fern-platform
vela up -f deployments/fern-platform-kubevela.yaml

# Check deployment status
vela status fern-platform -n fern-platform

# If workflow fails, resume it:
vela workflow resume fern-platform -n fern-platform
```

## Access the Application

1. **Verify hosts entries (should already be configured):**
   ```bash
   cat /etc/hosts | grep -E "fern-platform|keycloak"
   # Should show:
   # 127.0.0.1 fern-platform.local
   # 127.0.0.1 keycloak
   ```

2. **Access URLs:**
   - Fern Platform: http://fern-platform.local:8080
   - Keycloak Admin: http://keycloak:8080/admin

3. **Test Users:**
   - Admin: `admin@fern.com` / `admin123`
   - User: `user@fern.com` / `user123`

## Known Issues

**OAuth Invalid Scope Error:**
- Error: "invalid_scope: Invalid scopes: openid profile email"
- Cause: Keycloak 23.0 doesn't auto-create "profile" and "email" client scopes
- Workaround: Access Keycloak admin at http://keycloak:8080/admin (admin/admin123)
  1. Navigate to the fern-platform realm
  2. Go to Client scopes and create "profile" and "email" scopes
  3. Or modify the app to only request "openid" scope

**KubeVela Workflow Issues:**
- The workflow may fail at the Keycloak step with CUE evaluation errors
- This doesn't prevent the components from working
- Workaround: Resume the workflow:
  ```bash
  vela workflow resume fern-platform -n fern-platform
  ```

**Image Loading:**
- When building locally, the image is tagged as `fern-platform:latest` in k3d
- The deployment yaml may reference a different tag
- Fix: Either update the deployment or retag the image

## Troubleshooting

**Keycloak configuration issues:**
```bash
# Keycloak 23.0 has several breaking changes:
# 1. Removed fields:
#    - validPostLogoutRedirectUris: Now use attributes.post.logout.redirect.uris
#    - frontchannelLogoutUrl: Now configured via attributes
# 2. Password policy syntax changed:
#    - specialChars(1) is now invalid, causing "invalidPasswordMinSpecialCharsMessage" error
#    - Simplify to just "length(8)" or use new syntax
# 3. Client scopes:
#    - Must include "openid" in defaultClientScopes for OAuth to work
#    - Error: "invalid_scope: Invalid scopes: openid profile email"
# Fix: Update realm config in deployments/fern-platform-kubevela.yaml
```

**Deployment fails with "component not found":**
```bash
# Ensure component definitions are installed
vela comp
# If cloud-native-postgres or gateway are missing, reinstall:
vela def apply deployments/components/cnpg.cue
vela def apply deployments/components/gateway.cue
```

**Image not found:**
```bash
# Check image is loaded
docker exec k3d-fern-platform-server-0 crictl images | grep fern-platform
# Reload if needed
k3d image import anoop2811/fern-platform:latest -c fern-platform
```

**Can't access application:**
- Verify /etc/hosts contains the required entries
- Check ingress: `kubectl get ingress -n fern-platform`
- Check pods: `kubectl get pods -n fern-platform`

## Next Steps

- [Configure OAuth](../configuration/oauth.md) for different providers
- [Development Workflow](../developers/quick-start.md) for code changes
- [Production Deployment](./production.md) for real environments