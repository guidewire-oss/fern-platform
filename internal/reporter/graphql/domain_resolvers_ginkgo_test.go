package graphql

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"

	authDomain "github.com/guidewire-oss/fern-platform/internal/domains/auth/domain"
	projectsApp "github.com/guidewire-oss/fern-platform/internal/domains/projects/application"
	projectsDomain "github.com/guidewire-oss/fern-platform/internal/domains/projects/domain"
	tagsApp "github.com/guidewire-oss/fern-platform/internal/domains/tags/application"
	tagsDomain "github.com/guidewire-oss/fern-platform/internal/domains/tags/domain"
	testingApp "github.com/guidewire-oss/fern-platform/internal/domains/testing/application"
	testingDomain "github.com/guidewire-oss/fern-platform/internal/domains/testing/domain"
	"github.com/guidewire-oss/fern-platform/internal/reporter/graphql/model"
	"github.com/guidewire-oss/fern-platform/internal/testhelpers"
	"github.com/guidewire-oss/fern-platform/pkg/config"
	"github.com/guidewire-oss/fern-platform/pkg/logging"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var _ = Describe("DomainResolvers", func() {
	var (
		resolver  *Resolver
		logger    *logging.Logger
		db        *gorm.DB
		ctx       context.Context
	)

	BeforeEach(func() {
		var err error
		logger, err = logging.NewLogger(&config.LoggingConfig{
			Level:      "error",
			Format:     "json",
			Output:     "stdout",
			Structured: true,
		})
		Expect(err).NotTo(HaveOccurred())

		db, err = gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
		Expect(err).NotTo(HaveOccurred())

		ctx = context.Background()
	})

	Describe("GetTestRun_domain", func() {
		var (
			mockRepo       *testhelpers.MockTestRunRepository
			testingService *testingApp.TestRunService
		)

		BeforeEach(func() {
			mockRepo = new(testhelpers.MockTestRunRepository)
			testingService = testingApp.NewTestRunService(mockRepo, nil, nil)
			resolver = NewResolver(testingService, nil, nil, nil, nil, db, logger)
		})

		Context("with valid ID", func() {
			It("should return the test run", func() {
				now := time.Now()
				testRun := &testingDomain.TestRun{
					ID:        123,
					RunID:     "run-123",
					ProjectID: "proj-1",
					Status:    "completed",
					StartTime: now,
					Duration:  5 * time.Second,
					Tags:      []testingDomain.Tag{},
					SuiteRuns: []testingDomain.SuiteRun{},
				}

				mockRepo.On("GetByID", mock.Anything, uint(123)).Return(testRun, nil)

				result, err := resolver.Query().(*queryResolver).GetTestRun_domain(ctx, "123")

				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.ID).To(Equal("123"))
				Expect(result.RunID).To(Equal("run-123"))
				mockRepo.AssertExpectations(GinkgoT())
			})
		})

		Context("with invalid ID format", func() {
			It("should return an error", func() {
				result, err := resolver.Query().(*queryResolver).GetTestRun_domain(ctx, "invalid")

				Expect(err).To(HaveOccurred())
				Expect(result).To(BeNil())
			})
		})

		Context("when test run not found", func() {
			It("should return an error", func() {
				mockRepo.On("GetByID", mock.Anything, uint(999)).Return(nil, gorm.ErrRecordNotFound)

				result, err := resolver.Query().(*queryResolver).GetTestRun_domain(ctx, "999")

				Expect(err).To(HaveOccurred())
				Expect(result).To(BeNil())
				mockRepo.AssertExpectations(GinkgoT())
			})
		})
	})

	Describe("RecentTestRuns_domain", func() {
		var (
			mockRepo       *testhelpers.MockTestRunRepository
			testingService *testingApp.TestRunService
		)

		BeforeEach(func() {
			mockRepo = new(testhelpers.MockTestRunRepository)
			testingService = testingApp.NewTestRunService(mockRepo, nil, nil)
			resolver = NewResolver(testingService, nil, nil, nil, nil, db, logger)
		})

		Context("without project filter", func() {
			It("should return recent test runs", func() {
				now := time.Now()
				testRuns := []*testingDomain.TestRun{
					{
						ID:        1,
						RunID:     "run-1",
						ProjectID: "proj-1",
						Status:    "completed",
						StartTime: now,
						Tags:      []testingDomain.Tag{},
						SuiteRuns: []testingDomain.SuiteRun{},
					},
					{
						ID:        2,
						RunID:     "run-2",
						ProjectID: "proj-2",
						Status:    "running",
						StartTime: now,
						Tags:      []testingDomain.Tag{},
						SuiteRuns: []testingDomain.SuiteRun{},
					},
				}

				mockRepo.On("GetRecent", mock.Anything, 10).Return(testRuns, nil)

				result, err := resolver.Query().(*queryResolver).RecentTestRuns_domain(ctx, nil, nil)

				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(HaveLen(2))
				Expect(result[0].RunID).To(Equal("run-1"))
				Expect(result[1].RunID).To(Equal("run-2"))
				mockRepo.AssertExpectations(GinkgoT())
			})
		})

		Context("with project filter", func() {
			It("should return test runs for the project", func() {
				now := time.Now()
				projectID := "proj-1"
				testRuns := []*testingDomain.TestRun{
					{
						ID:        1,
						RunID:     "run-1",
						ProjectID: projectID,
						Status:    "completed",
						StartTime: now,
						Tags:      []testingDomain.Tag{},
						SuiteRuns: []testingDomain.SuiteRun{},
					},
				}

				mockRepo.On("GetLatestByProjectID", mock.Anything, projectID, 10).Return(testRuns, nil)

				result, err := resolver.Query().(*queryResolver).RecentTestRuns_domain(ctx, &projectID, nil)

				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(HaveLen(1))
				Expect(result[0].ProjectID).To(Equal(projectID))
				mockRepo.AssertExpectations(GinkgoT())
			})
		})

		Context("with custom limit", func() {
			It("should respect the limit", func() {
				limit := 5
				mockRepo.On("GetRecent", mock.Anything, limit).Return([]*testingDomain.TestRun{}, nil)

				result, err := resolver.Query().(*queryResolver).RecentTestRuns_domain(ctx, nil, &limit)

				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				mockRepo.AssertExpectations(GinkgoT())
			})
		})
	})

	Describe("GetProject_domain", func() {
		var (
			mockRepo        *testhelpers.MockProjectRepository
			mockPermRepo    *testhelpers.MockProjectPermissionRepository
			projectService  *projectsApp.ProjectService
		)

		BeforeEach(func() {
			mockRepo = new(testhelpers.MockProjectRepository)
			mockPermRepo = new(testhelpers.MockProjectPermissionRepository)
			projectService = projectsApp.NewProjectService(mockRepo, mockPermRepo)
			resolver = NewResolver(nil, projectService, nil, nil, nil, db, logger)
		})

		Context("with existing project", func() {
			It("should return the project", func() {
				projectID := "test-proj-1"
				project, _ := projectsDomain.NewProject(
					projectsDomain.ProjectID(projectID),
					"Test Project",
					projectsDomain.Team("test-team"),
				)

				mockRepo.On("FindByProjectID", mock.Anything, projectsDomain.ProjectID(projectID)).Return(project, nil)

				result, err := resolver.Query().(*queryResolver).GetProject_domain(ctx, projectID)

				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.ProjectID).To(Equal(projectID))
				Expect(result.Name).To(Equal("Test Project"))
				mockRepo.AssertExpectations(GinkgoT())
			})
		})

		Context("when project not found", func() {
			It("should return an error", func() {
				projectID := "nonexistent"
				mockRepo.On("FindByProjectID", mock.Anything, projectsDomain.ProjectID(projectID)).Return(nil, gorm.ErrRecordNotFound)

				result, err := resolver.Query().(*queryResolver).GetProject_domain(ctx, projectID)

				Expect(err).To(HaveOccurred())
				Expect(result).To(BeNil())
				mockRepo.AssertExpectations(GinkgoT())
			})
		})
	})

	Describe("ListProjects_domain", func() {
		var (
			mockRepo       *testhelpers.MockProjectRepository
			mockPermRepo   *testhelpers.MockProjectPermissionRepository
			projectService *projectsApp.ProjectService
		)

		BeforeEach(func() {
			mockRepo = new(testhelpers.MockProjectRepository)
			mockPermRepo = new(testhelpers.MockProjectPermissionRepository)
			projectService = projectsApp.NewProjectService(mockRepo, mockPermRepo)
			resolver = NewResolver(nil, projectService, nil, nil, nil, db, logger)
		})

		Context("with default pagination", func() {
			It("should return projects", func() {
				proj1, _ := projectsDomain.NewProject(projectsDomain.ProjectID("proj-1"), "Project 1", projectsDomain.Team("team1"))
				proj2, _ := projectsDomain.NewProject(projectsDomain.ProjectID("proj-2"), "Project 2", projectsDomain.Team("team2"))
				projects := []*projectsDomain.Project{proj1, proj2}

				mockRepo.On("FindAll", mock.Anything, 50, 0).Return(projects, int64(2), nil)

				result, err := resolver.Query().(*queryResolver).ListProjects_domain(ctx, nil, nil, nil)

				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(HaveLen(2))
				Expect(result[0].ProjectID).To(Equal("proj-1"))
				Expect(result[1].ProjectID).To(Equal("proj-2"))
				mockRepo.AssertExpectations(GinkgoT())
			})
		})

		Context("with custom pagination", func() {
			It("should respect limit and offset", func() {
				limit := 10
				offset := 20
				proj3, _ := projectsDomain.NewProject(projectsDomain.ProjectID("proj-3"), "Project 3", projectsDomain.Team("team1"))
				projects := []*projectsDomain.Project{proj3}

				mockRepo.On("FindAll", mock.Anything, limit, offset).Return(projects, int64(1), nil)

				result, err := resolver.Query().(*queryResolver).ListProjects_domain(ctx, &limit, &offset, nil)

				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(HaveLen(1))
				mockRepo.AssertExpectations(GinkgoT())
			})
		})
	})

	Describe("ListTags_domain", func() {
		var (
			mockRepo   *testhelpers.MockTagRepository
			tagService *tagsApp.TagService
		)

		BeforeEach(func() {
			mockRepo = new(testhelpers.MockTagRepository)
			tagService = tagsApp.NewTagService(mockRepo)
			resolver = NewResolver(nil, nil, tagService, nil, nil, db, logger)
		})

		Context("when tags exist", func() {
			It("should return all tags", func() {
				tag1, _ := tagsDomain.NewTag("Tag 1")
				tag2, _ := tagsDomain.NewTag("Tag 2")
				tags := []*tagsDomain.Tag{tag1, tag2}

				mockRepo.On("FindAll", mock.Anything).Return(tags, nil)

				result, err := resolver.Query().(*queryResolver).ListTags_domain(ctx)

				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(HaveLen(2))
				mockRepo.AssertExpectations(GinkgoT())
			})
		})

		Context("when service fails", func() {
			It("should return an error", func() {
				mockRepo.On("FindAll", mock.Anything).Return(nil, gorm.ErrInvalidDB)

				result, err := resolver.Query().(*queryResolver).ListTags_domain(ctx)

				Expect(err).To(HaveOccurred())
				Expect(result).To(BeNil())
				mockRepo.AssertExpectations(GinkgoT())
			})
		})
	})

	Describe("ProjectByProjectID_domain", func() {
		var (
			mockRepo       *testhelpers.MockProjectRepository
			mockPermRepo   *testhelpers.MockProjectPermissionRepository
			projectService *projectsApp.ProjectService
		)

		BeforeEach(func() {
			mockRepo = new(testhelpers.MockProjectRepository)
			mockPermRepo = new(testhelpers.MockProjectPermissionRepository)
			projectService = projectsApp.NewProjectService(mockRepo, mockPermRepo)
			resolver = NewResolver(nil, projectService, nil, nil, nil, db, logger)
		})

		Context("with valid project ID", func() {
			It("should return the project", func() {
				projectID := "test-proj-1"
				project, _ := projectsDomain.NewProject(
					projectsDomain.ProjectID(projectID),
					"Test Project",
					projectsDomain.Team("test-team"),
				)

				mockRepo.On("FindByProjectID", mock.Anything, projectsDomain.ProjectID(projectID)).Return(project, nil)

				result, err := resolver.Query().(*queryResolver).ProjectByProjectID_domain(ctx, projectID)

				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.ProjectID).To(Equal(projectID))
				mockRepo.AssertExpectations(GinkgoT())
			})
		})
	})

	Describe("Project_domain", func() {
		var (
			mockRepo       *testhelpers.MockProjectRepository
			mockPermRepo   *testhelpers.MockProjectPermissionRepository
			projectService *projectsApp.ProjectService
		)

		BeforeEach(func() {
			mockRepo = new(testhelpers.MockProjectRepository)
			mockPermRepo = new(testhelpers.MockProjectPermissionRepository)
			projectService = projectsApp.NewProjectService(mockRepo, mockPermRepo)
			resolver = NewResolver(nil, projectService, nil, nil, nil, db, logger)
		})

		Context("finding by database ID", func() {
			It("should search through all projects", func() {
				proj1, _ := projectsDomain.NewProject(projectsDomain.ProjectID("proj-1"), "Project 1", projectsDomain.Team("team1"))
				projects := []*projectsDomain.Project{proj1}

				mockRepo.On("FindAll", mock.Anything, 1000, 0).Return(projects, int64(1), nil)

				result, err := resolver.Query().(*queryResolver).Project_domain(ctx, "999")

				// Will return error since ID won't match
				Expect(err).To(HaveOccurred())
				Expect(result).To(BeNil())
				mockRepo.AssertExpectations(GinkgoT())
			})
		})

		Context("with invalid ID format", func() {
			It("should return an error", func() {
				mockRepo.On("FindAll", mock.Anything, 1000, 0).Return([]*projectsDomain.Project{}, int64(0), nil)

				result, err := resolver.Query().(*queryResolver).Project_domain(ctx, "invalid")

				Expect(err).To(HaveOccurred())
				Expect(result).To(BeNil())
			})
		})
	})

	Describe("CreateTag_domain", func() {
		var (
			mockRepo   *testhelpers.MockTagRepository
			tagService *tagsApp.TagService
		)

		BeforeEach(func() {
			mockRepo = new(testhelpers.MockTagRepository)
			tagService = tagsApp.NewTagService(mockRepo)
			resolver = NewResolver(nil, nil, tagService, nil, nil, db, logger)
		})

		Context("with valid input", func() {
			It("should create a tag", func() {
				input := model.CreateTagInput{
					Name: "test-tag",
				}

				tag, _ := tagsDomain.NewTag("test-tag")
				mockRepo.On("FindByName", mock.Anything, "test-tag").Return(tag, nil)

				result, err := resolver.Mutation().(*mutationResolver).CreateTag_domain(ctx, input)

				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				mockRepo.AssertExpectations(GinkgoT())
			})
		})

		Context("with empty name", func() {
			It("should return an error", func() {
				input := model.CreateTagInput{
					Name: "",
				}

				// Mock will be called with empty string which will fail validation
				mockRepo.On("FindByName", mock.Anything, "").Return(nil, gorm.ErrRecordNotFound)

				result, err := resolver.Mutation().(*mutationResolver).CreateTag_domain(ctx, input)

				Expect(err).To(HaveOccurred())
				Expect(result).To(BeNil())
			})
		})

		Context("when tag already exists", func() {
			It("should return the existing tag", func() {
				input := model.CreateTagInput{
					Name: "existing-tag",
				}

				existingTag, _ := tagsDomain.NewTag("existing-tag")
				mockRepo.On("FindByName", mock.Anything, "existing-tag").Return(existingTag, nil)

				result, err := resolver.Mutation().(*mutationResolver).CreateTag_domain(ctx, input)

				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				mockRepo.AssertExpectations(GinkgoT())
			})
		})
	})

	Describe("CreateProject_domain", func() {
		var (
			mockRepo       *testhelpers.MockProjectRepository
			mockPermRepo   *testhelpers.MockProjectPermissionRepository
			projectService *projectsApp.ProjectService
			authCtx        context.Context
		)

		BeforeEach(func() {
			mockRepo = new(testhelpers.MockProjectRepository)
			mockPermRepo = new(testhelpers.MockProjectPermissionRepository)
			projectService = projectsApp.NewProjectService(mockRepo, mockPermRepo)
			resolver = NewResolver(nil, projectService, nil, nil, nil, db, logger)

			// Create authenticated context with admin user
			user := &authDomain.User{
				UserID: "admin-user",
				Email:  "admin@example.com",
				Role:   authDomain.RoleAdmin,
				Groups: []authDomain.UserGroup{},
			}
			authCtx = context.WithValue(ctx, "user", user)
		})

		Context("as admin user", func() {
			It("should create a project", func() {
				input := model.CreateProjectInput{
					Name:      "New Project",
					ProjectID: "new-proj-1",
				}

				mockRepo.On("ExistsByProjectID", mock.Anything, projectsDomain.ProjectID("new-proj-1")).Return(false, nil)
				mockRepo.On("Save", mock.Anything, mock.AnythingOfType("*domain.Project")).Return(nil)
				mockPermRepo.On("Save", mock.Anything, mock.AnythingOfType("*domain.ProjectPermission")).Return(nil)

				result, err := resolver.Mutation().(*mutationResolver).CreateProject_domain(authCtx, input)

				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.Name).To(Equal("New Project"))
				mockRepo.AssertExpectations(GinkgoT())
			})
		})

		Context("as manager user", func() {
			It("should create a project", func() {
				user := &authDomain.User{
					UserID: "manager-user",
					Email:  "manager@example.com",
					Role:   authDomain.RoleManager,
					Groups: []authDomain.UserGroup{},
				}
				managerCtx := context.WithValue(ctx, "user", user)

				input := model.CreateProjectInput{
					Name:      "Manager Project",
					ProjectID: "manager-proj",
				}

				mockRepo.On("ExistsByProjectID", mock.Anything, projectsDomain.ProjectID("manager-proj")).Return(false, nil)
				mockRepo.On("Save", mock.Anything, mock.AnythingOfType("*domain.Project")).Return(nil)
				mockPermRepo.On("Save", mock.Anything, mock.AnythingOfType("*domain.ProjectPermission")).Return(nil)

				result, err := resolver.Mutation().(*mutationResolver).CreateProject_domain(managerCtx, input)

				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				mockRepo.AssertExpectations(GinkgoT())
			})
		})

		Context("as regular user", func() {
			It("should return permission error", func() {
				user := &authDomain.User{
					UserID: "regular-user",
					Email:  "user@example.com",
					Role:   authDomain.RoleUser,
					Groups: []authDomain.UserGroup{},
				}
				userCtx := context.WithValue(ctx, "user", user)

				input := model.CreateProjectInput{
					Name:      "User Project",
					ProjectID: "user-proj",
				}

				result, err := resolver.Mutation().(*mutationResolver).CreateProject_domain(userCtx, input)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("insufficient permissions"))
				Expect(result).To(BeNil())
			})
		})

		Context("without authentication", func() {
			It("should return authentication error", func() {
				input := model.CreateProjectInput{
					Name:      "Unauth Project",
					ProjectID: "unauth-proj",
				}

				result, err := resolver.Mutation().(*mutationResolver).CreateProject_domain(ctx, input)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to get current user"))
				Expect(result).To(BeNil())
			})
		})
	})

	Describe("UpdateProject_domain", func() {
		var (
			mockRepo       *testhelpers.MockProjectRepository
			mockPermRepo   *testhelpers.MockProjectPermissionRepository
			projectService *projectsApp.ProjectService
			authCtx        context.Context
		)

		BeforeEach(func() {
			mockRepo = new(testhelpers.MockProjectRepository)
			mockPermRepo = new(testhelpers.MockProjectPermissionRepository)
			projectService = projectsApp.NewProjectService(mockRepo, mockPermRepo)
			resolver = NewResolver(nil, projectService, nil, nil, nil, db, logger)

			user := &authDomain.User{
				UserID: "admin-user",
				Email:  "admin@example.com",
				Role:   authDomain.RoleAdmin,
				Groups: []authDomain.UserGroup{},
			}
			authCtx = context.WithValue(ctx, "user", user)
		})

		Context("as admin user", func() {
			It("should update the project", func() {
				projectID := "proj-1"
				project, _ := projectsDomain.NewProject(
					projectsDomain.ProjectID(projectID),
					"Original Name",
					projectsDomain.Team("team1"),
				)
				project.SetID(1) // Set the database ID to match the input "1"

				newName := "Updated Name"
				input := model.UpdateProjectInput{
					Name: &newName,
				}

				// UpdateProject_domain first tries to parse ID as uint and calls ListProjects
				mockRepo.On("FindAll", mock.Anything, 1000, 0).Return([]*projectsDomain.Project{project}, int64(1), nil)
				mockRepo.On("FindByProjectID", mock.Anything, projectsDomain.ProjectID(projectID)).Return(project, nil)
				mockRepo.On("Update", mock.Anything, mock.AnythingOfType("*domain.Project")).Return(nil)

				result, err := resolver.Mutation().(*mutationResolver).UpdateProject_domain(authCtx, "1", input)

				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				mockRepo.AssertExpectations(GinkgoT())
			})
		})

		Context("with invalid project ID", func() {
			It("should return an error", func() {
				input := model.UpdateProjectInput{}

				// "invalid" is not a uint, so it will be treated as ProjectID and call GetProject directly
				mockRepo.On("FindByProjectID", mock.Anything, projectsDomain.ProjectID("invalid")).Return(nil, gorm.ErrRecordNotFound)

				result, err := resolver.Mutation().(*mutationResolver).UpdateProject_domain(authCtx, "invalid", input)

				Expect(err).To(HaveOccurred())
				Expect(result).To(BeNil())
			})
		})
	})

	Describe("DeleteProject_domain", func() {
		var (
			mockRepo       *testhelpers.MockProjectRepository
			mockPermRepo   *testhelpers.MockProjectPermissionRepository
			projectService *projectsApp.ProjectService
			authCtx        context.Context
		)

		BeforeEach(func() {
			mockRepo = new(testhelpers.MockProjectRepository)
			mockPermRepo = new(testhelpers.MockProjectPermissionRepository)
			projectService = projectsApp.NewProjectService(mockRepo, mockPermRepo)
			resolver = NewResolver(nil, projectService, nil, nil, nil, db, logger)

			user := &authDomain.User{
				UserID: "admin-user",
				Email:  "admin@example.com",
				Role:   authDomain.RoleAdmin,
				Groups: []authDomain.UserGroup{},
			}
			authCtx = context.WithValue(ctx, "user", user)
		})

		Context("as admin user", func() {
			It("should delete the project", func() {
				projectID := "proj-1"
				project, _ := projectsDomain.NewProject(
					projectsDomain.ProjectID(projectID),
					"Project to Delete",
					projectsDomain.Team("team1"),
				)
				project.SetID(1) // Set the database ID to match the input "1"

				// DeleteProject_domain tries to parse ID as uint and calls ListProjects
				mockRepo.On("FindAll", mock.Anything, 1000, 0).Return([]*projectsDomain.Project{project}, int64(1), nil)
				mockRepo.On("FindByProjectID", mock.Anything, projectsDomain.ProjectID(projectID)).Return(project, nil)
				mockRepo.On("Delete", mock.Anything, uint(1)).Return(nil)

				result, err := resolver.Mutation().(*mutationResolver).DeleteProject_domain(authCtx, "1")

				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(BeTrue())
				mockRepo.AssertExpectations(GinkgoT())
			})
		})

		Context("with invalid project ID", func() {
			It("should return an error", func() {
				// "invalid" is not a uint, so it will be treated as ProjectID and call GetProject directly
				mockRepo.On("FindByProjectID", mock.Anything, projectsDomain.ProjectID("invalid")).Return(nil, gorm.ErrRecordNotFound)

				result, err := resolver.Mutation().(*mutationResolver).DeleteProject_domain(authCtx, "invalid")

				Expect(err).To(HaveOccurred())
				Expect(result).To(BeFalse())
			})
		})

		Context("as regular user", func() {
			It("should return permission error", func() {
				user := &authDomain.User{
					UserID: "regular-user",
					Email:  "user@example.com",
					Role:   authDomain.RoleUser,
					Groups: []authDomain.UserGroup{},
				}
				userCtx := context.WithValue(ctx, "user", user)

				projectID := "proj-1"
				project, _ := projectsDomain.NewProject(
					projectsDomain.ProjectID(projectID),
					"Project to Delete",
					projectsDomain.Team("team1"),
				)
				project.SetID(1) // Set the database ID to match the input "1"

				// DeleteProject_domain tries to parse ID as uint and calls ListProjects
				mockRepo.On("FindAll", mock.Anything, 1000, 0).Return([]*projectsDomain.Project{project}, int64(1), nil)
				mockRepo.On("FindByProjectID", mock.Anything, projectsDomain.ProjectID(projectID)).Return(project, nil)

				result, err := resolver.Mutation().(*mutationResolver).DeleteProject_domain(userCtx, "1")

				Expect(err).To(HaveOccurred())
				Expect(result).To(BeFalse())
			})
		})
	})

	Describe("Projects_domain", func() {
		var (
			mockRepo       *testhelpers.MockProjectRepository
			mockPermRepo   *testhelpers.MockProjectPermissionRepository
			projectService *projectsApp.ProjectService
			authCtx        context.Context
		)

		BeforeEach(func() {
			mockRepo = new(testhelpers.MockProjectRepository)
			mockPermRepo = new(testhelpers.MockProjectPermissionRepository)
			projectService = projectsApp.NewProjectService(mockRepo, mockPermRepo)
			resolver = NewResolver(nil, projectService, nil, nil, nil, db, logger)

			user := &authDomain.User{
				UserID: "test-user",
				Email:  "test@example.com",
				Role:   authDomain.RoleUser,
				Groups: []authDomain.UserGroup{{GroupName: "team1"}},
			}
			authCtx = context.WithValue(ctx, "user", user)
		})

		Context("with valid pagination", func() {
			It("should return projects connection", func() {
				proj1, _ := projectsDomain.NewProject(projectsDomain.ProjectID("proj-1"), "Project 1", projectsDomain.Team("team1"))
				proj2, _ := projectsDomain.NewProject(projectsDomain.ProjectID("proj-2"), "Project 2", projectsDomain.Team("team1"))
				projects := []*projectsDomain.Project{proj1, proj2}

				first := 20
				mockRepo.On("FindAll", mock.Anything, first, 0).Return(projects, int64(2), nil)

				result, err := resolver.Query().(*queryResolver).Projects_domain(authCtx, nil, &first, nil)

				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.Edges).To(HaveLen(2))
				mockRepo.AssertExpectations(GinkgoT())
			})
		})

		Context("without authentication", func() {
			It("should return authentication error", func() {
				result, err := resolver.Query().(*queryResolver).Projects_domain(ctx, nil, nil, nil)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("not authenticated"))
				Expect(result).To(BeNil())
			})
		})
	})

	Describe("DashboardSummary_domain", func() {
		var (
			mockTestRunRepo *testhelpers.MockTestRunRepository
			mockProjectRepo *testhelpers.MockProjectRepository
			mockPermRepo    *testhelpers.MockProjectPermissionRepository
			testingService  *testingApp.TestRunService
			projectService  *projectsApp.ProjectService
		)

		BeforeEach(func() {
			mockTestRunRepo = new(testhelpers.MockTestRunRepository)
			mockProjectRepo = new(testhelpers.MockProjectRepository)
			mockPermRepo = new(testhelpers.MockProjectPermissionRepository)
			testingService = testingApp.NewTestRunService(mockTestRunRepo, nil, nil)
			projectService = projectsApp.NewProjectService(mockProjectRepo, mockPermRepo)
			resolver = NewResolver(testingService, projectService, nil, nil, nil, db, logger)
		})

		Context("with projects and test runs", func() {
			It("should return dashboard summary", func() {
				projects := []*projectsDomain.Project{}
				proj1, _ := projectsDomain.NewProject(projectsDomain.ProjectID("proj-1"), "Project 1", projectsDomain.Team("team1"))
				projects = append(projects, proj1)

				mockProjectRepo.On("FindAll", mock.Anything, 1000, 0).Return(projects, int64(1), nil)
				mockTestRunRepo.On("GetRecent", mock.Anything, 100).Return([]*testingDomain.TestRun{}, nil)
				mockTestRunRepo.On("GetLatestByProjectID", mock.Anything, "proj-1", 1).Return([]*testingDomain.TestRun{}, nil)

				result, err := resolver.Query().(*queryResolver).DashboardSummary_domain(ctx)

				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				mockProjectRepo.AssertExpectations(GinkgoT())
			})
		})
	})

	Describe("TestRuns_domain", func() {
		var (
			mockRepo       *testhelpers.MockTestRunRepository
			testingService *testingApp.TestRunService
			authCtx        context.Context
		)

		BeforeEach(func() {
			mockRepo = new(testhelpers.MockTestRunRepository)
			testingService = testingApp.NewTestRunService(mockRepo, nil, nil)
			resolver = NewResolver(testingService, nil, nil, nil, nil, db, logger)

			user := &authDomain.User{
				UserID: "test-user",
				Email:  "test@example.com",
				Role:   authDomain.RoleUser,
				Groups: []authDomain.UserGroup{},
			}
			authCtx = context.WithValue(ctx, "user", user)
		})

		Context("with project filter", func() {
			It("should return test runs", func() {
				projectID := "proj-1"
				filter := &model.TestRunFilter{
					ProjectID: &projectID,
				}
				first := 20

				testRuns := []*testingDomain.TestRun{
					{
						ID:        1,
						RunID:     "run-1",
						ProjectID: projectID,
						Status:    "completed",
						StartTime: time.Now(),
						Tags:      []testingDomain.Tag{},
						SuiteRuns: []testingDomain.SuiteRun{},
					},
				}

				mockRepo.On("GetLatestByProjectID", mock.Anything, projectID, first).Return(testRuns, nil)
				mockRepo.On("CountByProjectID", mock.Anything, projectID).Return(int64(1), nil)

				result, err := resolver.Query().(*queryResolver).TestRuns_domain(authCtx, filter, &first, nil, nil, nil)

				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.Edges).To(HaveLen(1))
				mockRepo.AssertExpectations(GinkgoT())
			})
		})

		Context("without project filter", func() {
			It("should return empty results when no test runs exist", func() {
				mockRepo.On("FindAll", mock.Anything, 20, 0).Return([]*testingDomain.TestRun{}, int64(0), nil)

				result, err := resolver.Query().(*queryResolver).TestRuns_domain(ctx, nil, nil, nil, nil, nil)

				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.Edges).To(HaveLen(0))
			})
		})
	})

	Describe("Tags_domain", func() {
		var (
			mockRepo   *testhelpers.MockTagRepository
			tagService *tagsApp.TagService
			authCtx    context.Context
		)

		BeforeEach(func() {
			mockRepo = new(testhelpers.MockTagRepository)
			tagService = tagsApp.NewTagService(mockRepo)
			resolver = NewResolver(nil, nil, tagService, nil, nil, db, logger)

			user := &authDomain.User{
				UserID: "test-user",
				Email:  "test@example.com",
				Role:   authDomain.RoleUser,
				Groups: []authDomain.UserGroup{},
			}
			authCtx = context.WithValue(ctx, "user", user)
		})

		Context("with pagination", func() {
			It("should return tags connection", func() {
				tag1, _ := tagsDomain.NewTag("tag1")
				tag2, _ := tagsDomain.NewTag("tag2")
				tags := []*tagsDomain.Tag{tag1, tag2}

				first := 20
				mockRepo.On("FindAll", mock.Anything).Return(tags, nil)

				result, err := resolver.Query().(*queryResolver).Tags_domain(authCtx, nil, &first, nil)

				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.Edges).To(HaveLen(2))
				mockRepo.AssertExpectations(GinkgoT())
			})
		})

		Context("with filter", func() {
			It("should return filtered tags", func() {
				tag1, _ := tagsDomain.NewTag("environment:production")
				tags := []*tagsDomain.Tag{tag1}

				first := 20
				search := "environment"
				filter := &model.TagFilter{Search: &search}

				mockRepo.On("FindAll", mock.Anything).Return(tags, nil)

				result, err := resolver.Query().(*queryResolver).Tags_domain(ctx, filter, &first, nil)

				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
			})
		})
	})

	Describe("TreemapData_domain", func() {
		var (
			mockRepo        *testhelpers.MockTestRunRepository
			mockProjectRepo *testhelpers.MockProjectRepository
			mockPermRepo    *testhelpers.MockProjectPermissionRepository
			testingService  *testingApp.TestRunService
			projectService  *projectsApp.ProjectService
		)

		BeforeEach(func() {
			mockRepo = new(testhelpers.MockTestRunRepository)
			mockProjectRepo = new(testhelpers.MockProjectRepository)
			mockPermRepo = new(testhelpers.MockProjectPermissionRepository)
			testingService = testingApp.NewTestRunService(mockRepo, nil, nil)
			projectService = projectsApp.NewProjectService(mockProjectRepo, mockPermRepo)
			resolver = NewResolver(testingService, projectService, nil, nil, nil, db, logger)
		})

		Context("with valid project ID", func() {
			It("should return treemap data", func() {
				projectID := "proj-1"
				days := 30

				project, _ := projectsDomain.NewProject(projectsDomain.ProjectID(projectID), "Project 1", projectsDomain.Team("team1"))

				testRuns := []*testingDomain.TestRun{
					{
						ID:        1,
						RunID:     "run-1",
						ProjectID: projectID,
						Status:    "completed",
						StartTime: time.Now(),
						SuiteRuns: []testingDomain.SuiteRun{
							{
								ID:        1,
								Name: "Suite 1",
								SpecRuns: []*testingDomain.SpecRun{
									{
										ID:       1,
										Name: "Spec 1",
										Status:   "passed",
										Duration: 100 * time.Millisecond,
									},
								},
							},
						},
					},
				}

				mockProjectRepo.On("FindAll", mock.Anything, 1000, 0).Return([]*projectsDomain.Project{project}, int64(1), nil)
				mockRepo.On("GetLatestByProjectID", mock.Anything, projectID, 1000).Return(testRuns, nil)
				mockRepo.On("CountByProjectID", mock.Anything, projectID).Return(int64(1), nil)
				mockRepo.On("GetWithDetails", mock.Anything, uint(1)).Return(testRuns[0], nil)

				result, err := resolver.Query().(*queryResolver).TreemapData_domain(ctx, &projectID, &days)

				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
			})
		})

		Context("without project ID", func() {
			It("should return treemap data for all projects", func() {
				mockProjectRepo.On("FindAll", mock.Anything, 1000, 0).Return([]*projectsDomain.Project{}, int64(0), nil)

				result, err := resolver.Query().(*queryResolver).TreemapData_domain(ctx, nil, nil)

				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
			})
		})
	})
})
