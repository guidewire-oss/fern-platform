# Troubleshooting Guide

**Common issues and solutions for Fern Platform**

## Quick Diagnostics

Before diving into specific issues, run these checks:

```bash
# 1. Check if all pods are running
kubectl get pods -n fern-platform

# 2. Check service health
curl http://fern-platform.local:8080/health

# 3. View recent logs
kubectl logs -n fern-platform deployment/fern-platform --tail=50

# 4. Check ingress status
kubectl get ingress -n fern-platform
```

## Common Issues

### 🔴 Platform Won't Start

#### Symptoms
- Pods stuck in `Pending` or `CrashLoopBackOff` state
- Health endpoint not responding

#### Solutions

**1. Check Resource Availability**
```bash
# Check node resources
kubectl describe nodes
kubectl top nodes

# Common issue: Insufficient CPU/memory
# Solution: Free up resources or reduce resource requests
```

**2. Database Connection Issues**
```bash
# Check PostgreSQL pod
kubectl logs -n fern-platform deployment/postgres

# Common issues:
# - PostgreSQL not ready
# - Wrong connection string
# - Database not initialized

# Fix: Wait for PostgreSQL to be ready
kubectl wait --for=condition=ready pod -l app=postgres -n fern-platform --timeout=300s
```

**3. Missing Configuration**
```bash
# Check ConfigMap
kubectl describe configmap fern-platform-config -n fern-platform

# Ensure required environment variables are set:
# - DB_HOST, DB_PORT, DB_NAME, DB_USER, DB_PASSWORD
# - OAUTH_* variables if OAuth is enabled
```

### 🔴 OAuth Login Failures

#### Symptoms
- "Redirect URI mismatch" error
- Can't log in with test users
- Infinite redirect loops

#### Solutions

**1. Check /etc/hosts Entries**
```bash
# Must have these entries:
cat /etc/hosts | grep -E "fern-platform|keycloak"

# Should show:
# 127.0.0.1 fern-platform.local
# 127.0.0.1 keycloak

# If missing, add them:
echo "127.0.0.1 fern-platform.local" | sudo tee -a /etc/hosts
echo "127.0.0.1 keycloak" | sudo tee -a /etc/hosts
```

**2. Use Correct URL**
- ✅ Access via: `http://fern-platform.local:8080`
- ❌ NOT via: `http://localhost:8080`

**3. Check Keycloak Configuration**
```bash
# Port-forward to Keycloak admin
kubectl port-forward -n fern-platform svc/keycloak 8090:8080

# Access http://localhost:8090
# Login: admin/admin
# Check client configuration matches redirect URIs
```

**4. Clear Browser Data**
- Clear cookies for fern-platform.local and keycloak
- Try incognito/private browsing mode

### 🔴 Test Data Not Appearing

#### Symptoms
- API returns 200 but data doesn't show in UI
- GraphQL queries return empty results

#### Solutions

**1. Check Project ID**
```bash
# List all projects
curl http://fern-platform.local:8080/api/v1/projects

# Ensure you're using correct project ID in submissions
```

**2. Verify Data Submission**
```bash
# Check if test run was created
curl http://fern-platform.local:8080/api/v1/test-runs?projectId=YOUR_PROJECT_ID

# Common issues:
# - Wrong project ID
# - Malformed JSON
# - Missing required fields
```

**3. Check Database**
```bash
# Get PostgreSQL pod name
POSTGRES_POD=$(kubectl get pods -n fern-platform -l app=postgres -o jsonpath='{.items[0].metadata.name}')

# Connect to PostgreSQL
kubectl exec -it -n fern-platform $POSTGRES_POD -- psql -U postgres fern_platform

# Check data
SELECT COUNT(*) FROM test_runs;
SELECT * FROM test_runs ORDER BY created_at DESC LIMIT 5;
```

### 🔴 Performance Issues

#### Symptoms
- Slow page loads
- Timeouts on large datasets
- High CPU/memory usage

#### Solutions

**1. Check Resource Usage**
```bash
# Pod resource usage
kubectl top pods -n fern-platform

# If high usage, scale resources:
kubectl edit deployment fern-platform -n fern-platform
# Increase resources.requests and resources.limits
```

**2. Database Performance**
```bash
# Get PostgreSQL pod name
POSTGRES_POD=$(kubectl get pods -n fern-platform -l app=postgres -o jsonpath='{.items[0].metadata.name}')

# Check slow queries
kubectl exec -it -n fern-platform $POSTGRES_POD -- psql -U postgres fern_platform -c "SELECT * FROM pg_stat_statements ORDER BY total_time DESC LIMIT 10;"

# Add indexes if needed (check migrations/)
```

**3. Redis Cache Issues**
```bash
# Check Redis
kubectl logs -n fern-platform deployment/redis

# Get Redis pod name
REDIS_POD=$(kubectl get pods -n fern-platform -l app=redis -o jsonpath='{.items[0].metadata.name}')

# Clear cache if corrupted
kubectl exec -it -n fern-platform $REDIS_POD -- redis-cli FLUSHALL
```

### 🔴 API Errors

#### Common HTTP Status Codes

**400 Bad Request**
- Check request payload format
- Ensure all required fields are present
- Validate data types (duration in milliseconds, not seconds)

**401 Unauthorized**
- Check authentication token
- Ensure OAuth is properly configured
- Try getting a new token

**404 Not Found**
- Verify endpoint URL
- Check if resource (project, test run) exists
- Ensure correct ID format

**500 Internal Server Error**
```bash
# Check application logs
kubectl logs -n fern-platform deployment/fern-platform --tail=100 | grep ERROR

# Common causes:
# - Database connection lost
# - Nil pointer exceptions
# - Configuration errors
```

## Debugging Tools

### Enable Debug Logging

```yaml
# Edit deployment
kubectl edit deployment fern-platform -n fern-platform

# Add environment variable:
env:
  - name: LOG_LEVEL
    value: "debug"
```

### Database Queries

```sql
-- Check recent test runs
SELECT id, project_id, status, created_at 
FROM test_runs 
ORDER BY created_at DESC 
LIMIT 10;

-- Check flaky tests
SELECT * FROM flaky_tests 
WHERE status = 'active';

-- Check user sessions
SELECT * FROM sessions 
WHERE expires_at > NOW();
```

### Health Check Details

```bash
# Detailed health check
curl http://fern-platform.local:8080/health/detailed

# Should return:
{
  "status": "healthy",
  "checks": {
    "database": "ok",
    "redis": "ok",
    "migrations": "ok"
  }
}
```

## Reset Procedures

### Reset Database (Development Only)

```bash
# Scale down application
kubectl scale deployment fern-platform -n fern-platform --replicas=0

# Get PostgreSQL pod name
POSTGRES_POD=$(kubectl get pods -n fern-platform -l app=postgres -o jsonpath='{.items[0].metadata.name}')

# Connect to PostgreSQL
kubectl exec -it -n fern-platform $POSTGRES_POD -- psql -U postgres

# Drop and recreate database
DROP DATABASE fern_platform;
CREATE DATABASE fern_platform;
\q

# Scale up application (migrations run automatically)
kubectl scale deployment fern-platform -n fern-platform --replicas=1
```

### Clear All Test Data

```bash
# Via API (keeps projects and users)
curl -X DELETE http://fern-platform.local:8080/api/v1/test-runs/all \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

### Restart Everything

```bash
# Quick restart
kubectl rollout restart deployment -n fern-platform

# Full restart
kubectl delete pods --all -n fern-platform
# Pods will be recreated automatically
```

## Getting Help

### Collect Diagnostic Information

```bash
# Generate diagnostic bundle
kubectl cluster-info dump --namespace fern-platform --output-directory=/tmp/fern-diagnose

# Include:
# - Pod descriptions
# - Recent logs
# - ConfigMaps
# - Service definitions
```

### Where to Get Help

1. **Check existing issues**: [GitHub Issues](https://github.com/guidewire-oss/fern-platform/issues)
2. **Ask questions**: [GitHub Discussions](https://github.com/guidewire-oss/fern-platform/discussions)
3. **Report bugs**: Create a new issue with diagnostic information

### Information to Include

When reporting issues, include:

1. **Environment**
   - Kubernetes version: `kubectl version`
   - Fern Platform version/commit
   - Deployment method (k3d, EKS, GKE, etc.)

2. **Steps to Reproduce**
   - Exact commands run
   - Expected vs actual behavior

3. **Logs and Errors**
   - Application logs
   - Pod descriptions
   - Browser console errors (for UI issues)

## Known Issues

### k3d Specific

- **Issue**: k3d cluster loses data after restart
- **Solution**: Use persistent volumes or external database

### macOS Specific

- **Issue**: DNS resolution for .local domains
- **Solution**: Use /etc/hosts entries instead of relying on mDNS

### Windows Specific

- **Issue**: Line ending issues in scripts
- **Solution**: Configure Git to use LF endings: `git config core.autocrlf false`

## Related Documentation

- [Configuration Guide](../configuration/) - Detailed configuration options
- [Quick Start](../developers/quick-start.md) - Initial setup guide
- [Architecture](../ARCHITECTURE.md) - System design for deeper debugging