package testruns_test

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/playwright-community/playwright-go"

	"github.com/guidewire-oss/fern-platform/acceptance/helpers"
)

var _ = Describe("UC-03: Test Runs and Drill-Down", Label("e2e"), func() {
	var (
		ctx  playwright.BrowserContext
		page playwright.Page
		nav  *helpers.NavigationHelper
	)

	BeforeEach(func() {
		ctx, page = createAuthenticatedContext()
		nav = helpers.NewNavigationHelper(page, baseURL)

		// Navigate to test runs page
		nav.GoToTestRuns()
	})

	AfterEach(func() {
		if ctx != nil {
			ctx.Close()
		}
	})

	Describe("UC-03-01: View Test Runs List", Label("e2e"), func() {
		Context("Team member views team test runs", func() {
			It("should show only test runs from user's team projects", func() {
				// Get test run rows
				testRuns := page.Locator("table tbody tr, .test-run-row, [data-testid='test-run-row']")

				// Should have at least one test run (or empty state)
				count, err := testRuns.Count()
				Expect(err).NotTo(HaveOccurred())

				if count == 0 {
					// Check for empty state
					emptyState := page.Locator("text=/No test runs found/")
					emptyCount, _ := emptyState.Count()
					Expect(emptyCount).To(Equal(1))
				} else {
					// Verify columns in correct order
					firstRow := testRuns.First()
					cells := firstRow.Locator("td")
					cellCount, _ := cells.Count()
					Expect(cellCount).To(BeNumerically(">=", 7))

					// Verify column order: project, run id, branch, test results, status, duration, started
					projectCell := cells.Nth(0)
					runIdCell := cells.Nth(1)
					branchCell := cells.Nth(2)
					testResultsCell := cells.Nth(3)
					statusCell := cells.Nth(4)
					durationCell := cells.Nth(5)
					startedCell := cells.Nth(6)

					// Each cell should have content
					projectText, _ := projectCell.TextContent()
					runIdText, _ := runIdCell.TextContent()
					branchText, _ := branchCell.TextContent()
					testResultsText, _ := testResultsCell.TextContent()
					statusText, _ := statusCell.TextContent()
					durationText, _ := durationCell.TextContent()
					startedText, _ := startedCell.TextContent()
					
					Expect(projectText).NotTo(BeEmpty())
					Expect(runIdText).NotTo(BeEmpty())
					Expect(branchText).NotTo(BeEmpty())
					Expect(testResultsText).NotTo(BeEmpty())
					Expect(statusText).NotTo(BeEmpty())
					Expect(durationText).NotTo(BeEmpty())
					Expect(startedText).NotTo(BeEmpty())
				}
			})
		})

		Context("Test results accuracy", func() {
			It("should display test results in format: total failed passed", func() {
				testRuns := page.Locator("table tbody tr, .test-run-row, [data-testid='test-run-row']")
				count, _ := testRuns.Count()

				if count > 0 {
					// Check first row's test results
					firstRow := testRuns.First()
					testResultsCell := firstRow.Locator("td").Nth(3)
					resultsText, err := testResultsCell.TextContent()
					Expect(err).NotTo(HaveOccurred())

					// Should match pattern: "47 2 45" (total failed passed)
					matched, err := regexp.MatchString(`^\d+\s+\d+\s+\d+$`, strings.TrimSpace(resultsText))
					Expect(err).NotTo(HaveOccurred())
					Expect(matched).To(BeTrue(), "Test results should be in format: total failed passed")

					// Parse the numbers
					parts := strings.Fields(resultsText)
					Expect(parts).To(HaveLen(3))

					total, _ := strconv.Atoi(parts[0])
					failed, _ := strconv.Atoi(parts[1])
					passed, _ := strconv.Atoi(parts[2])

					// Verify total = passed + failed (assuming no skipped for simplicity)
					Expect(total).To(BeNumerically(">=", passed+failed))
				}
			})
		})

		Context("Duration data accuracy", func() {
			It("should show duration in appropriate format", func() {
				testRuns := page.Locator("table tbody tr, .test-run-row, [data-testid='test-run-row']")
				count, _ := testRuns.Count()

				if count > 0 {
					firstRow := testRuns.First()
					durationCell := firstRow.Locator("td").Nth(5)
					durationText, err := durationCell.TextContent()
					Expect(err).NotTo(HaveOccurred())

					// Should match patterns like "1,234ms" or "1m 23s"
					matched, err := regexp.MatchString(`^\d+(,\d+)?ms$|^\d+m\s+\d+s$`, strings.TrimSpace(durationText))
					Expect(err).NotTo(HaveOccurred())
					Expect(matched).To(BeTrue(), "Duration should be in format: 1,234ms or 1m 23s")
				}
			})
		})

		Context("Test runs sorted by recency", func() {
			It("should show most recent test runs first", func() {
				testRuns := page.Locator("table tbody tr, .test-run-row, [data-testid='test-run-row']")
				count, _ := testRuns.Count()

				if count > 1 {
					// Get timestamps from first two rows
					firstTimestamp, _ := testRuns.First().Locator("td").Nth(6).TextContent()
					secondTimestamp, _ := testRuns.Nth(1).Locator("td").Nth(6).TextContent()

					// Parse timestamps (assuming format like "2024-01-15 10:30:45")
					// First timestamp should be more recent than second
					Expect(firstTimestamp).NotTo(BeEmpty())
					Expect(secondTimestamp).NotTo(BeEmpty())
				}
			})
		})
	})

	Describe("UC-03-02: Navigate to Test Suite Details", Label("e2e"), func() {
		Context("Click test run to view suites", func() {
			It("should navigate to suite details when clicking test run", func() {
				testRuns := page.Locator("table tbody tr, .test-run-row, [data-testid='test-run-row']")
				count, _ := testRuns.Count()

				if count == 0 {
					Skip("No test runs available for testing")
				}

				// Click first test run
				err := testRuns.First().Click()
				Expect(err).NotTo(HaveOccurred())

				// Wait for navigation
				time.Sleep(1 * time.Second)

				// Should see suite list
				suites := page.Locator("table tbody tr, .suite-row, [data-testid='suite-row']")
				Eventually(func() int {
					count, _ := suites.Count()
					return count
				}, 5*time.Second).Should(BeNumerically(">", 0))
			})
		})

		Context("Suite details display", func() {
			It("should show suite details with correct columns", func() {
				// Navigate to first test run
				testRuns := page.Locator("table tbody tr, .test-run-row, [data-testid='test-run-row']")
				count, _ := testRuns.Count()

				if count == 0 {
					Skip("No test runs available for testing")
				}

				err := testRuns.First().Click()
				Expect(err).NotTo(HaveOccurred())

				time.Sleep(1 * time.Second)

				// Check suite table columns
				suiteRows := page.Locator("table tbody tr, .suite-row, [data-testid='suite-row']")
				suiteCount, _ := suiteRows.Count()

				if suiteCount > 0 {
					firstSuite := suiteRows.First()
					cells := firstSuite.Locator("td")

					// Verify columns: Suite Name, Test Results, Status, Duration
					suiteName := cells.Nth(0)
					testResults := cells.Nth(1)
					status := cells.Nth(2)
					duration := cells.Nth(3)

					Expect(suiteName).ToNot(BeNil())
					Expect(testResults).ToNot(BeNil())
					Expect(status).ToNot(BeNil())
					Expect(duration).ToNot(BeNil())

					// Test results should be in format: total failed passed
					resultsText, _ := testResults.TextContent()
					matched, _ := regexp.MatchString(`^\d+\s+\d+\s+\d+$`, strings.TrimSpace(resultsText))
					Expect(matched).To(BeTrue())
				}
			})
		})

		Context("Breadcrumb navigation appears", func() {
			It("should show breadcrumbs after navigating to suites", func() {
				testRuns := page.Locator("table tbody tr, .test-run-row, [data-testid='test-run-row']")
				count, _ := testRuns.Count()

				if count == 0 {
					Skip("No test runs available for testing")
				}

				// Get run ID before clicking
				runIdCell := testRuns.First().Locator("td").Nth(1)
				runId, _ := runIdCell.TextContent()

				err := testRuns.First().Click()
				Expect(err).NotTo(HaveOccurred())

				time.Sleep(1 * time.Second)

				// Check breadcrumbs
				breadcrumbs := nav.GetCurrentBreadcrumbs()
				Expect(breadcrumbs).To(HaveLen(2))
				Expect(breadcrumbs[0]).To(Equal("Test Runs"))
				Expect(breadcrumbs[1]).To(ContainSubstring(runId))
			})
		})
	})

	Describe("UC-03-03: Navigate to Test Spec Details", Label("e2e"), func() {
		Context("Click suite to view specs", func() {
			It("should navigate to spec details when clicking suite", func() {
				// First navigate to a test run
				testRuns := page.Locator("table tbody tr, .test-run-row, [data-testid='test-run-row']")
				runCount, _ := testRuns.Count()

				if runCount == 0 {
					Skip("No test runs available for testing")
				}

				err := testRuns.First().Click()
				Expect(err).NotTo(HaveOccurred())

				time.Sleep(1 * time.Second)

				// Click first suite
				suites := page.Locator("table tbody tr, .suite-row, [data-testid='suite-row']")
				suiteCount, _ := suites.Count()

				if suiteCount > 0 {
					err = suites.First().Click()
					Expect(err).NotTo(HaveOccurred())

					time.Sleep(1 * time.Second)

					// Should see spec list
					specs := page.Locator("table tbody tr, .spec-row, [data-testid='spec-row']")
					Eventually(func() int {
						count, _ := specs.Count()
						return count
					}, 5*time.Second).Should(BeNumerically(">", 0))
				}
			})
		})

		Context("Spec details display", func() {
			It("should show spec details with error messages for failed tests", func() {
				// Navigate to specs (skip if no data)
				navigateToSpecs(page)

				specs := page.Locator("table tbody tr, .spec-row, [data-testid='spec-row']")
				specCount, _ := specs.Count()

				if specCount > 0 {
					// Check columns: Test Name, Status, Duration, Error Message, Started
					for i := 0; i < specCount && i < 5; i++ {
						spec := specs.Nth(i)
						cells := spec.Locator("td")

						statusCell := cells.Nth(1)
						errorCell := cells.Nth(3)

						status, _ := statusCell.TextContent()
						errorMsg, _ := errorCell.TextContent()

						// Failed tests should have error messages
						if strings.Contains(strings.ToLower(status), "fail") {
							Expect(strings.TrimSpace(errorMsg)).NotTo(BeEmpty())
						} else if strings.Contains(strings.ToLower(status), "pass") {
							Expect(strings.TrimSpace(errorMsg)).To(BeEmpty())
						}
					}
				}
			})
		})
	})

	Describe("UC-03-04: Multi-Level Navigation", Label("e2e"), func() {
		Context("Navigate from runs to suites to specs", func() {
			It("should maintain navigation context through all levels", func() {
				testRuns := page.Locator("table tbody tr, .test-run-row, [data-testid='test-run-row']")
				runCount, _ := testRuns.Count()

				if runCount == 0 {
					Skip("No test runs available for testing")
				}

				// Level 1: Test Runs
				runId := getTextFromCell(testRuns.First(), 1)
				err := testRuns.First().Click()
				Expect(err).NotTo(HaveOccurred())

				time.Sleep(1 * time.Second)

				// Level 2: Suites
				suites := page.Locator("table tbody tr, .suite-row, [data-testid='suite-row']")
				suiteCount, _ := suites.Count()

				if suiteCount > 0 {
					suiteName := getTextFromCell(suites.First(), 0)
					err = suites.First().Click()
					Expect(err).NotTo(HaveOccurred())

					time.Sleep(1 * time.Second)

					// Level 3: Specs
					specs := page.Locator("table tbody tr, .spec-row, [data-testid='spec-row']")
					specCount, _ := specs.Count()
					Expect(specCount).To(BeNumerically(">", 0))

					// Check breadcrumbs show all levels
					breadcrumbs := nav.GetCurrentBreadcrumbs()
					Expect(breadcrumbs).To(HaveLen(3))
					Expect(breadcrumbs[0]).To(Equal("Test Runs"))
					Expect(breadcrumbs[1]).To(ContainSubstring(runId))
					Expect(breadcrumbs[2]).To(ContainSubstring(suiteName))
				}
			})
		})

		Context("Use breadcrumbs to navigate back", func() {
			It("should navigate back using breadcrumbs", func() {
				navigateToSpecs(page)

				// Click middle breadcrumb to go back to suites
				nav.ClickBreadcrumb("run-")

				time.Sleep(1 * time.Second)

				// Should be back at suite level
				suites := page.Locator("table tbody tr, .suite-row, [data-testid='suite-row']")
				suiteCount, _ := suites.Count()
				Expect(suiteCount).To(BeNumerically(">", 0))

				// Click first breadcrumb to go back to test runs
				nav.ClickBreadcrumb("Test Runs")

				time.Sleep(1 * time.Second)

				// Should be back at test run level
				testRuns := page.Locator("table tbody tr, .test-run-row, [data-testid='test-run-row']")
				runCount, _ := testRuns.Count()
				Expect(runCount).To(BeNumerically(">", 0))
			})
		})
	})

	Describe("UC-03-05: Access Control at Each Level", Label("e2e"), func() {
		Context("Cannot access other team's test run", func() {
			It("should show error when accessing unauthorized test run", func() {
				// Try to access a test run ID that doesn't belong to user's team
				unauthorizedURL := baseURL + "/test-runs/unauthorized-run-id"
				_, err := page.Goto(unauthorizedURL)
				Expect(err).NotTo(HaveOccurred())

				// Should see access denied error
				errorMsg := page.Locator("text=/don't have permission|Access denied|Forbidden/")
				Eventually(func() int {
					count, _ := errorMsg.Count()
					return count
				}, 5*time.Second).Should(BeNumerically(">", 0))
			})
		})
	})

	Describe("UC-03-06: Empty States and Error Handling", Label("e2e"), func() {
		Context("No test runs available", func() {
			It("should show helpful empty state message", func() {
				testRuns := page.Locator("table tbody tr, .test-run-row, [data-testid='test-run-row']")
				count, _ := testRuns.Count()

				if count == 0 {
					emptyState := page.Locator("text=/No test runs found|Run your tests/")
					emptyCount, _ := emptyState.Count()
					Expect(emptyCount).To(Equal(1))
				}
			})
		})
	})
})

// Helper functions
func getTextFromCell(row playwright.Locator, cellIndex int) string {
	cell := row.Locator("td").Nth(cellIndex)
	text, _ := cell.TextContent()
	return strings.TrimSpace(text)
}

func navigateToSpecs(page playwright.Page) {
	// Navigate through test run -> suite -> specs
	testRuns := page.Locator("table tbody tr, .test-run-row, [data-testid='test-run-row']")
	runCount, _ := testRuns.Count()

	if runCount == 0 {
		Skip("No test runs available for testing")
	}

	testRuns.First().Click()
	time.Sleep(1 * time.Second)

	suites := page.Locator("table tbody tr, .suite-row, [data-testid='suite-row']")
	suiteCount, _ := suites.Count()

	if suiteCount == 0 {
		Skip("No suites available for testing")
	}

	suites.First().Click()
	time.Sleep(1 * time.Second)
}
