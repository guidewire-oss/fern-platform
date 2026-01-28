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

		It("returns skipped when skipped present but no failures", func() {
			specs := []*testingDomain.SpecRun{
				{Status: "passed"},
				{Status: "skipped"},
			}
			Expect(h.calculateSuiteStatus(specs)).To(Equal("skipped"))
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

	Describe("convertApiTagsToDomain", func() {
		It("returns nil for empty tag array", func() {
			tags := h.convertApiTagsToDomain([]Tag{})
			Expect(tags).To(BeNil())
		})

		It("converts API tags to domain tags", func() {
			apiTags := []Tag{
				{ID: 1, Name: "priority:high"},
				{ID: 2, Name: "browser:chrome"},
			}

			domainTags := h.convertApiTagsToDomain(apiTags)
			Expect(domainTags).To(HaveLen(2))
			Expect(domainTags[0].ID).To(Equal(uint(1)))
			Expect(domainTags[0].Name).To(Equal("priority:high"))
			Expect(domainTags[1].ID).To(Equal(uint(2)))
			Expect(domainTags[1].Name).To(Equal("browser:chrome"))
		})
	})

	Describe("mergeUniqueTags", func() {
		It("merges two tag slices without duplicates", func() {
			existing := []testingDomain.Tag{
				{ID: 1, Name: "priority:high", Category: "priority", Value: "high"},
				{ID: 2, Name: "browser:chrome", Category: "browser", Value: "chrome"},
			}

			newTags := []testingDomain.Tag{
				{ID: 2, Name: "browser:chrome", Category: "browser", Value: "chrome"}, // duplicate
				{ID: 3, Name: "smoke", Category: "", Value: "smoke"},
			}

			merged := h.mergeUniqueTags(existing, newTags)
			Expect(merged).To(HaveLen(3)) // 1, 2, 3

			tagIDs := make(map[uint]bool)
			for _, tag := range merged {
				tagIDs[tag.ID] = true
			}
			Expect(tagIDs).To(HaveKey(uint(1)))
			Expect(tagIDs).To(HaveKey(uint(2)))
			Expect(tagIDs).To(HaveKey(uint(3)))
		})

		It("ignores tags with ID 0 in both slices", func() {
			existing := []testingDomain.Tag{
				{ID: 0, Name: "unprocessed1"},
				{ID: 1, Name: "tag1"},
			}

			newTags := []testingDomain.Tag{
				{ID: 0, Name: "unprocessed2"},
				{ID: 2, Name: "tag2"},
			}

			merged := h.mergeUniqueTags(existing, newTags)
			Expect(merged).To(HaveLen(2))
			for _, tag := range merged {
				Expect(tag.ID).NotTo(Equal(uint(0)))
			}
		})

		It("handles empty slices", func() {
			existing := []testingDomain.Tag{
				{ID: 1, Name: "tag1"},
			}

			merged1 := h.mergeUniqueTags(existing, []testingDomain.Tag{})
			Expect(merged1).To(HaveLen(1))

			merged2 := h.mergeUniqueTags([]testingDomain.Tag{}, existing)
			Expect(merged2).To(HaveLen(1))

			merged3 := h.mergeUniqueTags([]testingDomain.Tag{}, []testingDomain.Tag{})
			Expect(merged3).To(HaveLen(0))
		})
	})

	Describe("convertSpecRuns with tags", func() {
		It("converts tags from API SpecRuns to domain SpecRuns", func() {
			start := time.Now()
			end := start.Add(1 * time.Second)

			req := []SpecRun{
				{
					ID:              1,
					SuiteID:         10,
					SpecDescription: "test with tags",
					Status:          "passed",
					StartTime:       start,
					EndTime:         end,
					Tags: []Tag{
						{ID: 1, Name: "smoke"},
						{ID: 2, Name: "priority:high"},
					},
				},
			}

			domain := h.convertSpecRuns(req)
			Expect(domain).To(HaveLen(1))
			Expect(domain[0].Tags).To(HaveLen(2))
			Expect(domain[0].Tags[0].ID).To(Equal(uint(1)))
			Expect(domain[0].Tags[1].ID).To(Equal(uint(2)))
		})
	})

	Describe("convertApiSuiteRunstoDomain with tags", func() {
		It("converts tags from API SuiteRuns to domain SuiteRuns", func() {
			start := time.Now()
			end := start.Add(500 * time.Millisecond)

			reqSuites := []SuiteRun{
				{
					ID:        100,
					SuiteName: "suite-with-tags",
					StartTime: start,
					EndTime:   end,
					Tags: []Tag{
						{ID: 1, Name: "regression"},
					},
					SpecRuns: []SpecRun{
						{
							ID:              1,
							SuiteID:         100,
							SpecDescription: "spec",
							Status:          "passed",
							StartTime:       start,
							EndTime:         end,
							Tags: []Tag{
								{ID: 2, Name: "smoke"},
							},
						},
					},
				},
			}

			ds := h.convertApiSuiteRunstoDomain(reqSuites)
			Expect(ds).To(HaveLen(1))
			Expect(ds[0].Tags).To(HaveLen(1))
			Expect(ds[0].Tags[0].ID).To(Equal(uint(1)))
			Expect(ds[0].SpecRuns).To(HaveLen(1))
			Expect(ds[0].SpecRuns[0].Tags).To(HaveLen(1))
			Expect(ds[0].SpecRuns[0].Tags[0].ID).To(Equal(uint(2)))
		})
	})

	Describe("convertDomainTestRunToAPI with tags", func() {
		It("includes tags in the API response", func() {
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
				Tags: []testingDomain.Tag{
					{ID: 1, Name: "priority:high", Category: "priority", Value: "high"},
					{ID: 2, Name: "smoke", Category: "", Value: "smoke"},
				},
				Metadata: map[string]interface{}{"k": "v"},
			}

			apiMap := h.convertDomainTestRunToAPI(tr)
			Expect(apiMap).To(HaveKey("tags"))
			tags := apiMap["tags"].([]testingDomain.Tag)
			Expect(tags).To(HaveLen(2))
			Expect(tags[0].ID).To(Equal(uint(1)))
			Expect(tags[1].ID).To(Equal(uint(2)))
		})
	})

	Describe("Tag propagation from JSON request to domain", func() {
		It("should convert JSON request with tags to domain objects", func() {
			// This simulates what happens when a JSON request comes in
			// The JSON is unmarshaled into the API request structs
			start := time.Now()
			end := start.Add(1 * time.Second)

			// Simulate JSON request body after c.ShouldBindJSON
			req := TestRunRequest{
				TestProjectID: "project-123",
				SuiteRuns: []SuiteRun{
					{
						ID:        1,
						SuiteName: "Test Suite",
						StartTime: start,
						EndTime:   end,
						Tags: []Tag{
							{ID: 1, Name: "suite-tag"},
						},
						SpecRuns: []SpecRun{
							{
								ID:              1,
								SuiteID:         1,
								SpecDescription: "Test Spec",
								Status:          "passed",
								StartTime:       start,
								EndTime:         end,
								Tags: []Tag{
									{ID: 2, Name: "spec-tag"},
								},
							},
						},
					},
				},
			}

			// Convert to domain (this is what happens in recordTestRun)
			domainSuiteRuns := h.convertApiSuiteRunstoDomain(req.SuiteRuns)

			// Verify tags made it through
			Expect(domainSuiteRuns).To(HaveLen(1))
			Expect(domainSuiteRuns[0].Tags).To(HaveLen(1))
			Expect(domainSuiteRuns[0].Tags[0].ID).To(Equal(uint(1)))
			Expect(domainSuiteRuns[0].Tags[0].Name).To(Equal("suite-tag"))

			Expect(domainSuiteRuns[0].SpecRuns).To(HaveLen(1))
			Expect(domainSuiteRuns[0].SpecRuns[0].Tags).To(HaveLen(1))
			Expect(domainSuiteRuns[0].SpecRuns[0].Tags[0].ID).To(Equal(uint(2)))
			Expect(domainSuiteRuns[0].SpecRuns[0].Tags[0].Name).To(Equal("spec-tag"))
		})
	})

	Describe("Tag propagation from API to domain", func() {
		It("should propagate tags from API SpecRuns to domain SpecRuns", func() {
			start := time.Now()
			end := start.Add(1 * time.Second)

			// Create API request with tags in SpecRuns
			apiSpecRuns := []SpecRun{
				{
					ID:              1,
					SuiteID:         100,
					SpecDescription: "spec with tags",
					Status:          "passed",
					StartTime:       start,
					EndTime:         end,
					Tags: []Tag{
						{ID: 10, Name: "smoke"},
						{ID: 11, Name: "priority:high"},
					},
				},
				{
					ID:              2,
					SuiteID:         100,
					SpecDescription: "spec without tags",
					Status:          "passed",
					StartTime:       start,
					EndTime:         end,
					Tags:            []Tag{}, // Empty tags
				},
			}

			// Convert to domain
			domainSpecRuns := h.convertSpecRuns(apiSpecRuns)

			Expect(domainSpecRuns[0].Tags).To(HaveLen(2))
			Expect(domainSpecRuns[0].Tags[0].ID).To(Equal(uint(10)))
			Expect(domainSpecRuns[0].Tags[0].Name).To(Equal("smoke"))
			Expect(domainSpecRuns[0].Tags[1].ID).To(Equal(uint(11)))
			Expect(domainSpecRuns[0].Tags[1].Name).To(Equal("priority:high"))

			// Verify second spec has no tags
			Expect(domainSpecRuns[1].Tags).To(BeNil())
		})

		It("should propagate tags from API SuiteRuns to domain SuiteRuns", func() {
			start := time.Now()
			end := start.Add(500 * time.Millisecond)

			// Create API request with tags in SuiteRuns
			apiSuiteRuns := []SuiteRun{
				{
					ID:        100,
					SuiteName: "suite with tags",
					StartTime: start,
					EndTime:   end,
					Tags: []Tag{
						{ID: 20, Name: "regression"},
						{ID: 21, Name: "browser:chrome"},
					},
					SpecRuns: []SpecRun{
						{
							ID:              1,
							SuiteID:         100,
							SpecDescription: "spec",
							Status:          "passed",
							StartTime:       start,
							EndTime:         end,
							Tags: []Tag{
								{ID: 30, Name: "smoke"},
							},
						},
					},
				},
			}

			domainSuiteRuns := h.convertApiSuiteRunstoDomain(apiSuiteRuns)

			// Verify suite has tags
			Expect(domainSuiteRuns[0].Tags).To(HaveLen(2))
			Expect(domainSuiteRuns[0].Tags[0].ID).To(Equal(uint(20)))
			Expect(domainSuiteRuns[0].Tags[0].Name).To(Equal("regression"))
			Expect(domainSuiteRuns[0].Tags[1].ID).To(Equal(uint(21)))
			Expect(domainSuiteRuns[0].Tags[1].Name).To(Equal("browser:chrome"))

			// Verify spec within suite also has tags
			Expect(domainSuiteRuns[0].SpecRuns).To(HaveLen(1))
			Expect(domainSuiteRuns[0].SpecRuns[0].Tags).To(HaveLen(1))
			Expect(domainSuiteRuns[0].SpecRuns[0].Tags[0].ID).To(Equal(uint(30)))
			Expect(domainSuiteRuns[0].SpecRuns[0].Tags[0].Name).To(Equal("smoke"))
		})

		It("should propagate tags through complete conversion chain", func() {
			start := time.Now()
			end := start.Add(500 * time.Millisecond)

			// Create API request with tags at multiple levels
			apiSuiteRuns := []SuiteRun{
				{
					ID:        100,
					SuiteName: "suite 1",
					StartTime: start,
					EndTime:   end,
					Tags: []Tag{
						{ID: 1, Name: "suite-tag"},
					},
					SpecRuns: []SpecRun{
						{
							ID:              1,
							SuiteID:         100,
							SpecDescription: "spec 1",
							Status:          "passed",
							StartTime:       start,
							EndTime:         end,
							Tags: []Tag{
								{ID: 2, Name: "spec-tag-1"},
							},
						},
						{
							ID:              2,
							SuiteID:         100,
							SpecDescription: "spec 2",
							Status:          "passed",
							StartTime:       start,
							EndTime:         end,
							Tags: []Tag{
								{ID: 3, Name: "spec-tag-2"},
							},
						},
					},
				},
				{
					ID:        101,
					SuiteName: "suite 2",
					StartTime: start,
					EndTime:   end,
					Tags: []Tag{
						{ID: 4, Name: "another-suite-tag"},
					},
					SpecRuns: []SpecRun{
						{
							ID:              3,
							SuiteID:         101,
							SpecDescription: "spec 3",
							Status:          "passed",
							StartTime:       start,
							EndTime:         end,
							Tags: []Tag{
								{ID: 5, Name: "spec-tag-3"},
							},
						},
					},
				},
			}

			// Convert to domain
			domainSuiteRuns := h.convertApiSuiteRunstoDomain(apiSuiteRuns)

			// Verify all tags are propagated
			Expect(domainSuiteRuns).To(HaveLen(2))

			// Suite 1
			Expect(domainSuiteRuns[0].Tags).To(HaveLen(1))
			Expect(domainSuiteRuns[0].Tags[0].Name).To(Equal("suite-tag"))
			Expect(domainSuiteRuns[0].SpecRuns).To(HaveLen(2))
			Expect(domainSuiteRuns[0].SpecRuns[0].Tags).To(HaveLen(1))
			Expect(domainSuiteRuns[0].SpecRuns[0].Tags[0].Name).To(Equal("spec-tag-1"))
			Expect(domainSuiteRuns[0].SpecRuns[1].Tags).To(HaveLen(1))
			Expect(domainSuiteRuns[0].SpecRuns[1].Tags[0].Name).To(Equal("spec-tag-2"))

			// Suite 2
			Expect(domainSuiteRuns[1].Tags).To(HaveLen(1))
			Expect(domainSuiteRuns[1].Tags[0].Name).To(Equal("another-suite-tag"))
			Expect(domainSuiteRuns[1].SpecRuns).To(HaveLen(1))
			Expect(domainSuiteRuns[1].SpecRuns[0].Tags).To(HaveLen(1))
			Expect(domainSuiteRuns[1].SpecRuns[0].Tags[0].Name).To(Equal("spec-tag-3"))
		})

		It("should handle mixed scenarios with some tags and some without", func() {
			start := time.Now()
			end := start.Add(500 * time.Millisecond)

			apiSuiteRuns := []SuiteRun{
				{
					ID:        100,
					SuiteName: "suite with tags",
					StartTime: start,
					EndTime:   end,
					Tags:      []Tag{{ID: 10, Name: "tag1"}},
					SpecRuns: []SpecRun{
						{ID: 1, SuiteID: 100, SpecDescription: "spec with tags", Status: "passed", StartTime: start, EndTime: end, Tags: []Tag{{ID: 11, Name: "tag2"}}},
						{ID: 2, SuiteID: 100, SpecDescription: "spec without tags", Status: "passed", StartTime: start, EndTime: end, Tags: nil},
					},
				},
				{
					ID:        101,
					SuiteName: "suite without tags",
					StartTime: start,
					EndTime:   end,
					Tags:      nil, // No tags
					SpecRuns: []SpecRun{
						{
							ID:              3,
							SuiteID:         101,
							SpecDescription: "spec",
							Status:          "passed",
							StartTime:       start,
							EndTime:         end,
							Tags:            []Tag{}, // Empty array
						},
					},
				},
			}

			domainSuiteRuns := h.convertApiSuiteRunstoDomain(apiSuiteRuns)

			// Suite 1 should have tags
			Expect(domainSuiteRuns[0].Tags).To(HaveLen(1))
			Expect(domainSuiteRuns[0].SpecRuns[0].Tags).To(HaveLen(1))
			Expect(domainSuiteRuns[0].SpecRuns[1].Tags).To(BeNil())

			Expect(domainSuiteRuns[1].Tags).To(BeNil())
			Expect(domainSuiteRuns[1].SpecRuns[0].Tags).To(BeNil())
		})
	})
})
