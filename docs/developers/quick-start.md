# 🚀 Quick Start Guide

**Get Fern Platform running in 15 minutes with OAuth authentication and sample data**

> ⚠️ **Development Status**: Fern Platform is under active development. Core features are stable and production-ready, but expect regular updates and API evolution. [See project status](../../README.md#️-project-status) for details.


## ⚡ Choose Your Path

| Time Available  | Method                                                  | Best For                            |
|-----------------|---------------------------------------------------------|-------------------------------------|
| **2 minutes**   | [Docker Quick Start](#-2-minute-docker-quick-start)     | Quick evaluation                    |
| **15 minutes**  | [Full Local Setup](#-15-minute-full-setup)              | Developers ready to explore         |
| **+5 minutes**  | [IDE Debugging](#-ide-debugging-extends-15-minute-setup) | Contributors who want breakpoints   |
| **30 minutes**  | [Production Ready](#-30-minute-production-setup)        | Teams preparing for deployment      |

---

## 🐳 2-Minute Docker Quick Start (Coming Soon)

**Perfect for:** Quick evaluation without full setup

> **Note**: Docker images are being published. Once available, you'll be able to use:
> - `ghcr.io/guidewire-oss/fern-platform:latest`
> - `docker.io/guidewireoss/fern-platform:latest`

### Prerequisites
- Docker Engine 20.10+
- PostgreSQL and Redis (local or cloud)

### Future Usage

```bash
# Once images are available:
docker run -d \
  --name fern-platform \
  -p 8080:8080 \
  -e DB_HOST=host.docker.internal \
  -e DB_USER=postgres \
  -e DB_PASSWORD=yourpassword \
  -e DB_NAME=fern_platform \
  -e REDIS_HOST=host.docker.internal \
  ghcr.io/guidewire-oss/fern-platform:latest

# Check health
curl http://localhost:8080/health

# View logs
docker logs -f fern-platform
```

### Docker Compose (All-in-One)

Save this as `docker-compose.yml`:

```yaml
version: '3.8'
services:
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_PASSWORD: postgres
      POSTGRES_DB: fern_platform
  redis:
    image: redis:7-alpine
  fern-platform:
    image: ghcr.io/guidewire-oss/fern-platform:latest
    ports:
      - "8080:8080"
    environment:
      DB_HOST: postgres
      DB_PASSWORD: postgres
      REDIS_HOST: redis
      # Optional: Only needed if using JIRA integration
      # Generate with: openssl rand -hex 32
      JIRA_ENCRYPTION_KEY: ""
    depends_on:
      - postgres
      - redis
```

Run: `docker-compose up -d`

> **Note**: Set `JIRA_ENCRYPTION_KEY` to a 64-character hex string (generated with `openssl rand -hex 32`) only if you plan to use JIRA integration. If not using JIRA, you can leave it empty.

Visit `http://localhost:8080`

---

## 🔥 15-Minute Full Setup

**Perfect for:** Developers who want to explore features and APIs

### Prerequisites (5 minutes)

#### 1. Install Required Tools

Fern Platform supports Linux, macOS, and Windows. Choose your installation method:

**macOS (Homebrew)**
```bash
brew install k3d kubectl helm

# Install vela CLI
curl -fsSl https://kubevela.io/script/install.sh | bash
```

**Linux**
```bash
# Install k3d
curl -s https://raw.githubusercontent.com/k3d-io/k3d/main/install.sh | bash

# Install kubectl
curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
sudo install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl

# Install helm
curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash

# Install vela CLI
curl -fsSl https://kubevela.io/script/install.sh | bash
```

**Windows (PowerShell as Administrator)**
```powershell
# Install Chocolatey if not present
Set-ExecutionPolicy Bypass -Scope Process -Force; [System.Net.ServicePointManager]::SecurityProtocol = [System.Net.ServicePointManager]::SecurityProtocol -bor 3072; iex ((New-Object System.Net.WebClient).DownloadString('https://community.chocolatey.org/install.ps1'))

# Install tools
choco install k3d kubernetes-cli kubernetes-helm

# Install vela CLI
Invoke-WebRequest -Uri "https://github.com/kubevela/kubevela/releases/latest/download/vela-windows-amd64.exe" -OutFile "vela.exe"
Move-Item vela.exe $env:WINDIR\system32\
```

**For other installation methods:**
- k3d: https://k3d.io/stable/#installation
- kubectl: https://kubernetes.io/docs/tasks/tools/
- helm: https://helm.sh/docs/intro/install/
- vela: https://kubevela.io/docs/installation/kubernetes#install-vela-cli

#### 2. Configure hosts file (Required for OAuth)

**macOS/Linux**
```bash
# Add these entries to /etc/hosts for OAuth authentication to work:
sudo sh -c 'echo "127.0.0.1 fern-platform.local" >> /etc/hosts'
sudo sh -c 'echo "127.0.0.1 keycloak" >> /etc/hosts'

# Verify entries were added:
cat /etc/hosts | grep -E "fern-platform|keycloak"
```

**Windows (PowerShell as Administrator)**
```powershell
# Add entries to hosts file
Add-Content -Path $env:windir\System32\drivers\etc\hosts -Value "`n127.0.0.1 fern-platform.local"
Add-Content -Path $env:windir\System32\drivers\etc\hosts -Value "127.0.0.1 keycloak"

# Verify entries were added
Get-Content $env:windir\System32\drivers\etc\hosts | Select-String "fern-platform|keycloak"
```

> ⚠️ **Important**: Without these /etc/hosts entries, OAuth login will fail with redirect errors.

### One-Command Setup (10 minutes)
```bash
# Clone and setup everything
git clone https://github.com/guidewire-oss/fern-platform
cd fern-platform

# This single command:
# 1. Creates k3d cluster with port mappings
# 2. Installs KubeVela and CloudNativePG operators
# 3. Builds Fern Platform from your local source and loads it into k3d
# 4. Deploys PostgreSQL, Keycloak, and Fern Platform (using your locally-built image)
# 5. Opens browser automatically
#
# To pick up code changes, just re-run `make deploy-all` — it rebuilds and redeploys.
make deploy-all

# After deployment completes, the browser will open to:
# http://fern-platform.local:8080

# Note: OAuth authentication requires the /etc/hosts entries configured above.
# See docs/developers/networking-and-dns.md for detailed explanation.
```

### Test Credentials
```bash
# Multiple test users are available with different permission levels
# See docs/configuration/test-users.md for complete list

# Example users:
Admin: admin@fern.com / test123 (full system access)
Manager: fern-manager@fern.com / test123 (can edit fern team projects)
User: fern-user@fern.com / test123 (read-only access to fern team)
```

### What You Get
- ✅ **Full OAuth flow** with Keycloak
- ✅ **Admin vs user roles** and permissions
- ✅ **Real database** with migrations
- ✅ **All APIs** (REST + GraphQL)
- ✅ **Modern web UI** with treemap visualization
- ✅ **Multi-framework test data** support
- 🚧 **AI features planned** (not yet implemented)

### Verify Everything Works
```bash
# 1. Check all pods are running
kubectl get pods -n fern-platform

# 2. Test health endpoint  
curl http://fern-platform.local:8080/health

# 3. Access the application
open http://fern-platform.local:8080

# Note: Use fern-platform.local:8080, not localhost:8080, for OAuth to work.
```

---

## 🐛 IDE Debugging (extends 15-minute setup)

**Perfect for:** Contributors who want to set breakpoints and step through Fern Platform code.

This builds on the 15-minute setup. After `make deploy-all` is running, you can run Fern Platform from your IDE on port 8081, reusing the cluster's Postgres and Keycloak. The k3d load balancer stays running throughout — do not stop it.

### Step 1: Add hosts entry for Postgres

The app resolves Postgres as `localhost.fern-platform` (Kubernetes DNS format). Add an alias so it resolves on your host:

```bash
sudo sh -c 'echo "127.0.0.1 localhost.fern-platform" >> /etc/hosts'
```

Verify all required entries are present:

```bash
cat /etc/hosts | grep -E "fern-platform|keycloak"
# Should show:
# 127.0.0.1 fern-platform.local
# 127.0.0.1 keycloak
# 127.0.0.1 localhost.fern-platform
```

### Step 2: Port-forward Postgres so the IDE can reach it

In a separate terminal (this command blocks; keep it open):

```bash
kubectl port-forward -n fern-platform svc/postgres-rw 5432:5432
```

### Step 3: Get the Postgres password

```bash
kubectl get secret -n fern-platform postgres-app -o jsonpath='{.data.password}' | base64 -d
```

Copy the output — you'll need it in the next step.

### Step 4: Update `config/config.yaml`

Update the `server`, `database`, and `auth` sections of `config/config.yaml`, replacing `<password>` with the value from Step 3. Run from the repo root so the config file is picked up correctly.

```yaml
server:
  port: 8081

database:
  host: "localhost"
  port: 5432
  user: "app"
  password: "<password from Step 3>"
  dbname: "app"
  sslmode: "disable"
  timezone: "UTC"
  maxOpenConns: 25
  maxIdleConns: 5
  connMaxLifetime: "300s"
  connMaxIdleTime: "300s"

auth:
  enabled: true
  jwksUrl: "http://keycloak:8080/realms/fern-platform/protocol/openid-connect/certs"
  issuer: "http://keycloak:8080/realms/fern-platform"
  audience: "fern-platform-web"
  oauth:
    enabled: true
    clientId: "fern-platform-web"
    clientSecret: "fern-platform-client-secret"
    redirectUrl: "http://localhost:8081/auth/callback"
    scopes: "openid,profile,email,groups"
    authUrl: "http://keycloak:8080/realms/fern-platform/protocol/openid-connect/auth"
    tokenUrl: "http://keycloak:8080/realms/fern-platform/protocol/openid-connect/token"
    userInfoUrl: "http://keycloak:8080/realms/fern-platform/protocol/openid-connect/userinfo"
    jwksUrl: "http://keycloak:8080/realms/fern-platform/protocol/openid-connect/certs"
    issuerUrl: "http://keycloak:8080/realms/fern-platform"
    logoutUrl: "http://keycloak:8080/realms/fern-platform/protocol/openid-connect/logout"
    introspectionUrl: "http://keycloak:8080/realms/fern-platform/protocol/openid-connect/token/introspect"
    introspectionClientId: "fern-platform-introspection"
    introspectionClientSecret: "fern-introspection-secret"
    adminUsers: "admin@fern.com"
```

> 💡 Keep your local `config/config.yaml` overrides out of commits — the cluster password is environment-specific.

### Step 5: Run Fern Platform from your IDE

Run from the repo root:

```bash
go run cmd/fern-platform/main.go
```

Or run/debug `cmd/fern-platform/main.go` from your IDE with the working directory set to the repo root.

Open `http://localhost:8081` and log in with the test credentials (e.g. `admin@fern.com / test123`).

> ℹ️ The cluster instance remains accessible at `http://fern-platform.local:8080`.

---

## 🏢 30-Minute Production Setup

**Perfect for:** Teams ready to deploy to their Kubernetes cluster

> ⚠️ **Production Note**: While Fern Platform is stable and used in production, it's under active development. Test thoroughly in staging, maintain backups, and monitor for updates. [See production guidance](../../README.md#-production-usage).

### Step 1: Customize Configuration (10 minutes)

1. **Copy the deployment template:**
   ```bash
   cp deployments/fern-platform-kubevela.yaml deployments/production.yaml
   ```

2. **Update OAuth settings:**
   ```yaml
   # In deployments/production.yaml
   env:
     - name: OAUTH_CLIENT_ID
       value: "your-production-client-id"
     - name: OAUTH_CLIENT_SECRET
       value: "your-production-client-secret"
     - name: OAUTH_AUTH_URL
       value: "https://auth.yourcompany.com/oauth2/authorize"
     # ... other OAuth endpoints
   ```

3. **Configure admin users:**
   ```yaml
   env:
     - name: OAUTH_ADMIN_USERS
       value: "admin@yourcompany.com,platform-team@yourcompany.com"
     - name: OAUTH_ADMIN_GROUPS
       value: "platform-admins,engineering-leads"
   ```

### Step 2: Deploy to Production (15 minutes)

```bash
# 1. Create namespace
kubectl create namespace fern-platform-prod

# 2. Deploy with production config
kubectl apply -f deployments/production.yaml

# 3. Wait for deployment
kubectl wait --for=condition=Available \
  deployment/fern-platform -n fern-platform-prod \
  --timeout=300s

# 4. Set up ingress/load balancer (depends on your cluster)
kubectl apply -f deployments/ingress-production.yaml
```

### Step 3: Verify Production Setup (5 minutes)

```bash
# 1. Check all components
kubectl get pods -n fern-platform-prod

# 2. Test health endpoint
kubectl port-forward -n fern-platform-prod service/fern-platform 8080:8080 &
curl http://fern-platform.local:8080/health

# 3. Test with your OAuth provider
# Visit your production URL and test login

# For local testing with port-forward:
curl http://fern-platform.local:8080/health
```

**[📖 Complete production deployment guide →](../operations/production-setup.md)**

---

## 🔧 Next Steps After Setup

### 1. Explore the Platform (5 minutes)
- **Dashboard**: Overview of platform status and recent activity
- **Test Summaries**: Grid and treemap views of all projects  
- **Test Runs**: Detailed test execution data
- **Admin Panel** (admin users only): User and project management

### 2. Create a Project (5 minutes)
Have your team manager:
1. Log in with manager privileges
2. Navigate to **Projects** → **Create New Project**
3. Share the project ID with your team

### 3. Install a Client Library (5 minutes)
Choose the client for your test framework:

#### JavaScript/Jest
```bash
npm install --save-dev @guidewire/fern-jest-client
```

#### Java/JUnit
```xml
<dependency>
    <groupId>com.guidewire.fern</groupId>
    <artifactId>fern-junit-client</artifactId>
    <version>1.0.0</version>
</dependency>
```

#### Go/Ginkgo
```bash
go get github.com/guidewire-oss/fern-ginkgo-client
```

### 4. Configure and Run Tests (10 minutes)
Set your environment variables:
```bash
export FERN_URL=http://fern-platform.local:8080
export FERN_PROJECT_ID=your-project-id

# Optional: Only if using JIRA integration
export JIRA_ENCRYPTION_KEY=$(openssl rand -hex 32)
```

Run your tests as usual - results will automatically appear in Fern Platform!

**[📖 Complete integration guide →](./integration-guide.md)**

### 5. Prepare for AI Features (Coming Soon)
The platform is designed with AI integration in mind, but these features are not yet implemented:

**Planned AI capabilities:**
- 🤖 **Flaky test detection** using statistical analysis
- 📊 **Failure pattern recognition** with ML models  
- 💡 **Smart test recommendations** powered by LLMs
- 🔍 **Automated root cause analysis**

**[📋 See our AI roadmap →](../../issues?q=is%3Aissue+is%3Aopen+label%3Aai)**

---

## 🆘 Troubleshooting

### Common Issues

#### "Pod stuck in Pending state"
```bash
# Check node resources
kubectl describe nodes
kubectl top nodes

# Usually means insufficient CPU/memory
# Solution: Free up resources or use smaller resource requests
```

#### "DNS resolution failed"
```bash
# Check /etc/hosts entries
cat /etc/hosts | grep fern-platform

# Should show:
# 127.0.0.1 fern-platform.local
# 127.0.0.1 keycloak

# Add if missing:
echo "127.0.0.1 fern-platform.local" | sudo tee -a /etc/hosts
echo "127.0.0.1 keycloak" | sudo tee -a /etc/hosts
```

#### "OAuth redirect error"
```bash
# Make sure you're accessing via the correct URL
# ✅ Good: http://fern-platform.local:8080
# ❌ Bad:  http://localhost:8080

# Check Keycloak client configuration matches
kubectl logs -n fern-platform deployment/keycloak | grep -i redirect
```

### Get Help
- 🐛 **Found a bug?** [Open an issue](../../issues)
- ❓ **Have questions?** [Start a discussion](../../discussions)  
- 📖 **Need more details?** [Browse full documentation](../README.md)
- 🔧 **Production issues?** [Troubleshooting guide](../operations/troubleshooting.md)

---

## ✅ Quick Start Checklist

Copy this checklist to track your progress:

```markdown
## Fern Platform Quick Start

### Setup
- [ ] Prerequisites installed (k3d, kubectl, helm, vela)
- [ ] /etc/hosts entries added (fern-platform.local, keycloak)
- [ ] Repository cloned
- [ ] `make deploy-all` completed successfully
- [ ] All pods showing "Running" status

### Testing  
- [ ] Health endpoint responds: `curl http://fern-platform.local:8080/health`
- [ ] Web UI accessible: http://fern-platform.local:8080
- [ ] Admin login works: admin@fern.com / test123
- [ ] User login works: fern-user@fern.com / test123
- [ ] Can view dashboard, test summaries, and test runs

### Integration
- [ ] Submitted first test data via API
- [ ] Test run appears in dashboard
- [ ] Configured CI/CD integration (optional)
- [ ] Set up JIRA integration (optional, see [JIRA Integration Guide](../configuration/jira-integration.md))
- [ ] Added AI API keys (optional)

### Next Steps
- [ ] Read [Architecture Overview](../architecture/overview.md)
- [ ] Explore [API Documentation](api-reference.md)
- [ ] Join [Community Discussions](../../discussions)
```

---

**🎉 Congratulations!** You now have a fully functional Fern Platform. Ready to dive deeper? Check out our [Developer Guide](local-development.md) or [Architecture Overview](../architecture/overview.md).