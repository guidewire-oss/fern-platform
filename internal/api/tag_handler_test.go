package api

import (
	"bytes"
	"context"
	"encoding/json"
	_ "errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	_ "time"

	"github.com/gin-gonic/gin"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	tagsApp "github.com/guidewire-oss/fern-platform/internal/domains/tags/application"
	tagsDomain "github.com/guidewire-oss/fern-platform/internal/domains/tags/domain"
	"github.com/guidewire-oss/fern-platform/pkg/logging"
)

// --- In-memory repository for testing ---
// Implements the minimal interface used by TagService: FindByName, Save, FindByID, FindAll, Delete, AssignToTestRun

type inMemoryTagRepo struct {
	mu       sync.Mutex
	nextID   int
	byID     map[tagsDomain.TagID]*tagsDomain.Tag
	byName   map[string]*tagsDomain.Tag
	assigns  map[string][]tagsDomain.TagID
	failFind bool // toggle to simulate errors when needed
}

func newInMemoryTagRepo() *inMemoryTagRepo {
	return &inMemoryTagRepo{
		nextID:  1,
		byID:    make(map[tagsDomain.TagID]*tagsDomain.Tag),
		byName:  make(map[string]*tagsDomain.Tag),
		assigns: make(map[string][]tagsDomain.TagID),
	}
}

func (r *inMemoryTagRepo) FindByName(ctx context.Context, name string) (*tagsDomain.Tag, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	name = strings.ToLower(name)
	if r.failFind {
		return nil, fmt.Errorf("repo failure")
	}
	if t, ok := r.byName[name]; ok {
		return t, nil
	}
	return nil, fmt.Errorf("not found")
}

func (r *inMemoryTagRepo) Save(ctx context.Context, t *tagsDomain.Tag) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	// Assign ID if empty
	id := t.ID()
	if id == "" {
		id = tagsDomain.TagID(strconv.Itoa(r.nextID))
		r.nextID++
	}
	rt := tagsDomain.ReconstructTag(id, strings.ToLower(t.Name()), t.Category(), t.Value(), t.CreatedAt())
	r.byID[id] = rt
	r.byName[rt.Name()] = rt
	return nil
}

func (r *inMemoryTagRepo) FindByID(ctx context.Context, id tagsDomain.TagID) (*tagsDomain.Tag, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.failFind {
		return nil, fmt.Errorf("repo failure")
	}
	if t, ok := r.byID[id]; ok {
		return t, nil
	}
	return nil, fmt.Errorf("not found")
}

func (r *inMemoryTagRepo) FindAll(ctx context.Context) ([]*tagsDomain.Tag, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.failFind {
		return nil, fmt.Errorf("repo failure")
	}
	out := make([]*tagsDomain.Tag, 0, len(r.byID))
	for _, t := range r.byID {
		out = append(out, t)
	}
	return out, nil
}

func (r *inMemoryTagRepo) Delete(ctx context.Context, id tagsDomain.TagID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.byID[id]; !ok {
		return fmt.Errorf("not found")
	}
	name := r.byID[id].Name()
	delete(r.byID, id)
	delete(r.byName, name)
	return nil
}

func (r *inMemoryTagRepo) AssignToTestRun(ctx context.Context, testRunID string, tagIDs []tagsDomain.TagID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	// ensure each tag exists
	for _, id := range tagIDs {
		if _, ok := r.byID[id]; !ok {
			return fmt.Errorf("tag %s not found", id)
		}
	}
	r.assigns[testRunID] = append(r.assigns[testRunID], tagIDs...)
	return nil
}

// --- Tests ---

var _ = Describe("TagHandler & tag processing", func() {
	var (
		repo    *inMemoryTagRepo
		svc     *tagsApp.TagService
		handler *TagHandler
		router  *gin.Engine
	)

	BeforeEach(func() {
		gin.SetMode(gin.TestMode)
		repo = newInMemoryTagRepo()
		svc = tagsApp.NewTagService(repo)
		// create a minimal logger; the code under test doesn't depend on logger behaviour
		var lg logging.Logger
		// Construct handler with value copy (NewTagHandler does deref)
		handler = NewTagHandler(svc, &lg)
		router = gin.New()
		userGroup := router.Group("/api/v1")
		adminGroup := router.Group("/api/v1/admin")
		handler.RegisterRoutes(userGroup, adminGroup)
	})

	Describe("processTagList and ProcessTestRunTags", func() {
		It("GetOrCreateTag will create new tags and mutate request", func() {
			// Prepare request with run-level tag and nested tags
			req := &TestRunRequest{
				Tags: []Tag{{Name: "Priority:High"}},
				SuiteRuns: []SuiteRun{
					{
						Tags: []Tag{{Name: "env:prod"}},
						SpecRuns: []SpecRun{
							{Tags: []Tag{{Name: "smoke"}}},
						},
					},
				},
			}

			err := ProcessTestRunTags(context.Background(), svc, req)
			Expect(err).To(BeNil())
			// After processing, tags should have IDs assigned (non-zero)
			Expect(len(req.Tags)).To(Equal(1))
			Expect(req.Tags[0].ID).NotTo(Equal(uint64(0)))
			Expect(len(req.SuiteRuns)).To(Equal(1))
			Expect(req.SuiteRuns[0].Tags[0].ID).NotTo(Equal(uint64(0)))
			Expect(req.SuiteRuns[0].SpecRuns[0].Tags[0].ID).NotTo(Equal(uint64(0)))
		})

		It("returns error when repository fails during GetOrCreateTag", func() {
			// make repo fail
			repo.failFind = true
			req := &TestRunRequest{
				Tags: []Tag{{Name: "x"}},
			}
			err := ProcessTestRunTags(context.Background(), svc, req)
			Expect(err).ToNot(BeNil())
		})

		It("processes request with no tags at all levels", func() {
			req := &TestRunRequest{
				Tags: []Tag{},
				SuiteRuns: []SuiteRun{
					{
						Tags: []Tag{},
						SpecRuns: []SpecRun{
							{Tags: []Tag{}},
						},
					},
				},
			}

			err := ProcessTestRunTags(context.Background(), svc, req)
			Expect(err).To(BeNil())
			Expect(len(req.Tags)).To(Equal(0))
			Expect(len(req.SuiteRuns[0].Tags)).To(Equal(0))
			Expect(len(req.SuiteRuns[0].SpecRuns[0].Tags)).To(Equal(0))
		})

		It("processes request with multiple suite runs and spec runs with tags", func() {
			req := &TestRunRequest{
				Tags: []Tag{{Name: "run-tag"}},
				SuiteRuns: []SuiteRun{
					{
						Tags: []Tag{{Name: "suite1-tag"}},
						SpecRuns: []SpecRun{
							{Tags: []Tag{{Name: "spec1-tag"}}},
							{Tags: []Tag{{Name: "spec2-tag"}}},
						},
					},
					{
						Tags: []Tag{{Name: "suite2-tag"}},
						SpecRuns: []SpecRun{
							{Tags: []Tag{{Name: "spec3-tag"}}},
						},
					},
				},
			}

			err := ProcessTestRunTags(context.Background(), svc, req)
			Expect(err).To(BeNil())
			// Verify all tags got IDs
			Expect(req.Tags[0].ID).NotTo(Equal(uint64(0)))
			Expect(req.SuiteRuns[0].Tags[0].ID).NotTo(Equal(uint64(0)))
			Expect(req.SuiteRuns[0].SpecRuns[0].Tags[0].ID).NotTo(Equal(uint64(0)))
			Expect(req.SuiteRuns[0].SpecRuns[1].Tags[0].ID).NotTo(Equal(uint64(0)))
			Expect(req.SuiteRuns[1].Tags[0].ID).NotTo(Equal(uint64(0)))
			Expect(req.SuiteRuns[1].SpecRuns[0].Tags[0].ID).NotTo(Equal(uint64(0)))
		})

		It("returns error when suite-level tag processing fails", func() {
			req := &TestRunRequest{
				SuiteRuns: []SuiteRun{
					{Tags: []Tag{{Name: "test"}}},
				},
			}
			repo.failFind = true
			err := ProcessTestRunTags(context.Background(), svc, req)
			Expect(err).ToNot(BeNil())
		})

		It("returns error when spec-level tag processing fails", func() {
			req := &TestRunRequest{
				SuiteRuns: []SuiteRun{
					{
						SpecRuns: []SpecRun{
							{Tags: []Tag{{Name: "test"}}},
						},
					},
				},
			}
			repo.failFind = true
			err := ProcessTestRunTags(context.Background(), svc, req)
			Expect(err).ToNot(BeNil())
		})
	})

	Describe("HTTP endpoints", func() {
		Describe("createTag", func() {
			It("returns 400 for invalid JSON", func() {
				w := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/tags", bytes.NewBufferString(`{"bad":`))
				router.ServeHTTP(w, req)
				Expect(w.Code).To(Equal(http.StatusBadRequest))
			})

			It("returns 400 for missing required name field", func() {
				payload := map[string]string{"description": "d", "color": "blue"}
				b, _ := json.Marshal(payload)
				w := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/tags", bytes.NewBuffer(b))
				req.Header.Set("Content-Type", "application/json")
				router.ServeHTTP(w, req)
				Expect(w.Code).To(Equal(http.StatusBadRequest))
			})

			It("creates a tag and returns 201", func() {
				payload := map[string]string{"name": "MyTag", "description": "d", "color": "blue"}
				b, _ := json.Marshal(payload)
				w := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/tags", bytes.NewBuffer(b))
				req.Header.Set("Content-Type", "application/json")
				router.ServeHTTP(w, req)
				Expect(w.Code).To(Equal(http.StatusCreated))
				var out map[string]interface{}
				Expect(json.Unmarshal(w.Body.Bytes(), &out)).To(Succeed())
				Expect(out["name"]).To(Equal("mytag")) // NewTag normalizes to lowercase trimmed
				Expect(out["description"]).To(Equal("d"))
			})

			It("returns 500 when service fails to create tag", func() {
				repo.failFind = true
				payload := map[string]string{"name": "FailTag"}
				b, _ := json.Marshal(payload)
				w := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/tags", bytes.NewBuffer(b))
				req.Header.Set("Content-Type", "application/json")
				router.ServeHTTP(w, req)
				Expect(w.Code).To(Equal(http.StatusInternalServerError))
			})
		})

		Describe("getTag and getTagByName", func() {
			It("returns 404 when not found by ID", func() {
				w := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodGet, "/api/v1/tags/9999", nil)
				router.ServeHTTP(w, req)
				Expect(w.Code).To(Equal(http.StatusNotFound))
			})

			It("returns 404 when not found by name", func() {
				w := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodGet, "/api/v1/tags/by-name/nonexistent-tag-name", nil)
				router.ServeHTTP(w, req)
				Expect(w.Code).To(Equal(http.StatusNotFound))
			})

			It("returns created tag via id and name", func() {
				// create a tag via service to get an ID
				tag, err := svc.CreateTag(context.Background(), "Sample")
				Expect(err).To(BeNil())
				Expect(tag).ToNot(BeNil())

				// GET by ID
				w1 := httptest.NewRecorder()
				req1 := httptest.NewRequest(http.MethodGet, "/api/v1/tags/"+string(tag.ID()), nil)
				router.ServeHTTP(w1, req1)
				Expect(w1.Code).To(Equal(http.StatusOK))
				var r1 map[string]interface{}
				Expect(json.Unmarshal(w1.Body.Bytes(), &r1)).To(Succeed())
				Expect(r1["name"]).To(Equal(tag.Name()))

				// GET by name
				w2 := httptest.NewRecorder()
				req2 := httptest.NewRequest(http.MethodGet, "/api/v1/tags/by-name/"+tag.Name(), nil)
				router.ServeHTTP(w2, req2)
				Expect(w2.Code).To(Equal(http.StatusOK))
			})
		})

		Describe("updateTag", func() {
			It("returns 400 for malformed json", func() {
				w := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/tags/1", bytes.NewBufferString("{"))
				router.ServeHTTP(w, req)
				Expect(w.Code).To(Equal(http.StatusBadRequest))
			})

			It("returns 404 when tag missing", func() {
				w := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/tags/33", bytes.NewBufferString(`{"name":"x"}`))
				req.Header.Set("Content-Type", "application/json")
				router.ServeHTTP(w, req)
				Expect(w.Code).To(Equal(http.StatusNotFound))
			})

			It("returns 200 with metadata for existing tag", func() {
				tag, _ := svc.CreateTag(context.Background(), "to-update")
				w := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/tags/"+string(tag.ID()), bytes.NewBufferString(`{"description":"d","color":"c"}`))
				req.Header.Set("Content-Type", "application/json")
				router.ServeHTTP(w, req)
				Expect(w.Code).To(Equal(http.StatusOK))
				var out map[string]interface{}
				Expect(json.Unmarshal(w.Body.Bytes(), &out)).To(Succeed())
				Expect(tagsDomain.TagID(out["id"].(string))).To(Equal(tag.ID()))
				Expect(out["description"]).To(Equal("d"))
			})
		})

		Describe("deleteTag", func() {
			It("deletes existing tag successfully", func() {
				tag, _ := svc.CreateTag(context.Background(), "todel")
				w := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/tags/"+string(tag.ID()), nil)
				router.ServeHTTP(w, req)
				Expect(w.Code).To(Equal(http.StatusOK))
			})

			It("returns 500 when delete fails", func() {
				// simulate missing tag -> Service.DeleteTag will return error
				w := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/tags/does-not-exist", nil)
				router.ServeHTTP(w, req)
				Expect(w.Code).To(Equal(http.StatusInternalServerError))
			})
		})

		Describe("listTags and popular", func() {
			It("lists tags (empty)", func() {
				w := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodGet, "/api/v1/tags", nil)
				router.ServeHTTP(w, req)
				Expect(w.Code).To(Equal(http.StatusOK))
				var arr []interface{}
				Expect(json.Unmarshal(w.Body.Bytes(), &arr)).To(Succeed())
			})

			It("lists tags and returns results", func() {
				_, _ = svc.CreateTag(context.Background(), "a")
				_, _ = svc.CreateTag(context.Background(), "b")
				w := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodGet, "/api/v1/tags", nil)
				router.ServeHTTP(w, req)
				Expect(w.Code).To(Equal(http.StatusOK))
				var arr []map[string]interface{}
				Expect(json.Unmarshal(w.Body.Bytes(), &arr)).To(Succeed())
				Expect(len(arr)).To(Equal(2))
			})

			It("returns 500 when listTags service fails", func() {
				repo.failFind = true
				w := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodGet, "/api/v1/tags", nil)
				router.ServeHTTP(w, req)
				Expect(w.Code).To(Equal(http.StatusInternalServerError))
			})

			It("get popular with limit parameter", func() {
				// ensure several tags exist
				for i := 0; i < 5; i++ {
					_, _ = svc.CreateTag(context.Background(), fmt.Sprintf("tag%d", i))
				}
				w := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodGet, "/api/v1/tags/popular?limit=2", nil)
				router.ServeHTTP(w, req)
				Expect(w.Code).To(Equal(http.StatusOK))
				var arr []map[string]interface{}
				Expect(json.Unmarshal(w.Body.Bytes(), &arr)).To(Succeed())
				Expect(len(arr)).To(Equal(2))
			})

			It("get popular with no limit uses default of 10", func() {
				// Create 15 tags
				for i := 0; i < 15; i++ {
					_, _ = svc.CreateTag(context.Background(), fmt.Sprintf("popular-%d", i))
				}
				w := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodGet, "/api/v1/tags/popular", nil)
				router.ServeHTTP(w, req)
				Expect(w.Code).To(Equal(http.StatusOK))
				var arr []map[string]interface{}
				Expect(json.Unmarshal(w.Body.Bytes(), &arr)).To(Succeed())
				Expect(len(arr)).To(Equal(10)) // Should be limited to default 10
			})

			It("get popular with limit larger than available tags", func() {
				_, _ = svc.CreateTag(context.Background(), "only-one")
				w := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodGet, "/api/v1/tags/popular?limit=100", nil)
				router.ServeHTTP(w, req)
				Expect(w.Code).To(Equal(http.StatusOK))
				var arr []map[string]interface{}
				Expect(json.Unmarshal(w.Body.Bytes(), &arr)).To(Succeed())
				Expect(len(arr)).To(Equal(1)) // Only 1 tag exists
			})

			It("returns bad request for invalid limit (<=0)", func() {
				w := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodGet, "/api/v1/tags/popular?limit=0", nil)
				router.ServeHTTP(w, req)
				Expect(w.Code).To(Equal(http.StatusBadRequest))
			})

			It("returns bad request for negative limit", func() {
				w := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodGet, "/api/v1/tags/popular?limit=-5", nil)
				router.ServeHTTP(w, req)
				Expect(w.Code).To(Equal(http.StatusBadRequest))
			})

			It("returns 400 for invalid (non-numeric) limit parameter", func() {
				// Invalid limit string (cannot parse to int) triggers the else if l <= 0 branch
				// because strconv.Atoi returns 0 on error, making l <= 0 true
				w := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodGet, "/api/v1/tags/popular?limit=invalid", nil)
				router.ServeHTTP(w, req)
				Expect(w.Code).To(Equal(http.StatusBadRequest))
			})

			It("returns 500 when popular tags service fails", func() {
				repo.failFind = true
				w := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodGet, "/api/v1/tags/popular", nil)
				router.ServeHTTP(w, req)
				Expect(w.Code).To(Equal(http.StatusInternalServerError))
			})
		})

		Describe("getTagUsageStats", func() {
			It("returns empty array (TODO stub)", func() {
				w := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodGet, "/api/v1/tags/usage-stats", nil)
				router.ServeHTTP(w, req)
				Expect(w.Code).To(Equal(http.StatusOK))
				var out []map[string]interface{}
				Expect(json.Unmarshal(w.Body.Bytes(), &out)).To(Succeed())
				Expect(len(out)).To(Equal(0))
			})
		})
	})
})
