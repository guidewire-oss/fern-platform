package domain_test

import (
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/guidewire-oss/fern-platform/internal/domains/analytics/domain"
)

func TestAnalyticsDomain(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Analytics Domain Suite")
}

var _ = Describe("FlakyTest", func() {
	Describe("struct construction", func() {
		It("should create a FlakyTest with all fields", func() {
			now := time.Now()
			ft := domain.FlakyTest{
				TestID:       "proj1_testA",
				ProjectID:    "proj1",
				TestName:     "testA",
				SuiteName:    "suite1",
				PackageName:  "pkg/foo",
				FirstSeen:    now.Add(-24 * time.Hour),
				LastSeen:     now,
				TotalRuns:    50,
				FailureCount: 10,
				FlakeScore:   0.2,
				Status:       domain.StatusActive,
				Metadata: domain.FlakyTestMetadata{
					FailurePatterns: []string{"timeout"},
					Environments:    []string{"ci"},
					Tags:            []string{"flaky"},
					RecentFailures: []domain.TestFailureInfo{
						{
							TestRunID:    "run1",
							FailedAt:     now,
							ErrorMessage: "timeout exceeded",
							Duration:     5 * time.Second,
							Environment:  "ci",
						},
					},
				},
			}

			Expect(ft.TestID).To(Equal("proj1_testA"))
			Expect(ft.ProjectID).To(Equal("proj1"))
			Expect(ft.TestName).To(Equal("testA"))
			Expect(ft.SuiteName).To(Equal("suite1"))
			Expect(ft.PackageName).To(Equal("pkg/foo"))
			Expect(ft.TotalRuns).To(Equal(50))
			Expect(ft.FailureCount).To(Equal(10))
			Expect(ft.FlakeScore).To(Equal(0.2))
			Expect(ft.Status).To(Equal(domain.StatusActive))
			Expect(ft.Metadata.FailurePatterns).To(HaveLen(1))
			Expect(ft.Metadata.Environments).To(HaveLen(1))
			Expect(ft.Metadata.Tags).To(HaveLen(1))
			Expect(ft.Metadata.RecentFailures).To(HaveLen(1))
			Expect(ft.Metadata.RecentFailures[0].ErrorMessage).To(Equal("timeout exceeded"))
		})
	})

	Describe("FlakyTestStatus constants", func() {
		It("should have correct status values", func() {
			Expect(string(domain.StatusActive)).To(Equal("active"))
			Expect(string(domain.StatusResolved)).To(Equal("resolved"))
			Expect(string(domain.StatusIgnored)).To(Equal("ignored"))
		})
	})
})

var _ = Describe("TestRunAnalysis", func() {
	It("should track new, still flaky, and resolved tests", func() {
		analysis := domain.TestRunAnalysis{
			TestRunID:     "run-123",
			ProjectID:     "proj-1",
			AnalyzedAt:    time.Now(),
			TotalTests:    100,
			NewFlaky:      []string{"test1", "test2"},
			StillFlaky:    []string{"test3"},
			ResolvedFlaky: []string{"test4", "test5", "test6"},
		}

		Expect(analysis.TestRunID).To(Equal("run-123"))
		Expect(analysis.TotalTests).To(Equal(100))
		Expect(analysis.NewFlaky).To(HaveLen(2))
		Expect(analysis.StillFlaky).To(HaveLen(1))
		Expect(analysis.ResolvedFlaky).To(HaveLen(3))
	})

	It("should handle empty slices", func() {
		analysis := domain.TestRunAnalysis{
			TestRunID:  "run-empty",
			ProjectID:  "proj-1",
			TotalTests: 50,
		}

		Expect(analysis.NewFlaky).To(BeNil())
		Expect(analysis.StillFlaky).To(BeNil())
		Expect(analysis.ResolvedFlaky).To(BeNil())
	})
})

var _ = Describe("FlakyTestDetectionConfig", func() {
	Describe("DefaultFlakyTestDetectionConfig", func() {
		It("should return sensible defaults", func() {
			config := domain.DefaultFlakyTestDetectionConfig()

			Expect(config.MinimumRuns).To(Equal(10))
			Expect(config.MinFailureRate).To(Equal(0.05))
			Expect(config.MaxFailureRate).To(Equal(0.95))
			Expect(config.AnalysisWindow).To(Equal(7 * 24 * time.Hour))
			Expect(config.ConsecutivePassesForResolution).To(Equal(20))
		})

		It("should have MinFailureRate less than MaxFailureRate", func() {
			config := domain.DefaultFlakyTestDetectionConfig()
			Expect(config.MinFailureRate).To(BeNumerically("<", config.MaxFailureRate))
		})
	})

	It("should allow custom configuration", func() {
		config := domain.FlakyTestDetectionConfig{
			MinimumRuns:                    5,
			MinFailureRate:                 0.1,
			MaxFailureRate:                 0.8,
			AnalysisWindow:                 3 * 24 * time.Hour,
			ConsecutivePassesForResolution: 10,
		}

		Expect(config.MinimumRuns).To(Equal(5))
		Expect(config.MinFailureRate).To(Equal(0.1))
		Expect(config.MaxFailureRate).To(Equal(0.8))
		Expect(config.AnalysisWindow).To(Equal(3 * 24 * time.Hour))
		Expect(config.ConsecutivePassesForResolution).To(Equal(10))
	})
})

var _ = Describe("TestExecutionResult", func() {
	It("should store execution result data", func() {
		result := domain.TestExecutionResult{
			TestRunID:   "run-1",
			TestName:    "TestFoo",
			SuiteName:   "MySuite",
			Status:      "failed",
			Duration:    2 * time.Second,
			ExecutedAt:  time.Now(),
			Error:       "assertion failed",
			Environment: map[string]string{"os": "linux", "go": "1.21"},
		}

		Expect(result.TestRunID).To(Equal("run-1"))
		Expect(result.TestName).To(Equal("TestFoo"))
		Expect(result.SuiteName).To(Equal("MySuite"))
		Expect(result.Status).To(Equal("failed"))
		Expect(result.Duration).To(Equal(2 * time.Second))
		Expect(result.Error).To(Equal("assertion failed"))
		Expect(result.Environment).To(HaveLen(2))
		Expect(result.Environment["os"]).To(Equal("linux"))
	})
})

var _ = Describe("TestFailureInfo", func() {
	It("should store failure details", func() {
		info := domain.TestFailureInfo{
			TestRunID:    "run-42",
			FailedAt:     time.Now(),
			ErrorMessage: "nil pointer dereference",
			Duration:     100 * time.Millisecond,
			Environment:  "staging",
		}

		Expect(info.TestRunID).To(Equal("run-42"))
		Expect(info.ErrorMessage).To(Equal("nil pointer dereference"))
		Expect(info.Duration).To(Equal(100 * time.Millisecond))
		Expect(info.Environment).To(Equal("staging"))
	})
})
