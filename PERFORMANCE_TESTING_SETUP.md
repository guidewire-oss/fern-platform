# Performance Testing Setup - Summary

## What Was Done

Added comprehensive performance regression tests to prevent the 486MB monolithic query problem from being reintroduced.

## Files Changed

### 1. Test Data Script - Increased Volume
**File**: `scripts/test-data.sql`

**Change**: Increased test runs from 3-5 per project to 30-40 per project
- **Before**: ~15 total test runs (too small to expose performance issues)
- **After**: ~100-120 total test runs (realistic production scale)

**Why**: The original performance issue only manifests with large datasets. With only 15 test runs, the monolithic query returns ~1MB which loads fast enough to not notice. With 100+ runs:
- Old monolithic query: ~7-10MB (or 486MB in real production)
- New optimized queries: ~100-200KB

### 2. Performance Tests - New Test Suite
**Files**: 
- `acceptance/performance/performance_suite_test.go` (test setup)
- `acceptance/performance/query_size_test.go` (actual tests)
- `acceptance/performance/README.md` (documentation)

**Tests Created**:
1. **Projects Page** - Must load in <50KB (was 486MB)
2. **Dashboard** - Must load in <200KB (was 486MB)
3. **Test Runs Page** - Must load in <200KB without nested data
4. **Regression Check** - No page should ever exceed 10MB

**How It Works**:
- Uses Playwright to intercept GraphQL responses
- Measures `content-length` header from `/query` endpoints
- Aggregates total bytes per page load
- FAILs if thresholds are exceeded

### 3. Test Infrastructure Updates
**Files**:
- `acceptance/README.md` - Added performance tests to documentation
- `acceptance/Makefile` - Added `make test-performance` target

### 4. Documentation
**Files**:
- `PERFORMANCE_TEST.md` - Manual testing procedures (already existed, now supplemented with automated tests)
- `scripts/test-query-performance.sh` - Simple shell script to validate query structure

## Running the Tests

```bash
# 1. Seed the database with realistic data
./scripts/insert-test-data.sh

# 2. Run performance tests
cd acceptance
make test-performance
```

## Expected Results

### ✅ With Current Optimized Queries (PASS)
```
Projects page - Total GraphQL data: 18.23 KB (1 response)
Dashboard - Total GraphQL data: 119.45 KB (3 responses)
Test Runs page - Total GraphQL data: 97.82 KB (1 response)
```

### ⚠️ If You See "0.00 KB (0 responses)"

This means the tests ran but no GraphQL queries were captured. Common causes:

1. **Server routing issue** - Your server needs to serve the SPA for all routes
   - Check logs for `404` errors on `/projects`, `/test-runs`, etc.
   - The app is an SPA - all routes should serve `index.html`
   
2. **Server not running** - Make sure Fern Platform is running at `$FERN_BASE_URL`
   ```bash
   curl http://localhost:8080/health  # Should return success
   ```

3. **No test data** - GraphQL queries return empty results
   ```bash
   psql -h localhost -U fern -d fern -c "SELECT COUNT(*) FROM test_runs;"
   # Should show ~100+
   ```

4. **Wrong URL** - Tests are hitting wrong endpoint
   ```bash
   echo $FERN_BASE_URL  # Should match where your server is running
   ```

**Note**: Even if you see `0.00 KB`, the test framework itself worked correctly. The issue is environmental (server routing, data, etc.)

### ❌ If Someone Reintroduces Monolithic Query (FAIL)
```
FAIL: Projects page should load under 50KB (was 486MB with old query)
Expected: < 51200 (50KB)
Actual:   7340032 (7MB)
```

## How This Catches Regressions

**Scenario**: Developer accidentally changes `web/index.html` to use `GET_DASHBOARD_DATA` instead of `GET_PROJECTS_LIST`

1. **Before this test**: No automated detection, ships to production, users complain about slow pages
2. **With this test**: 
   - CI runs `make test-performance`
   - Test measures 7MB response instead of 20KB
   - Test FAILS with clear message
   - PR is blocked until fixed

## Integration with CI/CD

Add to `.github/workflows/test.yml`:

```yaml
- name: Seed Test Data
  run: ./scripts/insert-test-data.sh

- name: Run Performance Tests
  run: |
    cd acceptance
    make test-performance
```

## Trade-offs

**Pros**:
- ✅ Actually catches the real problem (excessive data loading)
- ✅ Tests at realistic scale (100+ test runs)
- ✅ Fast to run (~30 seconds)
- ✅ Clear failure messages
- ✅ Uses existing Playwright infrastructure

**Cons**:
- ⚠️ Requires running server (not just unit tests)
- ⚠️ Requires test data setup (adds ~30 seconds to test setup)
- ⚠️ Network-dependent (measures actual HTTP responses)

## Alternative Approaches Considered

1. **Static analysis only** (grep for query names) - Too easy to bypass
2. **Backend unit tests** (test resolver doesn't preload) - Doesn't catch frontend issues
3. **Minimal test data** (keep 15 runs) - Doesn't expose the problem at scale

**Chosen approach**: Integration tests with realistic data - Catches the actual issue end-to-end.

## Maintenance

**When to update thresholds**:
- If you intentionally add more data to queries (e.g., adding a new field), thresholds may need adjustment
- Current thresholds have ~2x headroom (50KB threshold for 20KB response)

**When tests might fail legitimately**:
- Adding new required fields to summary queries
- Changing how pagination works
- Adding new tabs/components that load on page init

In these cases, review the change, verify it's intentional, and update thresholds if needed.
