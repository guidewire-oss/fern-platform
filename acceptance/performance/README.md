# GraphQL Query Performance Tests

## Purpose

These tests prevent regression of the **486MB monolithic query problem** that was fixed in this branch.

See `PERFORMANCE_FIX.md` for full details on the original issue.

## Authentication

**Default**: Auth is **disabled** (`AUTH_ENABLED=false`)

The tests work with or without authentication:
- **Auth disabled** (default): Tests run directly, no login needed
- **Auth enabled**: Tests auto-login with credentials, save cookies

This matches the open source deployment where auth is optional.

### Running Without Auth (Default)

```bash
# Deploy without auth (default for open source)
make deploy-all  # Sets AUTH_ENABLED=false

# Run tests
make test-performance
```

### Running With Auth (Your Local Setup)

```bash
# If your config.yaml has auth.enabled: true
export AUTH_ENABLED=true
export FERN_USERNAME=your-user@example.com
export FERN_PASSWORD=your-password

make test-performance
```

## What This Tests

1. **Response Size Limits** - Ensures GraphQL queries don't return massive responses
2. **Query Optimization** - Validates that pages only load the data they need
3. **Regression Prevention** - Catches if someone accidentally reintroduces the monolithic query pattern

## Test Data Requirements

**Critical**: These tests require ~100 test runs to expose the performance issue.

### For Local Development:

```bash
# Install PostgreSQL client if needed
brew install libpq
export PATH="/opt/homebrew/opt/libpq/bin:$PATH"

# Seed data (adjust connection details from your config/config.yaml)
psql -h localhost -p 5432 -U fern -d fern -f scripts/test-data.sql
```

### For Kubernetes:

```bash
./scripts/insert-test-data.sh
```

This creates:
- 3 projects
- ~100-120 test runs (30-40 per project)
- ~400-500 suite runs  
- ~3000-4000 spec runs

With this volume of data:
- **Old monolithic query** would return **~7-10MB** per page (or 486MB in production with more data)
- **New optimized queries** return **~100-200KB** per page

## Running the Tests

### Option 1: Local Development (Recommended for Testing Changes)

If you're running Fern Platform locally (not in Kubernetes):

```bash
# 1. Make sure PostgreSQL client is installed
brew install libpq
export PATH="/opt/homebrew/opt/libpq/bin:$PATH"

# 2. Seed test data to your local database
psql -h localhost -p 5432 -U fern -d fern -f scripts/test-data.sql
# Password: fern (or whatever is in your config/config.yaml)

# 3. Verify data was created (~100+ test runs)
psql -h localhost -p 5432 -U fern -d fern -c "SELECT COUNT(*) FROM test_runs;"

# 4. Start your local Fern Platform
go run cmd/server/main.go  # or however you start it

# 5. Run performance tests
export FERN_BASE_URL=http://localhost:8080
cd acceptance
make test-performance
```

### Option 2: Kubernetes Deployment (CI/CD)

If you have Fern Platform deployed to k3d/Kubernetes:

```bash
# 1. Deploy the platform
make deploy-all

# 2. Seed test data
./scripts/insert-test-data.sh

# 3. Run performance tests
make test-performance
```

### Option 3: Custom Environment

```bash
# Point to any running instance
export FERN_BASE_URL=https://your-fern-instance.com
export AUTH_ENABLED=true  # if auth is required
export FERN_USERNAME=your-user@example.com
export FERN_PASSWORD=your-password

cd acceptance
make test-performance
```

## Test Assertions

### Projects Page
- ✅ Total GraphQL responses < 50KB
- ❌ FAIL if > 50KB (indicates GET_PROJECTS_LIST is loading too much)

### Dashboard  
- ✅ Total GraphQL responses < 200KB
- ❌ FAIL if > 200KB (indicates monolithic query is being used)

### Test Runs Page
- ✅ Total GraphQL responses < 200KB (initial load)
- ✅ Should NOT preload nested suite/spec data
- ❌ FAIL if > 500KB (indicates preloading nested data)

### All Pages
- ✅ No single page should ever load > 10MB
- ❌ FAIL if any page loads > 10MB (regression to monolithic pattern)

## How the Tests Work

These tests use **Playwright** (a browser automation tool) to load pages and measure GraphQL response sizes:

1. **Browser Setup**: A headless Chromium browser is launched once for all tests (`browser` variable)
2. **Response Monitoring**: Each test attaches a listener to intercept all HTTP responses (via `page.On("response", ...)`)
3. **GraphQL Tracking**: When a response URL contains `/query`, the test records its `content-length` header and URL
4. **Critical Timing**: After navigating to a page, tests wait in two phases:
   - `WaitForLoadState(NetworkIdle)` - Waits for static assets (HTML, CSS, JS bundles) to finish loading
   - `WaitForTimeout(2000)` - **Critical 2-second wait** for React to initialize and make GraphQL calls
   
### Why the 2-second timeout is necessary

This application is a **Single Page Application (SPA)** with React. The loading sequence is:

```
1. Browser loads HTML → NetworkIdle achieved
2. JavaScript bundles download → NetworkIdle achieved again  
3. React parses, initializes, and renders components (takes time)
4. React components make GraphQL queries (this is what we're measuring)
5. GraphQL responses arrive
```

The `NetworkIdle` state occurs at step 2, but GraphQL calls don't happen until steps 4-5. Without the 2-second wait, tests would check `totalBytes` and `responseURLs` **before any GraphQL responses arrive**, causing assertions to fail (expecting at least 1 response, getting 0).

The 2-second timeout bridges the gap between "static assets loaded" and "React made its API calls."

## What Gets Measured

The tests intercept all GraphQL responses and track:
- `content-length` header from each `/query` response
- Total bytes transferred per page load (`totalBytes` variable)
- URLs of all GraphQL responses (`responseURLs` array)
- Large responses are logged for debugging (> 1MB)

## Expected Results

With the **optimized queries**:
```
Projects page - Total GraphQL data: ~20 KB (1 response)
Dashboard - Total GraphQL data: ~120 KB (3 responses)  
Test Runs page - Total GraphQL data: ~100 KB (1-2 responses)
```

With the **old monolithic query** (if regressed):
```
❌ Any page - Total GraphQL data: 7-10 MB (or 486MB in production)
```

## Debugging Failed Tests

If a test fails:

1. **Check test data**:
   ```bash
   # Verify test runs exist
   kubectl exec -it postgres-pod -n fern-platform -- \
     psql -U postgres -d fern_platform -c "SELECT COUNT(*) FROM test_runs;"
   # Should show ~100+
   ```

2. **Run with visible browser**:
   ```bash
   FERN_HEADLESS=false ginkgo -v ./performance
   # Open DevTools → Network tab to inspect GraphQL responses
   ```

3. **Check which query is being used**:
   ```bash
   # In browser DevTools, inspect the GraphQL query payload
   # Look for GET_PROJECTS_LIST vs GET_DASHBOARD_DATA
   ```

4. **Verify query definitions**:
   ```bash
   # Check that optimized queries exist
   grep "GET_PROJECTS_LIST\|GET_DASHBOARD_SUMMARY" web/js/graphql-client.js
   ```

## Adding to CI/CD

Add to your GitHub Actions workflow:

```yaml
- name: Run Performance Tests
  env:
    FERN_BASE_URL: ${{ secrets.FERN_BASE_URL }}
  run: |
    cd acceptance
    ./scripts/insert-test-data.sh
    ginkgo -v ./performance
```

## Related Documentation

- `PERFORMANCE_FIX.md` - Detailed explanation of the original issue and fix
- `PERFORMANCE_TEST.md` - Manual testing procedures
- `web/js/graphql-client.js` - Query definitions
- `scripts/test-data.sql` - Test data generation script
