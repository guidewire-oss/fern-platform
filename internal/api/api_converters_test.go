package api

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	projectsDomain "github.com/guidewire-oss/fern-platform/internal/domains/projects/domain"
	testingDomain "github.com/guidewire-oss/fern-platform/internal/domains/testing/domain"
)

var _ = Describe("DomainHandler helpers", func() {
	var h *DomainHandler

	BeforeEach(func() {
		h = &DomainHandler{}
	})

	Describe("convertSpecRuns", func() {
		It("computes durations, end pointers and sets error/failure messages correctly", func() {
			start := time.Now()
			end := start.Add(1500 * time.Millisecond)

			req := []SpecRun{
				{ID: 1, SuiteID: 10, SpecDescription: "ok", Status: "passed", StartTime: start, EndTime: end},
				{ID: 2, SuiteID: 10, SpecDescription: "fail", Status: "failed", StartTime: start, EndTime: end, Message: "assertion"},
				{ID: 3, SuiteID: 10, SpecDescription: "err", Status: "error", StartTime: start, EndTime: end, Message: "panic"},
				{ID: 4, SuiteID: 10, SpecDescription: "skipped", Status: "skipped", StartTime: time.Time{}, EndTime: time.Time{}},
			}

			domain := h.convertSpecRuns(req)
			Expect(domain).To(HaveLen(4))

			// passed
			Expect(domain[0].Name).To(Equal("ok"))
			Expect(domain[0].Status).To(Equal("passed"))
			Expect(domain[0].Duration.Seconds()).To(BeNumerically("~", 1.5, 0.01))
			Expect(domain[0].EndTime).ToNot(BeNil())

			// failed -> FailureMessage
			Expect(domain[1].Status).To(Equal("failed"))
			Expect(domain[1].FailureMessage).To(Equal("assertion"))
			Expect(domain[1].ErrorMessage).To(Equal(""))

			// error -> ErrorMessage
			Expect(domain[2].Status).To(Equal("error"))
			Expect(domain[2].ErrorMessage).To(Equal("panic"))
			Expect(domain[2].FailureMessage).To(Equal(""))

			// skipped -> zero duration and nil EndTime
			Expect(domain[3].Duration).To(Equal(time.Duration(0)))
			Expect(domain[3].EndTime).To(BeNil())
		})
	})

	Describe("calculateTestCounts", func() {
		It("counts status synonyms correctly", func() {
			specs := []*testingDomain.SpecRun{
				{Status: "passed"},
				{Status: "pass"},
				{Status: "failed"},
				{Status: "fail"},
				{Status: "error"},
				{Status: "skipped"},
				{Status: "skip"},
				{Status: "pending"},
				{Status: "weird"},
			}

			total, passed, failed, skipped := h.calculateTestCounts(specs)
			Expect(total).To(Equal(len(specs)))
			Expect(passed).To(Equal(2))
			Expect(failed).To(Equal(3))
			Expect(skipped).To(Equal(3))
		})
	})

	Describe("calculateSuiteStatus", func() {
		It("returns unknown for empty", func() {
			Expect(h.calculateSuiteStatus([]*testingDomain.SpecRun{})).To(Equal("unknown"))
		})

		It("returns failed when any failure/error present", func() {
			specs := []*testingDomain.SpecRun{
				{Status: "passed"},
				{Status: "fail"},
			}
			Expect(h.calculateSuiteStatus(specs)).To(Equal("failed"))
		})

		It("returns partial when skipped present but no failures", func() {
			specs := []*testingDomain.SpecRun{
				{Status: "passed"},
				{Status: "skipped"},
			}
			Expect(h.calculateSuiteStatus(specs)).To(Equal("partial"))
		})

		It("returns passed when all passing", func() {
			specs := []*testingDomain.SpecRun{
				{Status: "pass"},
				{Status: "passed"},
			}
			Expect(h.calculateSuiteStatus(specs)).To(Equal("passed"))
		})
	})

	Describe("convertApiSuiteRunstoDomain", func() {
		It("translates suites including durations, counts and statuses", func() {
			start := time.Now()
			end := start.Add(500 * time.Millisecond)

			reqSuites := []SuiteRun{
				{
					ID:        100,
					SuiteName: "suite-with-end",
					StartTime: start,
					EndTime:   end,
					SpecRuns: []SpecRun{
						{ID: 1, SuiteID: 100, SpecDescription: "a", Status: "passed", StartTime: start, EndTime: end},
						{ID: 2, SuiteID: 100, SpecDescription: "b", Status: "failed", StartTime: start, EndTime: end, Message: "boom"},
						{ID: 3, SuiteID: 100, SpecDescription: "c", Status: "skipped"},
					},
				},
				{
					ID:        101,
					SuiteName: "suite-no-end",
					StartTime: start,
					EndTime:   time.Time{}, // missing end
					SpecRuns: []SpecRun{
						{ID: 4, SuiteID: 101, SpecDescription: "d", Status: "pass", StartTime: start, EndTime: end},
					},
				},
			}

			ds := h.convertApiSuiteRunstoDomain(reqSuites)
			Expect(ds).To(HaveLen(2))

			// first suite assertions
			s1 := ds[0]
			Expect(s1.Name).To(Equal("suite-with-end"))
			Expect(s1.TotalTests).To(Equal(3))
			Expect(s1.PassedTests).To(Equal(1))
			Expect(s1.FailedTests).To(Equal(1))
			Expect(s1.SkippedTests).To(Equal(1))
			Expect(s1.Status).To(Equal("failed"))
			Expect(s1.Duration).To(BeNumerically("~", 500*time.Millisecond, 200*time.Millisecond))
			Expect(s1.EndTime).ToNot(BeNil())

			// second suite assertions (no end -> duration zero and EndTime nil)
			s2 := ds[1]
			Expect(s2.Name).To(ContainSubstring("no-end"))
			Expect(s2.TotalTests).To(Equal(1))
			Expect(s2.PassedTests).To(Equal(1))
			Expect(s2.Status).To(Equal("passed"))
			Expect(s2.Duration).To(Equal(time.Duration(0)))
			Expect(s2.EndTime).To(BeNil())
		})
	})

	Describe("calculateOverallTestCounts", func() {
		It("sums totals from suites", func() {
			suites := []testingDomain.SuiteRun{
				{TotalTests: 2, PassedTests: 2, FailedTests: 0, SkippedTests: 0},
				{TotalTests: 3, PassedTests: 2, FailedTests: 1, SkippedTests: 0},
			}

			total, passed, failed, skipped := h.calculateOverallTestCounts(suites)
			Expect(total).To(Equal(5))
			Expect(passed).To(Equal(4))
			Expect(failed).To(Equal(1))
			Expect(skipped).To(Equal(0))
		})
	})

	Describe("calculateOverallStatus", func() {
		It("returns failed if any suite contains failed spec", func() {
			suites := []SuiteRun{
				{SuiteName: "s1", SpecRuns: []SpecRun{{SpecDescription: "a", Status: "passed"}}},
				{SuiteName: "s2", SpecRuns: []SpecRun{{SpecDescription: "b", Status: "failed"}}},
			}
			Expect(h.calculateOverallStatus(suites)).To(Equal("failed"))
		})

		It("returns passed if no failed specs", func() {
			suites := []SuiteRun{
				{SuiteName: "s1", SpecRuns: []SpecRun{{SpecDescription: "a", Status: "skipped"}}},
				{SuiteName: "s2", SpecRuns: []SpecRun{{SpecDescription: "b", Status: "pass"}}},
			}
			Expect(h.calculateOverallStatus(suites)).To(Equal("passed"))
		})
	})

	Describe("convertDomainTestRunToAPI", func() {
		It("converts a domain TestRun into gin.H map and returns numeric seconds for duration", func() {
			now := time.Now()
			end := now.Add(3 * time.Second)
			tr := &testingDomain.TestRun{
				ID:           55,
				RunID:        "run-1",
				ProjectID:    "proj-xyz",
				Branch:       "main",
				GitCommit:    "abcdef",
				Status:       "passed",
				StartTime:    now,
				EndTime:      &end,
				Duration:     3 * time.Second,
				TotalTests:   10,
				PassedTests:  9,
				FailedTests:  1,
				SkippedTests: 0,
				Environment:  "ci",
				Metadata:     map[string]interface{}{"k": "v"},
			}

			apiMap := h.convertDomainTestRunToAPI(tr)
			Expect(apiMap).To(HaveKeyWithValue("id", tr.ID))
			Expect(apiMap).To(HaveKeyWithValue("runId", tr.RunID))
			Expect(apiMap["duration"]).To(BeNumerically("~", 3.0, 0.0001))
			Expect(apiMap).To(HaveKeyWithValue("totalTests", tr.TotalTests))
			Expect(apiMap).To(HaveKeyWithValue("metadata", tr.Metadata))
		})
	})

	Describe("convertProjectToAPI", func() {
		It("returns snapshot fields mapped to API map", func() {
			// create project via constructor and mutate via setters
			proj, err := projectsDomain.NewProject(projectsDomain.ProjectID("proj-1"), "MyProject", projectsDomain.Team("team-abc"))
			Expect(err).To(BeNil())

			// set additional fields via available methods
			err = proj.UpdateDefaultBranch("develop")
			Expect(err).To(BeNil())
			proj.UpdateDescription("desc")
			proj.UpdateRepository("https://repo")
			proj.SetSetting("x", "y")

			snap := proj.ToSnapshot()
			apiMap := h.convertProjectToAPI(proj)

			Expect(apiMap["id"]).To(Equal(snap.ID))
			Expect(apiMap["projectId"]).To(Equal(string(snap.ProjectID)))
			Expect(apiMap["name"]).To(Equal(snap.Name))
			Expect(apiMap["description"]).To(Equal(snap.Description))
			Expect(apiMap["repository"]).To(Equal(snap.Repository))
			Expect(apiMap["defaultBranch"]).To(Equal(snap.DefaultBranch))
			Expect(apiMap["team"]).To(Equal(string(snap.Team)))
			Expect(apiMap["isActive"]).To(Equal(snap.IsActive))
			Expect(apiMap["settings"]).To(Equal(snap.Settings))
			Expect(apiMap["createdAt"]).To(Equal(snap.CreatedAt))
			Expect(apiMap["updatedAt"]).To(Equal(snap.UpdatedAt))
		})
	})
})
