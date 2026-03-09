package domain_test

import (
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/guidewire-oss/fern-platform/internal/domains/summary/domain"
)

func TestSummaryDomain(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Summary Domain Suite")
}

var _ = Describe("Summary Domain Types", func() {

	Describe("SummaryRequest", func() {
		It("should hold project UUID, seed, and groupBy fields", func() {
			req := domain.SummaryRequest{
				ProjectUUID: "proj-123",
				Seed:        "seed-abc",
				GroupBy:     []string{"component", "priority"},
			}
			Expect(req.ProjectUUID).To(Equal("proj-123"))
			Expect(req.Seed).To(Equal("seed-abc"))
			Expect(req.GroupBy).To(HaveLen(2))
			Expect(req.GroupBy).To(ContainElements("component", "priority"))
		})

		It("should allow empty GroupBy", func() {
			req := domain.SummaryRequest{
				ProjectUUID: "proj-1",
				Seed:        "seed-1",
				GroupBy:     []string{},
			}
			Expect(req.GroupBy).To(BeEmpty())
		})
	})

	Describe("SummaryResponse", func() {
		It("should hold all response fields", func() {
			resp := domain.SummaryResponse{
				ProjectID: "proj-1",
				Seed:      "seed-1",
				Branch:    "main",
				SHA:       "abc123",
				Status:    "passed",
				Tests:     42,
				StartTime: "2025-01-01T10:00:00Z",
				EndTime:   "2025-01-01T10:30:00Z",
				Summary:   []map[string]interface{}{},
			}
			Expect(resp.ProjectID).To(Equal("proj-1"))
			Expect(resp.Seed).To(Equal("seed-1"))
			Expect(resp.Branch).To(Equal("main"))
			Expect(resp.SHA).To(Equal("abc123"))
			Expect(resp.Status).To(Equal("passed"))
			Expect(resp.Tests).To(Equal(42))
			Expect(resp.StartTime).To(Equal("2025-01-01T10:00:00Z"))
			Expect(resp.EndTime).To(Equal("2025-01-01T10:30:00Z"))
			Expect(resp.Summary).To(BeEmpty())
		})

		It("should allow summary entries with group data", func() {
			entry := map[string]interface{}{
				"component": "auth",
				"total":     10,
				"passed":    8,
				"failed":    2,
				"skipped":   0,
				"pending":   0,
			}
			resp := domain.SummaryResponse{
				Summary: []map[string]interface{}{entry},
			}
			Expect(resp.Summary).To(HaveLen(1))
			Expect(resp.Summary[0]["component"]).To(Equal("auth"))
			Expect(resp.Summary[0]["total"]).To(Equal(10))
		})

		It("should allow omitting optional fields", func() {
			resp := domain.SummaryResponse{
				ProjectID: "proj-1",
				Seed:      "seed-1",
				Status:    "NA",
				Tests:     0,
			}
			Expect(resp.SHA).To(BeEmpty())
			Expect(resp.StartTime).To(BeEmpty())
			Expect(resp.EndTime).To(BeEmpty())
			Expect(resp.Branch).To(BeEmpty())
		})
	})

	Describe("GroupedTestSummary", func() {
		It("should hold group keys and counts", func() {
			summary := domain.GroupedTestSummary{
				GroupKeys: map[string]string{
					"component": "auth",
					"priority":  "high",
				},
				Total:   10,
				Passed:  7,
				Failed:  2,
				Skipped: 1,
				Pending: 0,
			}
			Expect(summary.GroupKeys).To(HaveKeyWithValue("component", "auth"))
			Expect(summary.GroupKeys).To(HaveKeyWithValue("priority", "high"))
			Expect(summary.Total).To(Equal(10))
			Expect(summary.Passed).To(Equal(7))
			Expect(summary.Failed).To(Equal(2))
			Expect(summary.Skipped).To(Equal(1))
			Expect(summary.Pending).To(Equal(0))
		})

		It("should allow empty group keys", func() {
			summary := domain.GroupedTestSummary{
				GroupKeys: map[string]string{},
				Total:     5,
				Passed:    5,
			}
			Expect(summary.GroupKeys).To(BeEmpty())
			Expect(summary.Total).To(Equal(5))
		})
	})

	Describe("TestRunData", func() {
		It("should hold branch, SHA, times and suite runs", func() {
			start := time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC)
			end := time.Date(2025, 1, 1, 10, 30, 0, 0, time.UTC)
			data := domain.TestRunData{
				GitBranch: "main",
				GitSHA:    "abc123",
				StartTime: start,
				EndTime:   end,
				SuiteRuns: []domain.SuiteRunData{
					{
						SpecRuns: []domain.SpecRunData{
							{Status: "passed"},
						},
					},
				},
			}
			Expect(data.GitBranch).To(Equal("main"))
			Expect(data.GitSHA).To(Equal("abc123"))
			Expect(data.StartTime).To(Equal(start))
			Expect(data.EndTime).To(Equal(end))
			Expect(data.SuiteRuns).To(HaveLen(1))
			Expect(data.SuiteRuns[0].SpecRuns).To(HaveLen(1))
		})

		It("should allow empty suite runs", func() {
			data := domain.TestRunData{
				GitBranch: "develop",
				SuiteRuns: []domain.SuiteRunData{},
			}
			Expect(data.SuiteRuns).To(BeEmpty())
		})
	})

	Describe("SuiteRunData", func() {
		It("should hold spec runs and tags", func() {
			suite := domain.SuiteRunData{
				SpecRuns: []domain.SpecRunData{
					{Status: "passed", Tags: []domain.TagData{{Category: "env", Value: "prod"}}},
					{Status: "failed", Tags: []domain.TagData{{Category: "env", Value: "staging"}}},
				},
				Tags: []domain.TagData{
					{Category: "suite-level", Value: "smoke"},
				},
			}
			Expect(suite.SpecRuns).To(HaveLen(2))
			Expect(suite.Tags).To(HaveLen(1))
			Expect(suite.Tags[0].Category).To(Equal("suite-level"))
		})
	})

	Describe("SpecRunData", func() {
		It("should hold status and tags", func() {
			spec := domain.SpecRunData{
				Status: "passed",
				Tags: []domain.TagData{
					{Category: "component", Value: "auth"},
					{Category: "priority", Value: "high"},
				},
			}
			Expect(spec.Status).To(Equal("passed"))
			Expect(spec.Tags).To(HaveLen(2))
		})

		It("should allow no tags", func() {
			spec := domain.SpecRunData{
				Status: "failed",
				Tags:   []domain.TagData{},
			}
			Expect(spec.Tags).To(BeEmpty())
		})
	})

	Describe("TagData", func() {
		It("should hold category and value", func() {
			tag := domain.TagData{
				Category: "component",
				Value:    "auth",
			}
			Expect(tag.Category).To(Equal("component"))
			Expect(tag.Value).To(Equal("auth"))
		})

		It("should allow empty category", func() {
			tag := domain.TagData{
				Value: "standalone",
			}
			Expect(tag.Category).To(BeEmpty())
			Expect(tag.Value).To(Equal("standalone"))
		})
	})

	Describe("StatusCounts", func() {
		It("should hold all status count fields", func() {
			counts := domain.StatusCounts{
				Total:   100,
				Passed:  80,
				Failed:  10,
				Skipped: 5,
				Pending: 5,
			}
			Expect(counts.Total).To(Equal(100))
			Expect(counts.Passed).To(Equal(80))
			Expect(counts.Failed).To(Equal(10))
			Expect(counts.Skipped).To(Equal(5))
			Expect(counts.Pending).To(Equal(5))
		})

		It("should default to zero values", func() {
			counts := domain.StatusCounts{}
			Expect(counts.Total).To(Equal(0))
			Expect(counts.Passed).To(Equal(0))
			Expect(counts.Failed).To(Equal(0))
			Expect(counts.Skipped).To(Equal(0))
			Expect(counts.Pending).To(Equal(0))
		})
	})
})
