# JIRA Integration Configuration

Fern Platform supports integration with JIRA to enable:
- Project management connector workflows
- Test tagging with JIRA issue IDs
- Field mapping between JIRA and Fern Platform
- Requirements traceability

## Is JIRA Integration Optional?

**Yes.** JIRA integration is completely optional. The Fern Platform works perfectly without it. Only configure JIRA integration if your team uses JIRA and wants to link test results with JIRA issues.

## Setup

### Prerequisites

JIRA integration requires an encryption key to securely store JIRA credentials.

### 1. Generate Encryption Key

Generate a new 64-character hex string (32 bytes):

```bash
openssl rand -hex 32
```

Example output:
```
a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6
```

### 2. Set Environment Variable

Configure the encryption key for your deployment:

#### Local Development

```bash
export JIRA_ENCRYPTION_KEY=$(openssl rand -hex 32)
go run cmd/fern-platform/main.go
```

#### Docker Compose

Update `docker-compose.yml`:

```yaml
services:
  fern-platform:
    environment:
      JIRA_ENCRYPTION_KEY: "your-64-char-hex-string"
```

Run:
```bash
docker-compose up -d
```

#### Kubernetes / KubeVela

Update `deployments/fern-platform-kubevela.yaml`:

```yaml
spec:
  components:
    - type: webservice
      properties:
        env:
          - name: JIRA_ENCRYPTION_KEY
            value: "your-64-char-hex-string"
```

Then deploy:
```bash
kubectl apply -f deployments/fern-platform-kubevela.yaml
```

#### Environment File (.env)

Create a `.env` file (not committed to git):

```bash
JIRA_ENCRYPTION_KEY=your-64-char-hex-string
```

Load before running:
```bash
source .env
go run cmd/fern-platform/main.go
```

### 3. Start the Application

Once `JIRA_ENCRYPTION_KEY` is set, start Fern Platform:

```bash
go run cmd/fern-platform/main.go
```

Or restart your container/pod:
```bash
docker-compose restart fern-platform
kubectl rollout restart deployment/fern-platform -n fern-platform
```

## What Happens If JIRA_ENCRYPTION_KEY Is Not Set?

If you do not set `JIRA_ENCRYPTION_KEY`:
- The application starts normally
- JIRA integration features are disabled
- The Fern Platform UI does not show JIRA connection options
- Test results continue to be aggregated normally
- You can enable JIRA integration later by setting the variable and restarting

## What Happens If JIRA_ENCRYPTION_KEY Is Invalid?

The application will fail at startup with a clear error message:

```
panic: JIRA_ENCRYPTION_KEY is not valid hex: encoding/hex: invalid byte
```

or

```
panic: JIRA_ENCRYPTION_KEY must decode to exactly 32 bytes, got 24
```

**Fix:** Generate a new 64-character hex string with `openssl rand -hex 32` and update the configuration.

## Key Rotation

To rotate the encryption key:

1. **Generate a new key:**
   ```bash
   openssl rand -hex 32
   ```

2. **Update all JIRA connections** to re-encrypt credentials with the new key
   - This happens automatically when the application restarts
   - Existing credentials are decrypted with the old key and re-encrypted with the new key

3. **Update the environment variable** in your deployment configuration

4. **Restart the application**

## Security Best Practices

1. **Use a strong, random key** — Always generate with `openssl rand -hex 32`, never use hardcoded values
2. **Secure storage** — Store the key in:
   - Kubernetes Secrets (not ConfigMaps)
   - AWS Secrets Manager, HashiCorp Vault, or similar
   - Environment-specific `.env` files (add to `.gitignore`)
   - Never commit to version control
3. **Access control** — Limit who has access to the key
4. **Rotation** — Rotate keys periodically (every 90 days recommended)
5. **Audit** — Log access to JIRA credentials and JIRA operations

## Troubleshooting

### Application won't start: "JIRA_ENCRYPTION_KEY not valid"

**Problem:** The encryption key is not a valid 64-character hex string

**Solution:**
```bash
# Generate a new key
JIRA_KEY=$(openssl rand -hex 32)
echo $JIRA_KEY  # Verify it's 64 characters

# Set it in your environment or deployment
export JIRA_ENCRYPTION_KEY=$JIRA_KEY
```

### JIRA buttons don't appear in the UI

**Problem:** JIRA integration features are not visible

**Solution:** The application started without `JIRA_ENCRYPTION_KEY` set
```bash
# Set the environment variable
export JIRA_ENCRYPTION_KEY=$(openssl rand -hex 32)

# Restart the application
```

### "JIRA integration is not configured" error when creating a connection

**Problem:** The application was restarted without the encryption key

**Solution:** Ensure `JIRA_ENCRYPTION_KEY` is set in your environment before starting the application:
```bash
# Check if the variable is set
echo $JIRA_ENCRYPTION_KEY

# If empty, set it:
export JIRA_ENCRYPTION_KEY=your-64-char-hex-string

# Restart the application
```

## See Also

- [JIRA Integration Gap Analysis](../analysis/jira-integration-gap-analysis.md)
- [PM Connectors Management](../use-cases/10-pm-connectors-management.md)
- [Requirements Traceability RFC](../rfc/rfc-003-requirements-traceability-and-test-coverage-intelligence.md)
