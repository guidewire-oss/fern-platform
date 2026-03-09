package interfaces_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/gin-gonic/gin"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"

	"github.com/guidewire-oss/fern-platform/internal/domains/testing/application"
	"github.com/guidewire-oss/fern-platform/internal/domains/testing/domain"
	"github.com/guidewire-oss/fern-platform/internal/domains/testing/interfaces"
	"github.com/guidewire-oss/fern-platform/pkg/logging"
)

// --- Mock repositories for TestRunService ---

type adapterMockTestRunRepo struct{ mock.Mock }

func (m *adapterMockTestRunRepo) Create(ctx context.Context, testRun *domain.TestRun) error {
	args := m.Called(ctx, testRun)
	return args.Error(0)
}
func (m *adapterMockTestRunRepo) Update(ctx context.Context, testRun *domain.TestRun) error {
	args := m.Called(ctx, testRun)
	return args.Error(0)
}
func (m *adapterMockTestRunRepo) GetByID(ctx context.Context, id uint) (*domain.TestRun, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.TestRun), args.Error(1)
}
func (m *adapterMockTestRunRepo) GetByRunID(ctx context.Context, runID string) (*domain.TestRun, error) {
	args := m.Called(ctx, runID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.TestRun), args.Error(1)
}
func (m *adapterMockTestRunRepo) GetWithDetails(ctx context.Context, id uint) (*domain.TestRun, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.TestRun), args.Error(1)
}
func (m *adapterMockTestRunRepo) GetLatestByProjectID(ctx context.Context, projectID string, limit int) ([]*domain.TestRun, error) {
	args := m.Called(ctx, projectID, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.TestRun), args.Error(1)
}
func (m *adapterMockTestRunRepo) GetTestRunSummary(ctx context.Context, projectID string) (*domain.TestRunSummary, error) {
	args := m.Called(ctx, projectID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.TestRunSummary), args.Error(1)
}
func (m *adapterMockTestRunRepo) Delete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}
func (m *adapterMockTestRunRepo) CountByProjectID(ctx context.Context, projectID string) (int64, error) {
	return 0, nil
}
func (m *adapterMockTestRunRepo) Count(ctx context.Context) (int64, error) { return 0, nil }
func (m *adapterMockTestRunRepo) GetRecent(ctx context.Context, limit int) ([]*domain.TestRun, error) {
	return nil, nil
}
func (m *adapterMockTestRunRepo) List(ctx context.Context, limit, offset int) ([]*domain.TestRun, int64, error) {
	return nil, 0, nil
}

type adapterMockSuiteRunRepo struct{ mock.Mock }

func (m *adapterMockSuiteRunRepo) Create(ctx context.Context, suiteRun *domain.SuiteRun) error {
	args := m.Called(ctx, suiteRun)
	return args.Error(0)
}
func (m *adapterMockSuiteRunRepo) CreateBatch(ctx context.Context, suiteRuns []*domain.SuiteRun) error {
	return nil
}
func (m *adapterMockSuiteRunRepo) Update(ctx context.Context, suiteRun *domain.SuiteRun) error {
	return nil
}
func (m *adapterMockSuiteRunRepo) GetByID(ctx context.Context, id uint) (*domain.SuiteRun, error) {
	return nil, nil
}
func (m *adapterMockSuiteRunRepo) FindByTestRunID(ctx context.Context, testRunID uint) ([]*domain.SuiteRun, error) {
	args := m.Called(ctx, testRunID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.SuiteRun), args.Error(1)
}

type adapterMockSpecRunRepo struct{ mock.Mock }

func (m *adapterMockSpecRunRepo) Create(ctx context.Context, specRun *domain.SpecRun) error {
	return nil
}
func (m *adapterMockSpecRunRepo) CreateBatch(ctx context.Context, specRuns []*domain.SpecRun) error {
	return nil
}
func (m *adapterMockSpecRunRepo) Update(ctx context.Context, specRun *domain.SpecRun) error {
	return nil
}
func (m *adapterMockSpecRunRepo) GetByID(ctx context.Context, id uint) (*domain.SpecRun, error) {
	return nil, nil
}
func (m *adapterMockSpecRunRepo) FindBySuiteRunID(ctx context.Context, suiteRunID uint) ([]*domain.SpecRun, error) {
	return nil, nil
}

func newTestLogger() *logging.Logger {
	return logging.GetLogger()
}

var _ = Describe("TestServiceAdapter", func() {
	var (
		mockTestRunRepo  *adapterMockTestRunRepo
		mockSuiteRunRepo *adapterMockSuiteRunRepo
		mockSpecRunRepo  *adapterMockSpecRunRepo
		svc              *application.TestRunService
		adapter          *interfaces.TestServiceAdapter
		router           *gin.Engine
		logger           *logging.Logger
	)

	BeforeEach(func() {
		gin.SetMode(gin.TestMode)
		mockTestRunRepo = new(adapterMockTestRunRepo)
		mockSuiteRunRepo = new(adapterMockSuiteRunRepo)
		mockSpecRunRepo = new(adapterMockSpecRunRepo)
		svc = application.NewTestRunService(mockTestRunRepo, mockSuiteRunRepo, mockSpecRunRepo)
		logger = newTestLogger()
		adapter = interfaces.NewTestServiceAdapter(svc, logger)
		router = gin.New()
		api := router.Group("/api/v1")
		adapter.RegisterRoutes(api)
	})

	Describe("NewTestServiceAdapter", func() {
		It("should create an adapter", func() {
			Expect(adapter).ToNot(BeNil())
		})
	})

	Describe("CreateTestRun", func() {
		Context("with valid request", func() {
			It("should return 201", func() {
				mockTestRunRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.TestRun")).Return(nil)

				body := map[string]interface{}{
					"project_id": "proj-1",
					"name":       "Test Run 1",
					"branch":     "main",
				}
				jsonBody, _ := json.Marshal(body)

				req := httptest.NewRequest(http.MethodPost, "/api/v1/test-runs", bytes.NewReader(jsonBody))
				req.Header.Set("Content-Type", "application/json")
				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)

				Expect(w.Code).To(Equal(http.StatusCreated))
				var resp map[string]interface{}
				_ = json.Unmarshal(w.Body.Bytes(), &resp)
				Expect(resp["project_id"]).To(Equal("proj-1"))
			})
		})

		Context("with invalid JSON", func() {
			It("should return 400", func() {
				req := httptest.NewRequest(http.MethodPost, "/api/v1/test-runs", bytes.NewReader([]byte("invalid")))
				req.Header.Set("Content-Type", "application/json")
				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)

				Expect(w.Code).To(Equal(http.StatusBadRequest))
			})
		})

		Context("with missing required field", func() {
			It("should return 400 for missing project_id", func() {
				body := map[string]interface{}{
					"name": "Test Run 1",
				}
				jsonBody, _ := json.Marshal(body)

				req := httptest.NewRequest(http.MethodPost, "/api/v1/test-runs", bytes.NewReader(jsonBody))
				req.Header.Set("Content-Type", "application/json")
				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)

				Expect(w.Code).To(Equal(http.StatusBadRequest))
			})
		})

		Context("when service fails", func() {
			It("should return 500", func() {
				mockTestRunRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.TestRun")).
					Return(errors.New("db error"))

				body := map[string]interface{}{
					"project_id": "proj-1",
				}
				jsonBody, _ := json.Marshal(body)

				req := httptest.NewRequest(http.MethodPost, "/api/v1/test-runs", bytes.NewReader(jsonBody))
				req.Header.Set("Content-Type", "application/json")
				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)

				Expect(w.Code).To(Equal(http.StatusInternalServerError))
			})
		})
	})

	Describe("GetTestRun", func() {
		Context("with valid ID", func() {
			It("should return 200 with test run", func() {
				tr := &domain.TestRun{
					ID:        1,
					ProjectID: "proj-1",
					Status:    "completed",
				}
				mockTestRunRepo.On("GetByID", mock.Anything, uint(1)).Return(tr, nil)

				req := httptest.NewRequest(http.MethodGet, "/api/v1/test-runs/1", nil)
				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)

				Expect(w.Code).To(Equal(http.StatusOK))
			})
		})

		Context("with invalid ID", func() {
			It("should return 400", func() {
				req := httptest.NewRequest(http.MethodGet, "/api/v1/test-runs/invalid", nil)
				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)

				Expect(w.Code).To(Equal(http.StatusBadRequest))
			})
		})

		Context("when test run not found", func() {
			It("should return 404", func() {
				mockTestRunRepo.On("GetByID", mock.Anything, uint(999)).
					Return((*domain.TestRun)(nil), errors.New("test run not found"))

				req := httptest.NewRequest(http.MethodGet, "/api/v1/test-runs/999", nil)
				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)

				Expect(w.Code).To(Equal(http.StatusNotFound))
			})
		})
	})

	Describe("GetTestRunDetails", func() {
		Context("with valid ID", func() {
			It("should return 200", func() {
				tr := &domain.TestRun{
					ID:        1,
					ProjectID: "proj-1",
					Status:    "completed",
				}
				mockTestRunRepo.On("GetWithDetails", mock.Anything, uint(1)).Return(tr, nil)

				req := httptest.NewRequest(http.MethodGet, "/api/v1/test-runs/1/details", nil)
				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)

				Expect(w.Code).To(Equal(http.StatusOK))
			})
		})

		Context("with invalid ID", func() {
			It("should return 400", func() {
				req := httptest.NewRequest(http.MethodGet, "/api/v1/test-runs/abc/details", nil)
				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)

				Expect(w.Code).To(Equal(http.StatusBadRequest))
			})
		})

		Context("when not found", func() {
			It("should return 404", func() {
				mockTestRunRepo.On("GetWithDetails", mock.Anything, uint(999)).
					Return((*domain.TestRun)(nil), errors.New("test run not found"))

				req := httptest.NewRequest(http.MethodGet, "/api/v1/test-runs/999/details", nil)
				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)

				Expect(w.Code).To(Equal(http.StatusNotFound))
			})
		})
	})

	Describe("CompleteTestRun", func() {
		Context("with valid request", func() {
			It("should return 200", func() {
				tr := &domain.TestRun{
					ID:     1,
					Status: "running",
				}
				mockTestRunRepo.On("GetByID", mock.Anything, uint(1)).Return(tr, nil)
				mockSuiteRunRepo.On("FindByTestRunID", mock.Anything, uint(1)).Return([]*domain.SuiteRun{}, nil)
				mockTestRunRepo.On("Update", mock.Anything, tr).Return(nil)

				body := map[string]interface{}{
					"status": "completed",
				}
				jsonBody, _ := json.Marshal(body)

				req := httptest.NewRequest(http.MethodPut, "/api/v1/test-runs/1/complete", bytes.NewReader(jsonBody))
				req.Header.Set("Content-Type", "application/json")
				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)

				Expect(w.Code).To(Equal(http.StatusOK))
			})
		})

		Context("with invalid ID", func() {
			It("should return 400", func() {
				body := map[string]interface{}{
					"status": "completed",
				}
				jsonBody, _ := json.Marshal(body)

				req := httptest.NewRequest(http.MethodPut, "/api/v1/test-runs/bad/complete", bytes.NewReader(jsonBody))
				req.Header.Set("Content-Type", "application/json")
				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)

				Expect(w.Code).To(Equal(http.StatusBadRequest))
			})
		})

		Context("with invalid body", func() {
			It("should return 400", func() {
				req := httptest.NewRequest(http.MethodPut, "/api/v1/test-runs/1/complete", bytes.NewReader([]byte("{}")))
				req.Header.Set("Content-Type", "application/json")
				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)

				Expect(w.Code).To(Equal(http.StatusBadRequest))
			})
		})
	})

	Describe("GetProjectTestRuns", func() {
		Context("with valid project ID", func() {
			It("should return 200 with test runs", func() {
				testRuns := []*domain.TestRun{
					{ID: 1, ProjectID: "proj-1", Status: "completed"},
				}
				mockTestRunRepo.On("GetLatestByProjectID", mock.Anything, "proj-1", 50).Return(testRuns, nil)

				req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/proj-1/test-runs", nil)
				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)

				Expect(w.Code).To(Equal(http.StatusOK))
				var resp map[string]interface{}
				_ = json.Unmarshal(w.Body.Bytes(), &resp)
				Expect(resp["total"]).To(BeEquivalentTo(1))
			})
		})

		Context("with custom limit", func() {
			It("should use the provided limit", func() {
				mockTestRunRepo.On("GetLatestByProjectID", mock.Anything, "proj-1", 10).Return([]*domain.TestRun{}, nil)

				req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/proj-1/test-runs?limit=10", nil)
				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)

				Expect(w.Code).To(Equal(http.StatusOK))
			})
		})

		Context("when service fails", func() {
			It("should return 500", func() {
				mockTestRunRepo.On("GetLatestByProjectID", mock.Anything, "proj-err", 50).
					Return(([]*domain.TestRun)(nil), errors.New("db error"))

				req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/proj-err/test-runs", nil)
				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)

				Expect(w.Code).To(Equal(http.StatusInternalServerError))
			})
		})
	})

	Describe("GetProjectTestRunSummary", func() {
		Context("with valid project ID", func() {
			It("should return 200 with summary", func() {
				summary := &domain.TestRunSummary{
					TotalRuns:   10,
					PassedRuns:  8,
					FailedRuns:  2,
					SuccessRate: 0.8,
				}
				mockTestRunRepo.On("GetTestRunSummary", mock.Anything, "proj-1").Return(summary, nil)

				req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/proj-1/test-runs/summary", nil)
				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)

				Expect(w.Code).To(Equal(http.StatusOK))
			})
		})

		Context("when service fails", func() {
			It("should return 500", func() {
				mockTestRunRepo.On("GetTestRunSummary", mock.Anything, "proj-err").
					Return((*domain.TestRunSummary)(nil), errors.New("db error"))

				req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/proj-err/test-runs/summary", nil)
				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)

				Expect(w.Code).To(Equal(http.StatusInternalServerError))
			})
		})
	})

	Describe("ResolveTestRuns", func() {
		It("should delegate to service", func() {
			testRuns := []*domain.TestRun{
				{ID: 1, ProjectID: "proj-1"},
			}
			mockTestRunRepo.On("GetLatestByProjectID", mock.Anything, "proj-1", 10).Return(testRuns, nil)

			result, err := adapter.ResolveTestRuns(context.Background(), "proj-1", 10)

			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(HaveLen(1))
		})
	})

	Describe("ResolveTestRunDetails", func() {
		It("should delegate to service", func() {
			tr := &domain.TestRun{ID: 1, ProjectID: "proj-1"}
			mockTestRunRepo.On("GetWithDetails", mock.Anything, uint(1)).Return(tr, nil)

			result, err := adapter.ResolveTestRunDetails(context.Background(), 1)

			Expect(err).ToNot(HaveOccurred())
			Expect(result).ToNot(BeNil())
			Expect(result.ProjectID).To(Equal("proj-1"))
		})
	})

	Describe("RegisterRoutes", func() {
		It("should register all routes", func() {
			testRouter := gin.New()
			group := testRouter.Group("/api/v1")
			adapter.RegisterRoutes(group)

			routes := testRouter.Routes()
			routePaths := make(map[string]bool)
			for _, r := range routes {
				routePaths[r.Method+":"+r.Path] = true
			}

			Expect(routePaths).To(HaveKey("POST:/api/v1/test-runs"))
			Expect(routePaths).To(HaveKey("GET:/api/v1/test-runs/:id"))
			Expect(routePaths).To(HaveKey("GET:/api/v1/test-runs/:id/details"))
			Expect(routePaths).To(HaveKey("PUT:/api/v1/test-runs/:id/complete"))
			Expect(routePaths).To(HaveKey("POST:/api/v1/test-runs/with-suites"))
			Expect(routePaths).To(HaveKey("GET:/api/v1/projects/:projectId/test-runs"))
			Expect(routePaths).To(HaveKey("GET:/api/v1/projects/:projectId/test-runs/summary"))
		})
	})

	Describe("CreateTestRunWithSuites", func() {
		Context("with valid request", func() {
			It("should return 201", func() {
				mockTestRunRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.TestRun")).Return(nil)
				mockSuiteRunRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.SuiteRun")).Return(nil)
				mockSpecRunRepo.On("CreateBatch", mock.Anything, mock.Anything).Return(nil)
				mockTestRunRepo.On("GetByID", mock.Anything, mock.Anything).Return(&domain.TestRun{
					ID:     1,
					Status: "completed",
				}, nil)
				mockSuiteRunRepo.On("FindByTestRunID", mock.Anything, mock.Anything).Return([]*domain.SuiteRun{}, nil)
				mockTestRunRepo.On("Update", mock.Anything, mock.Anything).Return(nil)

				body := map[string]interface{}{
					"project_id": "proj-1",
					"name":       "Full Run",
					"branch":     "main",
					"suites": []map[string]interface{}{
						{
							"name":         "Suite 1",
							"package_name": "pkg1",
							"specs": []map[string]interface{}{
								{
									"name":     "Spec 1",
									"status":   "passed",
									"duration": 100,
								},
							},
						},
					},
				}
				jsonBody, _ := json.Marshal(body)

				req := httptest.NewRequest(http.MethodPost, "/api/v1/test-runs/with-suites", bytes.NewReader(jsonBody))
				req.Header.Set("Content-Type", "application/json")
				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)

				Expect(w.Code).To(Equal(http.StatusCreated))
			})
		})

		Context("with invalid JSON", func() {
			It("should return 400", func() {
				req := httptest.NewRequest(http.MethodPost, "/api/v1/test-runs/with-suites", bytes.NewReader([]byte("bad")))
				req.Header.Set("Content-Type", "application/json")
				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)

				Expect(w.Code).To(Equal(http.StatusBadRequest))
			})
		})
	})
})

// Ensure time import is used
var _ = time.Now
