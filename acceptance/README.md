# Fern Platform Acceptance Tests

End-to-end acceptance tests for Fern Platform using Ginkgo, Playwright-go, and Gomega.

## Structure

The acceptance tests are organized by use case:

- `auth/` - Authentication tests (UC-00)
- `testsummaries/` - Test Summaries and Visualization tests (UC-02)
- `testruns/` - Test Runs and Drill-Down tests (UC-03)
- `helpers/` - Shared helper functions for authentication, navigation, etc.

Each package has its own test suite and can be run independently.

## Prerequisites

1. Go 1.22 or higher
2. Ginkgo CLI: `go install github.com/onsi/ginkgo/v2/ginkgo@latest`
3. Playwright browsers: `go run github.com/playwright-community/playwright-go/cmd/playwright@v0.4802.0 install --with-deps chromium`
4. Fern Platform running at the configured URL
5. Test data loaded via `scripts/insert-test-data.sh`
6. Valid test user credentials

## Running Tests

### Run all tests
```bash
make test
```

### Run specific test suites
```bash
make test-auth         # Authentication tests only
make test-summaries    # Test summaries tests only
make test-runs         # Test runs tests only
```

### Run with visible browser (debugging)
```bash
make test-headed       # Run with visible browser
make test-slow         # Run with 500ms slow motion
make test-record       # Run with video recording enabled
```

### Run with video recording
```bash
# Record all tests
FERN_RECORD_VIDEO=true make test

# Record specific test suite
FERN_RECORD_VIDEO=true make test-auth

# Videos are saved in each test directory's videos/ folder
# e.g., auth/videos/, testsummaries/videos/, testruns/videos/
```

### Run specific test by pattern
```bash
make test-focus FOCUS="should show only projects from user's team"
```

## Configuration

Tests can be configured via environment variables or command-line flags:

| Variable | Flag | Default | Description |
|----------|------|---------|-------------|
| `FERN_BASE_URL` | `-base-url` | `http://fern-platform.local:8080` | Base URL for Fern Platform |
| `FERN_USERNAME` | `-username` | `fern-user@fern.com` | Username for authentication |
| `FERN_PASSWORD` | `-password` | `test123` | Password for authentication |
| `FERN_TEAM_NAME` | `-team-name` | `fern` | Team name for test user |
| `FERN_HEADLESS` | `-headless` | `true` | Run browser in headless mode |
| `FERN_RECORD_VIDEO` | `-record-video` | `false` | Record videos of test runs |

### Example with custom configuration
```bash
# Test as admin (sees all teams)
FERN_BASE_URL=http://localhost:8080 \
FERN_USERNAME=admin@fern.com \
FERN_PASSWORD=admin123 \
make test

# Test as manager (sees fern team with management capabilities)
FERN_USERNAME=fern-manager@fern.com \
FERN_PASSWORD=test123 \
make test

# Test as user from different team (should not see fern team data)
FERN_USERNAME=atmos-user@fern.com \
FERN_PASSWORD=test123 \
FERN_TEAM_NAME=atmos \
make test
```

### Available Test Users

| Username | Password | Role | Team | Description |
|----------|----------|------|------|-------------|
| `admin@fern.com` | `admin123` | Admin | All | Platform admin with access to all teams |
| `fern-manager@fern.com` | `test123` | Manager | fern | Manager of the fern team |
| `fern-user@fern.com` | `test123` | User | fern | Regular user in the fern team |
| `atmos-user@fern.com` | `test123` | User | atmos | User in different team (no fern access) |

## Writing New Tests

1. Create a new package for your use case
2. Create a suite file (`*_suite_test.go`) with setup/teardown
3. Create test files (`*_test.go`) with Ginkgo specs
4. Use the helpers package for common operations

### Example Test Structure
```go
var _ = Describe("UC-XX: Feature Name", func() {
    var (
        ctx  playwright.BrowserContext
        page playwright.Page
    )

    BeforeEach(func() {
        ctx, page = createAuthenticatedContext()
        // Navigate to page under test
    })

    AfterEach(func() {
        if ctx != nil {
            ctx.Close()
        }
    })

    Describe("Scenario Name", func() {
        Context("Given some condition", func() {
            It("should do something", func() {
                // Test implementation
            })
        })
    })
})
```

## Test Data Requirements

The tests assume:
1. Test data has been loaded using `scripts/insert-test-data.sh`
2. The test user belongs to the configured team
3. The team has projects with test runs

## Debugging Failed Tests

1. Run with visible browser: `make test-headed`
2. Add slow motion: `make test-slow`
3. Use Playwright's debugging features:
   - `page.Screenshot()` to capture screenshots
   - `page.Pause()` to pause execution
   - Browser DevTools for inspection

## CI/CD Integration

```yaml
# Example GitHub Actions workflow
- name: Run E2E Tests
  env:
    FERN_BASE_URL: ${{ secrets.FERN_BASE_URL }}
    FERN_USERNAME: ${{ secrets.FERN_USERNAME }}
    FERN_PASSWORD: ${{ secrets.FERN_PASSWORD }}
  run: |
    cd acceptance
    make test
```

## Common Issues

### Browser not installed
```bash
make deps  # This will install Chromium
```

### Authentication failures
- Verify Keycloak is running
- Check credentials are correct
- Ensure user belongs to the configured team

### Element not found
- Check selectors match actual HTML
- Add appropriate waits for dynamic content
- Use multiple selector strategies

### Browser crashes (TargetClosedError)
The test suite automatically applies platform-specific browser configurations:

- **macOS**: Uses `--single-process` flag to resolve TLS certificate issues
- **Docker/CI**: Uses additional stability flags for containerized environments
- **Custom flags**: Set `PLAYWRIGHT_CHROMIUM_ARGS` environment variable

For Docker environments, ensure container is run with:
```bash
docker run --ipc=host --cap-add=SYS_ADMIN your-test-image
```

To debug browser launch issues:
```bash
DEBUG=1 make test  # Shows browser launch arguments
```

To override browser arguments:
```bash
export PLAYWRIGHT_CHROMIUM_ARGS="--disable-gpu --disable-software-rasterizer"
make test
```

## Contributing

When adding new tests:
1. Follow the existing structure and naming conventions
2. Use descriptive test names that match use case documentation
3. Add appropriate error handling and timeouts
4. Update this README if adding new configuration options