package application_test

import (
	"fmt"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/guidewire-oss/fern-platform/internal/domains/summary/application"
	"github.com/guidewire-oss/fern-platform/internal/domains/summary/domain"
)

func TestSummaryService(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "SummaryService Suite")
}

// --- Mock Repository ---
type mockSummaryRepo struct {
	testRuns []domain.TestRunData
	err      error
}

func (m *mockSummaryRepo) GetTestRunsBySeed(projectUUID string, seed string) ([]domain.TestRunData, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.testRuns, nil
}

var _ = Describe("SummaryService", func() {
	var (
		service *application.SummaryService
		repo    *mockSummaryRepo
	)

	BeforeEach(func() {
		repo = &mockSummaryRepo{}
		service = application.NewSummaryService(repo)
	})

	Describe("GetSummary", func() {
		Context("when repository returns error", func() {
			It("should return error", func() {
				repo.err = fmt.Errorf("database error")
				req := domain.SummaryRequest{
					ProjectUUID: "proj-123",
					Seed:        "seed-456",
					GroupBy:     []string{"component"},
				}

				result, err := service.GetSummary(req)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("database error"))
				Expect(result).To(BeNil())
			})
		})

		Context("when no test runs are found", func() {
			It("should return empty response with NA values", func() {
				repo.testRuns = []domain.TestRunData{}
				req := domain.SummaryRequest{
					ProjectUUID: "proj-123",
					Seed:        "seed-456",
					GroupBy:     []string{"component"},
				}

				result, err := service.GetSummary(req)

				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.ProjectID).To(Equal("proj-123"))
				Expect(result.Seed).To(Equal("seed-456"))
				Expect(result.Branch).To(Equal("NA"))
				Expect(result.Status).To(Equal("NA"))
				Expect(result.Tests).To(Equal(0))
				Expect(result.Summary).To(HaveLen(0))
			})
		})

		Context("when test runs exist with single grouping", func() {
			It("should aggregate by single tag", func() {
				startTime := time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC)
				endTime := time.Date(2025, 1, 1, 10, 30, 0, 0, time.UTC)

				repo.testRuns = []domain.TestRunData{
					{
						GitBranch: "main",
						GitSHA:    "abc123",
						StartTime: startTime,
						EndTime:   endTime,
						SuiteRuns: []domain.SuiteRunData{
							{
								SpecRuns: []domain.SpecRunData{
									{
										Status: "passed",
										Tags: []domain.TagData{
											{Category: "component", Value: "auth"},
										},
									},
									{
										Status: "failed",
										Tags: []domain.TagData{
											{Category: "component", Value: "auth"},
										},
									},
									{
										Status: "passed",
										Tags: []domain.TagData{
											{Category: "component", Value: "api"},
										},
									},
								},
							},
						},
					},
				}

				req := domain.SummaryRequest{
					ProjectUUID: "proj-123",
					Seed:        "seed-456",
					GroupBy:     []string{"component"},
				}

				result, err := service.GetSummary(req)

				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.ProjectID).To(Equal("proj-123"))
				Expect(result.Seed).To(Equal("seed-456"))
				Expect(result.Branch).To(Equal("main"))
				Expect(result.SHA).To(Equal("abc123"))
				Expect(result.Status).To(Equal("failed"))
				Expect(result.Tests).To(Equal(3))
				Expect(result.StartTime).To(Equal("2025-01-01T10:00:00Z"))
				Expect(result.EndTime).To(Equal("2025-01-01T10:30:00Z"))
				Expect(result.Summary).To(HaveLen(2))

				// Verify summary entries (order is sorted)
				summaryMap := make(map[string]map[string]interface{})
				for _, entry := range result.Summary {
					component := entry["component"].(string)
					summaryMap[component] = entry
				}

				Expect(summaryMap["api"]).To(HaveKeyWithValue("component", "api"))
				Expect(summaryMap["api"]).To(HaveKeyWithValue("total", 1))
				Expect(summaryMap["api"]).To(HaveKeyWithValue("passed", 1))

				Expect(summaryMap["auth"]).To(HaveKeyWithValue("component", "auth"))
				Expect(summaryMap["auth"]).To(HaveKeyWithValue("total", 2))
				Expect(summaryMap["auth"]).To(HaveKeyWithValue("passed", 1))
				Expect(summaryMap["auth"]).To(HaveKeyWithValue("failed", 1))
			})
		})

		Context("when test runs exist with multiple groupings", func() {
			It("should aggregate by multiple tags", func() {
				repo.testRuns = []domain.TestRunData{
					{
						GitBranch: "develop",
						GitSHA:    "def456",
						StartTime: time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC),
						EndTime:   time.Date(2025, 1, 1, 10, 30, 0, 0, time.UTC),
						SuiteRuns: []domain.SuiteRunData{
							{
								SpecRuns: []domain.SpecRunData{
									{
										Status: "passed",
										Tags: []domain.TagData{
											{Category: "component", Value: "auth"},
											{Category: "priority", Value: "high"},
										},
									},
									{
										Status: "failed",
										Tags: []domain.TagData{
											{Category: "component", Value: "auth"},
											{Category: "priority", Value: "low"},
										},
									},
									{
										Status: "passed",
										Tags: []domain.TagData{
											{Category: "component", Value: "api"},
											{Category: "priority", Value: "high"},
										},
									},
								},
							},
						},
					},
				}

				req := domain.SummaryRequest{
					ProjectUUID: "proj-123",
					Seed:        "seed-456",
					GroupBy:     []string{"component", "priority"},
				}

				result, err := service.GetSummary(req)

				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.Tests).To(Equal(3))
				Expect(result.Summary).To(HaveLen(3))

				// Verify each group exists
				foundGroups := make(map[string]bool)
				for _, entry := range result.Summary {
					key := fmt.Sprintf("%s|%s", entry["component"], entry["priority"])
					foundGroups[key] = true
				}

				Expect(foundGroups).To(HaveKey("api|high"))
				Expect(foundGroups).To(HaveKey("auth|high"))
				Expect(foundGroups).To(HaveKey("auth|low"))
			})
		})

		Context("when tags are missing", func() {
			It("should use 'unspecified' for missing tags", func() {
				repo.testRuns = []domain.TestRunData{
					{
						GitBranch: "main",
						GitSHA:    "abc123",
						StartTime: time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC),
						EndTime:   time.Date(2025, 1, 1, 10, 30, 0, 0, time.UTC),
						SuiteRuns: []domain.SuiteRunData{
							{
								SpecRuns: []domain.SpecRunData{
									{
										Status: "passed",
										Tags:   []domain.TagData{}, // No tags
									},
									{
										Status: "failed",
										Tags: []domain.TagData{
											{Category: "component", Value: "auth"},
										},
									},
								},
							},
						},
					},
				}

				req := domain.SummaryRequest{
					ProjectUUID: "proj-123",
					Seed:        "seed-456",
					GroupBy:     []string{"component"},
				}

				result, err := service.GetSummary(req)

				Expect(err).NotTo(HaveOccurred())
				Expect(result.Summary).To(HaveLen(2))

				// Check for unspecified entry
				foundUnspecified := false
				for _, entry := range result.Summary {
					if entry["component"] == "unspecified" {
						foundUnspecified = true
						Expect(entry["total"]).To(Equal(1))
						Expect(entry["passed"]).To(Equal(1))
					}
				}
				Expect(foundUnspecified).To(BeTrue())
			})
		})

		Context("when test runs have different statuses", func() {
			It("should count all status types", func() {
				repo.testRuns = []domain.TestRunData{
					{
						GitBranch: "main",
						GitSHA:    "abc123",
						StartTime: time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC),
						EndTime:   time.Date(2025, 1, 1, 10, 30, 0, 0, time.UTC),
						SuiteRuns: []domain.SuiteRunData{
							{
								SpecRuns: []domain.SpecRunData{
									{
										Status: "passed",
										Tags:   []domain.TagData{{Category: "component", Value: "auth"}},
									},
									{
										Status: "failed",
										Tags:   []domain.TagData{{Category: "component", Value: "auth"}},
									},
									{
										Status: "skipped",
										Tags:   []domain.TagData{{Category: "component", Value: "auth"}},
									},
									{
										Status: "pending",
										Tags:   []domain.TagData{{Category: "component", Value: "auth"}},
									},
								},
							},
						},
					},
				}

				req := domain.SummaryRequest{
					ProjectUUID: "proj-123",
					Seed:        "seed-456",
					GroupBy:     []string{"component"},
				}

				result, err := service.GetSummary(req)

				Expect(err).NotTo(HaveOccurred())
				Expect(result.Summary).To(HaveLen(1))

				authSummary := result.Summary[0]
				Expect(authSummary["component"]).To(Equal("auth"))
				Expect(authSummary["total"]).To(Equal(4))
				Expect(authSummary["passed"]).To(Equal(1))
				Expect(authSummary["failed"]).To(Equal(1))
				Expect(authSummary["skipped"]).To(Equal(1))
				Expect(authSummary["pending"]).To(Equal(1))
			})
		})

		Context("when overall status is passed", func() {
			It("should return status as passed", func() {
				repo.testRuns = []domain.TestRunData{
					{
						GitBranch: "main",
						GitSHA:    "abc123",
						StartTime: time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC),
						EndTime:   time.Date(2025, 1, 1, 10, 30, 0, 0, time.UTC),
						SuiteRuns: []domain.SuiteRunData{
							{
								SpecRuns: []domain.SpecRunData{
									{Status: "passed", Tags: []domain.TagData{{Category: "component", Value: "auth"}}},
									{Status: "passed", Tags: []domain.TagData{{Category: "component", Value: "api"}}},
								},
							},
						},
					},
				}

				req := domain.SummaryRequest{
					ProjectUUID: "proj-123",
					Seed:        "seed-456",
					GroupBy:     []string{"component"},
				}

				result, err := service.GetSummary(req)

				Expect(err).NotTo(HaveOccurred())
				Expect(result.Status).To(Equal("passed"))
			})
		})

		Context("when multiple test runs exist", func() {
			It("should aggregate across all test runs", func() {
				repo.testRuns = []domain.TestRunData{
					{
						GitBranch: "main",
						GitSHA:    "abc123",
						StartTime: time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC),
						EndTime:   time.Date(2025, 1, 1, 10, 15, 0, 0, time.UTC),
						SuiteRuns: []domain.SuiteRunData{
							{
								SpecRuns: []domain.SpecRunData{
									{Status: "passed", Tags: []domain.TagData{{Category: "component", Value: "auth"}}},
								},
							},
						},
					},
					{
						GitBranch: "main",
						GitSHA:    "abc123",
						StartTime: time.Date(2025, 1, 1, 10, 15, 0, 0, time.UTC),
						EndTime:   time.Date(2025, 1, 1, 10, 30, 0, 0, time.UTC),
						SuiteRuns: []domain.SuiteRunData{
							{
								SpecRuns: []domain.SpecRunData{
									{Status: "failed", Tags: []domain.TagData{{Category: "component", Value: "auth"}}},
								},
							},
						},
					},
				}

				req := domain.SummaryRequest{
					ProjectUUID: "proj-123",
					Seed:        "seed-456",
					GroupBy:     []string{"component"},
				}

				result, err := service.GetSummary(req)

				Expect(err).NotTo(HaveOccurred())
				Expect(result.Tests).To(Equal(2))
				Expect(result.StartTime).To(Equal("2025-01-01T10:00:00Z"))
				Expect(result.EndTime).To(Equal("2025-01-01T10:30:00Z"))
				Expect(result.Summary).To(HaveLen(1))

				authSummary := result.Summary[0]
				Expect(authSummary["component"]).To(Equal("auth"))
				Expect(authSummary["total"]).To(Equal(2))
				Expect(authSummary["passed"]).To(Equal(1))
				Expect(authSummary["failed"]).To(Equal(1))
			})
		})

		Context("when times are zero", func() {
			It("should not include time fields", func() {
				repo.testRuns = []domain.TestRunData{
					{
						GitBranch: "main",
						GitSHA:    "abc123",
						StartTime: time.Time{}, // Zero time
						EndTime:   time.Time{}, // Zero time
						SuiteRuns: []domain.SuiteRunData{
							{
								SpecRuns: []domain.SpecRunData{
									{Status: "passed", Tags: []domain.TagData{{Category: "component", Value: "auth"}}},
								},
							},
						},
					},
				}

				req := domain.SummaryRequest{
					ProjectUUID: "proj-123",
					Seed:        "seed-456",
					GroupBy:     []string{"component"},
				}

				result, err := service.GetSummary(req)

				Expect(err).NotTo(HaveOccurred())
				Expect(result.StartTime).To(BeEmpty())
				Expect(result.EndTime).To(BeEmpty())
			})
		})

		Context("when no groupBy is specified", func() {
			It("should create a single summary group", func() {
				repo.testRuns = []domain.TestRunData{
					{
						GitBranch: "main",
						GitSHA:    "abc123",
						StartTime: time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC),
						EndTime:   time.Date(2025, 1, 1, 10, 30, 0, 0, time.UTC),
						SuiteRuns: []domain.SuiteRunData{
							{
								SpecRuns: []domain.SpecRunData{
									{Status: "passed", Tags: []domain.TagData{{Category: "component", Value: "auth"}}},
									{Status: "failed", Tags: []domain.TagData{{Category: "component", Value: "api"}}},
								},
							},
						},
					},
				}

				req := domain.SummaryRequest{
					ProjectUUID: "proj-123",
					Seed:        "seed-456",
					GroupBy:     []string{}, // Empty groupBy
				}

				result, err := service.GetSummary(req)

				Expect(err).NotTo(HaveOccurred())
				Expect(result.Tests).To(Equal(2))
				Expect(result.Summary).To(HaveLen(1))

				summary := result.Summary[0]
				Expect(summary["total"]).To(Equal(2))
				Expect(summary["passed"]).To(Equal(1))
				Expect(summary["failed"]).To(Equal(1))
			})
		})
	})
})
