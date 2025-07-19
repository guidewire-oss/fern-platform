# Acceptance Tests Makefile

# Default values
FERN_BASE_URL ?= http://fern-platform.local:8080
FERN_USERNAME ?= fern-user@fern.com
FERN_PASSWORD ?= test123
FERN_TEAM_NAME ?= fern
FERN_HEADLESS ?= true
FERN_RECORD_VIDEO ?= false

# Export environment variables
export FERN_BASE_URL
export FERN_USERNAME
export FERN_PASSWORD
export FERN_TEAM_NAME
export FERN_HEADLESS
export FERN_RECORD_VIDEO

# Install dependencies
.PHONY: deps
deps:
	go mod download
	go run github.com/playwright-community/playwright-go/cmd/playwright install chromium

# Run all tests
.PHONY: test
test: deps
	ginkgo -r -v

# Run authentication tests only
.PHONY: test-auth
test-auth: deps
	ginkgo -v ./auth

# Run test summaries tests only
.PHONY: test-summaries
test-summaries: deps
	ginkgo -v ./testsummaries

# Run test runs tests only
.PHONY: test-runs
test-runs: deps
	ginkgo -v ./testruns

# Run projects tests only
.PHONY: test-projects
test-projects: deps
	ginkgo -v ./projects

# Run tests in watch mode
.PHONY: watch
watch: deps
	ginkgo watch -r -v

# Run tests with specific configuration
.PHONY: test-with-config
test-with-config: deps
	ginkgo -r -v \
		-- \
		-base-url=$(FERN_BASE_URL) \
		-username=$(FERN_USERNAME) \
		-password=$(FERN_PASSWORD) \
		-team-name=$(FERN_TEAM_NAME) \
		-headless=$(FERN_HEADLESS)

# Run tests in headed mode (visible browser)
.PHONY: test-headed
test-headed: deps
	FERN_HEADLESS=false ginkgo -r -v

# Run tests with slow motion for debugging
.PHONY: test-slow
test-slow: deps
	FERN_HEADLESS=false ginkgo -r -v -- -slow-mo=500

# Run tests with video recording
.PHONY: test-record
test-record: deps
	@mkdir -p auth/videos testsummaries/videos testruns/videos
	FERN_RECORD_VIDEO=true ginkgo -r -v

# Clean test artifacts
.PHONY: clean
clean:
	rm -rf screenshots/
	rm -rf videos/
	rm -rf test-results/

# Generate test report
.PHONY: report
report:
	ginkgo -r -v --json-report=test-results.json --output-dir=.

# Run specific test by name pattern
.PHONY: test-focus
test-focus: deps
	ginkgo -r -v --focus="$(FOCUS)"

# Example: make test-focus FOCUS="should show only projects from user's team"

# Help
.PHONY: help
help:
	@echo "Fern Platform Acceptance Tests"
	@echo ""
	@echo "Usage:"
	@echo "  make test              - Run all acceptance tests"
	@echo "  make test-auth         - Run authentication tests only"
	@echo "  make test-summaries    - Run test summaries tests only"
	@echo "  make test-runs         - Run test runs tests only"
	@echo "  make test-headed       - Run tests with visible browser"
	@echo "  make test-slow         - Run tests with slow motion (debugging)"
	@echo "  make watch            - Run tests in watch mode"
	@echo "  make clean            - Clean test artifacts"
	@echo "  make report           - Generate test report"
	@echo "  make test-focus FOCUS=<pattern> - Run tests matching pattern"
	@echo ""
	@echo "Environment Variables:"
	@echo "  FERN_BASE_URL     - Base URL for Fern Platform (default: http://fern-platform.local:8080)"
	@echo "  FERN_USERNAME     - Username for authentication (default: fern-user@fern.com)"
	@echo "  FERN_PASSWORD     - Password for authentication (default: test123)"
	@echo "  FERN_TEAM_NAME    - Team name for test user (default: fern)"
	@echo "  FERN_HEADLESS     - Run browser in headless mode (default: true)"
	@echo "  FERN_RECORD_VIDEO - Record videos of test runs (default: false)"

.DEFAULT_GOAL := help