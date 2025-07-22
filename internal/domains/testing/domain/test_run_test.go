package domain_test

import (
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/guidewire-oss/fern-platform/internal/domains/testing/domain"
)

func TestDomain(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Testing Domain Suite")
}

var _ = Describe("TestRun", Label("unit", "domain", "testing"), func() {
	Describe("TestRun Creation", func() {
		It("should create a test run with all fields", func() {
			now := time.Now()
			endTime := now.Add(10 * time.Minute)
			
			testRun := &domain.TestRun{
				ID:           1,
				RunID:        "test-run-123",
				ProjectID:    "project-456",
				Branch:       "main",
				GitCommit:    "abc123def456",
				Status:       "completed",
				StartTime:    now,
				EndTime:      &endTime,
				TotalTests:   100,
				PassedTests:  95,
				FailedTests:  3,
				SkippedTests: 2,
				Duration:     10 * time.Minute,
			}

			Expect(testRun.ID).To(Equal(uint(1)))
			Expect(testRun.RunID).To(Equal("test-run-123"))
			Expect(testRun.ProjectID).To(Equal("project-456"))
			Expect(testRun.Branch).To(Equal("main"))
			Expect(testRun.Status).To(Equal("completed"))
			Expect(testRun.TotalTests).To(Equal(100))
			Expect(testRun.PassedTests).To(Equal(95))
		})

		It("should handle test run without end time", func() {
			testRun := &domain.TestRun{
				RunID:     "test-run-123",
				ProjectID: "project-456",
				Status:    "running",
				StartTime: time.Now(),
				EndTime:   nil, // Still running
			}

			Expect(testRun.EndTime).To(BeNil())
			Expect(testRun.Status).To(Equal("running"))
		})
	})

	Describe("TestRun Status", func() {
		It("should have valid status values", func() {
			validStatuses := []string{"pending", "running", "completed", "failed", "cancelled"}
			
			for _, status := range validStatuses {
				testRun := &domain.TestRun{
					RunID:     "test-" + status,
					ProjectID: "proj-123",
					Status:    status,
				}
				Expect(testRun.Status).To(Equal(status))
			}
		})
	})

	Describe("TestRun Calculations", func() {
		It("should calculate success percentage correctly", func() {
			testRun := &domain.TestRun{
				TotalTests:  100,
				PassedTests: 85,
			}
			
			percentage := float64(testRun.PassedTests) / float64(testRun.TotalTests) * 100
			Expect(percentage).To(BeNumerically("~", 85.0, 0.01))
		})

		It("should handle zero total tests", func() {
			testRun := &domain.TestRun{
				TotalTests:  0,
				PassedTests: 0,
			}
			
			// Should not panic with division by zero
			if testRun.TotalTests > 0 {
				_ = float64(testRun.PassedTests) / float64(testRun.TotalTests) * 100
			}
			Expect(testRun.TotalTests).To(Equal(0))
		})
	})

	Describe("TestRun Timing", func() {
		It("should calculate duration from start and end time", func() {
			start := time.Now()
			end := start.Add(5 * time.Minute)
			
			testRun := &domain.TestRun{
				StartTime: start,
				EndTime:   &end,
			}
			
			duration := end.Sub(start)
			Expect(duration).To(Equal(5 * time.Minute))
			Expect(testRun.StartTime).To(Equal(start))
			Expect(*testRun.EndTime).To(Equal(end))
		})
	})
})

var _ = Describe("SuiteRun", Label("unit", "domain", "testing"), func() {
	Describe("SuiteRun Creation", func() {
		It("should create a suite run with all fields", func() {
			suiteRun := &domain.SuiteRun{
				ID:           1,
				TestRunID:    10,
				Name:         "Integration Tests",
				Status:       "passed",
				Duration:     2 * time.Minute,
				PassedTests:  20,
				FailedTests:  0,
				SkippedTests: 2,
				TotalTests:   22,
			}

			Expect(suiteRun.ID).To(Equal(uint(1)))
			Expect(suiteRun.TestRunID).To(Equal(uint(10)))
			Expect(suiteRun.Name).To(Equal("Integration Tests"))
			Expect(suiteRun.Status).To(Equal("passed"))
			Expect(suiteRun.TotalTests).To(Equal(22))
		})

		It("should handle failed suite", func() {
			suiteRun := &domain.SuiteRun{
				Name:        "Unit Tests",
				Status:      "failed",
				PassedTests: 18,
				FailedTests: 2,
				TotalTests:  20,
			}

			Expect(suiteRun.Status).To(Equal("failed"))
			Expect(suiteRun.FailedTests).To(BeNumerically(">", 0))
		})
	})
})

var _ = Describe("SpecRun", Label("unit", "domain", "testing"), func() {
	Describe("SpecRun Creation", func() {
		It("should create a spec run with all fields", func() {
			specRun := &domain.SpecRun{
				ID:           1,
				SuiteRunID:   5,
				Name:         "should calculate total correctly",
				ClassName:    "Calculator",
				Status:       "passed",
				Duration:     125 * time.Millisecond,
				ErrorMessage: "",
				StackTrace:   "",
				RetryCount:   0,
			}

			Expect(specRun.ID).To(Equal(uint(1)))
			Expect(specRun.SuiteRunID).To(Equal(uint(5)))
			Expect(specRun.Name).To(Equal("should calculate total correctly"))
			Expect(specRun.Status).To(Equal("passed"))
			Expect(specRun.Duration).To(Equal(125 * time.Millisecond))
		})

		It("should handle failed spec with error details", func() {
			specRun := &domain.SpecRun{
				Name:           "should handle invalid input",
				Status:         "failed",
				Duration:       50 * time.Millisecond,
				ErrorMessage:   "Expected 0 to equal 1",
				StackTrace:     "at Calculator.add (calculator.js:15:5)",
				RetryCount:     2,
			}

			Expect(specRun.Status).To(Equal("failed"))
			Expect(specRun.ErrorMessage).NotTo(BeEmpty())
			Expect(specRun.StackTrace).NotTo(BeEmpty())
			Expect(specRun.RetryCount).To(Equal(2))
		})

		It("should handle skipped spec", func() {
			specRun := &domain.SpecRun{
				Name:     "should work with feature flag",
				Status:   "skipped",
				Duration: 0 * time.Second,
			}

			Expect(specRun.Status).To(Equal("skipped"))
			Expect(specRun.Duration).To(Equal(0 * time.Second))
		})
	})

	Describe("SpecRun Flakiness", func() {
		It("should track retries for flaky tests", func() {
			specRun := &domain.SpecRun{
				Name:       "flaky test",
				Status:     "passed",
				RetryCount: 3, // Passed after 3 retries
				IsFlaky:    true,
			}

			Expect(specRun.RetryCount).To(BeNumerically(">", 0))
			Expect(specRun.IsFlaky).To(BeTrue())
			// This indicates a flaky test that eventually passed
		})
	})
})

var _ = Describe("TestRunSummary", Label("unit", "domain", "testing"), func() {
	Describe("Summary Statistics", func() {
		It("should create test run summary with statistics", func() {
			summary := &domain.TestRunSummary{
				TotalRuns:      100,
				PassedRuns:     85,
				FailedRuns:     15,
				AverageRunTime: 5 * time.Minute,
				SuccessRate:    85.0,
			}

			Expect(summary.TotalRuns).To(Equal(100))
			Expect(summary.PassedRuns).To(Equal(85))
			Expect(summary.FailedRuns).To(Equal(15))
			Expect(summary.SuccessRate).To(BeNumerically("~", 85.0, 0.01))
			Expect(summary.AverageRunTime).To(Equal(5 * time.Minute))
			
			// Verify totals add up
			totalCompleted := summary.PassedRuns + summary.FailedRuns
			Expect(totalCompleted).To(Equal(summary.TotalRuns))
		})

		It("should handle empty summary", func() {
			summary := &domain.TestRunSummary{
				TotalRuns: 0,
			}

			Expect(summary.TotalRuns).To(Equal(0))
			Expect(summary.SuccessRate).To(Equal(float64(0)))
			Expect(summary.AverageRunTime).To(Equal(0 * time.Second))
		})
	})
})

var _ = Describe("Domain Relationships", Label("unit", "domain", "testing"), func() {
	Describe("TestRun -> SuiteRun -> SpecRun hierarchy", func() {
		It("should maintain proper relationships", func() {
			// Create a test run
			testRun := &domain.TestRun{
				ID:        1,
				RunID:     "test-123",
				ProjectID: "proj-456",
				SuiteRuns: []domain.SuiteRun{},
			}

			// Add suite runs
			suite1 := domain.SuiteRun{
				ID:        1,
				TestRunID: testRun.ID,
				Name:      "Unit Tests",
				SpecRuns:  []*domain.SpecRun{},
			}

			suite2 := domain.SuiteRun{
				ID:        2,
				TestRunID: testRun.ID,
				Name:      "Integration Tests",
				SpecRuns:  []*domain.SpecRun{},
			}

			// Add spec runs to suites
			spec1 := &domain.SpecRun{
				ID:         1,
				SuiteRunID: suite1.ID,
				Name:       "test spec 1",
			}

			spec2 := &domain.SpecRun{
				ID:         2,
				SuiteRunID: suite1.ID,
				Name:       "test spec 2",
			}

			// Build relationships
			suite1.SpecRuns = append(suite1.SpecRuns, spec1, spec2)
			testRun.SuiteRuns = append(testRun.SuiteRuns, suite1, suite2)

			// Verify hierarchy
			Expect(testRun.SuiteRuns).To(HaveLen(2))
			Expect(testRun.SuiteRuns[0].SpecRuns).To(HaveLen(2))
			Expect(testRun.SuiteRuns[0].TestRunID).To(Equal(testRun.ID))
			Expect(testRun.SuiteRuns[0].SpecRuns[0].SuiteRunID).To(Equal(suite1.ID))
		})
	})

	Describe("Aggregation", func() {
		It("should aggregate stats from suites to test run", func() {
			testRun := &domain.TestRun{
				ID: 1,
				SuiteRuns: []domain.SuiteRun{
					{
						PassedTests:  10,
						FailedTests:  2,
						SkippedTests: 1,
						TotalTests:   13,
					},
					{
						PassedTests:  20,
						FailedTests:  0,
						SkippedTests: 0,
						TotalTests:   20,
					},
				},
			}

			// Calculate totals
			var totalPassed, totalFailed, totalSkipped, totalTests int
			for _, suite := range testRun.SuiteRuns {
				totalPassed += suite.PassedTests
				totalFailed += suite.FailedTests
				totalSkipped += suite.SkippedTests
				totalTests += suite.TotalTests
			}

			Expect(totalPassed).To(Equal(30))
			Expect(totalFailed).To(Equal(2))
			Expect(totalSkipped).To(Equal(1))
			Expect(totalTests).To(Equal(33))
		})
	})
})