package graphql

import (
"context"
"strconv"
"testing"
"time"

. "github.com/onsi/ginkgo/v2"
. "github.com/onsi/gomega"
"gorm.io/driver/sqlite"
"gorm.io/gorm"
gormlogger "gorm.io/gorm/logger"

authDomain "github.com/guidewire-oss/fern-platform/internal/domains/auth/domain"
projectsApp "github.com/guidewire-oss/fern-platform/internal/domains/projects/application"
projectsDomain "github.com/guidewire-oss/fern-platform/internal/domains/projects/domain"
projectsInfra "github.com/guidewire-oss/fern-platform/internal/domains/projects/infrastructure"
tagsApp "github.com/guidewire-oss/fern-platform/internal/domains/tags/application"
tagsDomain "github.com/guidewire-oss/fern-platform/internal/domains/tags/domain"
tagsInfra "github.com/guidewire-oss/fern-platform/internal/domains/tags/infrastructure"
testingApp "github.com/guidewire-oss/fern-platform/internal/domains/testing/application"
testingInfra "github.com/guidewire-oss/fern-platform/internal/domains/testing/infrastructure"
"github.com/guidewire-oss/fern-platform/internal/reporter/graphql/model"
"github.com/guidewire-oss/fern-platform/pkg/config"
"github.com/guidewire-oss/fern-platform/pkg/database"
"github.com/guidewire-oss/fern-platform/pkg/logging"
)

func TestDomainResolversIntegration(t *testing.T) {
RegisterFailHandler(Fail)
RunSpecs(t, "GraphQL Resolvers Test Suite")
}

var _ = Describe("Domain Resolvers - Integration Tests", func() {
var (
resolver       *Resolver
qr             *queryResolver
mr             *mutationResolver
logger         *logging.Logger
ctx            context.Context
db             *gorm.DB
testingService *testingApp.TestRunService
projectService *projectsApp.ProjectService
tagService     *tagsApp.TagService
)

BeforeEach(func() {
var err error
logger, err = logging.NewLogger(&config.LoggingConfig{
Level:      "error",
Format:     "json",
Output:     "stdout",
Structured: true,
})
Expect(err).To(BeNil())

// Create in-memory SQLite database
db, err = gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
Logger: gormlogger.Default.LogMode(gormlogger.Silent),
})
Expect(err).To(BeNil())

// Auto-migrate all models
err = db.AutoMigrate(
&database.TestRun{},
&database.SuiteRun{},
&database.SpecRun{},
&database.Tag{},
&database.TestRunTag{},
&database.ProjectDetails{},
&database.ProjectAccess{},
&database.ProjectPermission{},
&database.User{},
&database.UserGroup{},
)
Expect(err).To(BeNil())

// Create repositories
testRunRepo := testingInfra.NewGormTestRunRepository(db)
suiteRunRepo := testingInfra.NewGormSuiteRunRepository(db)
specRunRepo := testingInfra.NewGormSpecRunRepository(db)
projectRepo := projectsInfra.NewGormProjectRepository(db)
permissionRepo := projectsInfra.NewGormProjectPermissionRepository(db)
tagRepo := tagsInfra.NewGormTagRepository(db)

// Create services
testingService = testingApp.NewTestRunService(testRunRepo, suiteRunRepo, specRunRepo)
projectService = projectsApp.NewProjectService(projectRepo, permissionRepo)
tagService = tagsApp.NewTagService(tagRepo)

// Create resolver with real services
resolver = &Resolver{
testingService: testingService,
projectService: projectService,
tagService:     tagService,
db:             db,
logger:         logger,
}
qr = &queryResolver{resolver}
mr = &mutationResolver{resolver}
ctx = context.Background()
})

AfterEach(func() {
// Clean up database
sqlDB, err := db.DB()
if err == nil {
sqlDB.Close()
}
})

// Helper function tests
Describe("Helper Functions", func() {
It("convertDurationPtr should convert duration to milliseconds", func() {
result := convertDurationPtr(5 * time.Second)
Expect(result).NotTo(BeNil())
Expect(*result).To(Equal(5000))
})

It("getStringValue should handle nil pointer", func() {
result := getStringValue(nil)
Expect(result).To(Equal(""))
})

It("getStringValue should return value from pointer", func() {
str := "test"
result := getStringValue(&str)
Expect(result).To(Equal("test"))
})
})

// Test Run tests
Describe("GetTestRun_domain", func() {
It("should return error for non-existent ID", func() {
_, err := qr.GetTestRun_domain(ctx, "999999")
Expect(err).NotTo(BeNil())
})

It("should return error for invalid ID format", func() {
_, err := qr.GetTestRun_domain(ctx, "invalid-id")
Expect(err).NotTo(BeNil())
Expect(err.Error()).To(ContainSubstring("test run not found"))
})

It("should retrieve test run by ID", func() {
// Create a test run in database
now := time.Now()
dbTestRun := &database.TestRun{
RunID:       "test-run-123",
ProjectID:   "test-project",
Status:      "passed",
StartTime:   now.Add(-1 * time.Hour),
EndTime:     &now,
Duration:    3600000,
TotalTests:  10,
PassedTests: 10,
}
err := db.Create(dbTestRun).Error
Expect(err).To(BeNil())

result, err := qr.GetTestRun_domain(ctx, strconv.FormatUint(uint64(dbTestRun.ID), 10))
Expect(err).To(BeNil())
Expect(result).NotTo(BeNil())
Expect(result.RunID).To(Equal("test-run-123"))
Expect(result.ProjectID).To(Equal("test-project"))
})
})

Describe("RecentTestRuns_domain", func() {
It("should return empty list when no test runs", func() {
result, err := qr.RecentTestRuns_domain(ctx, nil, nil)
Expect(err).To(BeNil())
Expect(result).NotTo(BeNil())
Expect(result).To(HaveLen(0))
})

It("should return recent test runs", func() {
// Create multiple test runs
now := time.Now()
for i := 0; i < 3; i++ {
endTime := now.Add(-time.Duration(i) * time.Hour)
dbTestRun := &database.TestRun{
RunID:     "run-" + strconv.Itoa(i),
ProjectID: "project-1",
Status:    "passed",
StartTime: now.Add(-time.Duration(i+1) * time.Hour),
EndTime:   &endTime,
Duration:  1800000,
}
err := db.Create(dbTestRun).Error
Expect(err).To(BeNil())
}

result, err := qr.RecentTestRuns_domain(ctx, nil, nil)
Expect(err).To(BeNil())
Expect(result).NotTo(BeNil())
Expect(len(result)).To(BeNumerically(">=", 3))
})

It("should filter by projectID", func() {
now := time.Now()
// Create test runs for project-a
for i := 0; i < 2; i++ {
dbTestRun := &database.TestRun{
RunID:     "project-a-run-" + strconv.Itoa(i),
ProjectID: "project-a",
Status:    "passed",
StartTime: now,
EndTime:   &now,
}
err := db.Create(dbTestRun).Error
Expect(err).To(BeNil())
}

projectID := "project-a"
result, err := qr.RecentTestRuns_domain(ctx, &projectID, nil)
Expect(err).To(BeNil())
Expect(result).NotTo(BeNil())
Expect(len(result)).To(Equal(2))
for _, tr := range result {
Expect(tr.ProjectID).To(Equal("project-a"))
}
})
})

// Project tests
Describe("GetProject_domain", func() {
It("should return error for non-existent project", func() {
_, err := qr.GetProject_domain(ctx, "non-existent")
Expect(err).NotTo(BeNil())
})

It("should retrieve existing project", func() {
// Create project through service
_, err := projectService.CreateProject(ctx, "test-proj-1", "Test Project", "team1", "user1")
Expect(err).To(BeNil())

result, err := qr.GetProject_domain(ctx, "test-proj-1")
Expect(err).To(BeNil())
Expect(result).NotTo(BeNil())
Expect(result.ProjectID).To(Equal("test-proj-1"))
Expect(result.Name).To(Equal("Test Project"))
})
})

Describe("ListProjects_domain", func() {
It("should return empty list when no projects", func() {
result, err := qr.ListProjects_domain(ctx, nil, nil, nil)
Expect(err).To(BeNil())
Expect(result).NotTo(BeNil())
Expect(result).To(HaveLen(0))
})

It("should list all projects", func() {
// Create multiple projects
for i := 0; i < 3; i++ {
_, err := projectService.CreateProject(ctx,
projectsDomain.ProjectID("proj-"+strconv.Itoa(i)),
"Project "+strconv.Itoa(i),
"team1",
"user1")
Expect(err).To(BeNil())
}

result, err := qr.ListProjects_domain(ctx, nil, nil, nil)
Expect(err).To(BeNil())
Expect(result).NotTo(BeNil())
Expect(len(result)).To(Equal(3))
})

It("should support pagination with limit", func() {
// Create 5 projects
for i := 0; i < 5; i++ {
_, err := projectService.CreateProject(ctx,
projectsDomain.ProjectID("proj-"+strconv.Itoa(i)),
"Project "+strconv.Itoa(i),
"team1",
"user1")
Expect(err).To(BeNil())
}

limit := 3
result, err := qr.ListProjects_domain(ctx, &limit, nil, nil)
Expect(err).To(BeNil())
Expect(result).NotTo(BeNil())
Expect(len(result)).To(Equal(3))
})
})

// Tag tests
Describe("ListTags_domain", func() {
It("should return empty list when no tags", func() {
result, err := qr.ListTags_domain(ctx)
Expect(err).To(BeNil())
Expect(result).NotTo(BeNil())
Expect(result).To(HaveLen(0))
})

It("should list all tags", func() {
// Create tags through service
for i := 0; i < 3; i++ {
_, err := tagService.GetOrCreateTag(ctx, "tag-"+strconv.Itoa(i))
Expect(err).To(BeNil())
}

result, err := qr.ListTags_domain(ctx)
Expect(err).To(BeNil())
Expect(result).NotTo(BeNil())
Expect(len(result)).To(BeNumerically(">=", 3))
})
})

Describe("CreateTag_domain", func() {
It("should create a new tag", func() {
input := model.CreateTagInput{
Name: "new-tag",
}
result, err := mr.CreateTag_domain(ctx, input)
Expect(err).To(BeNil())
Expect(result).NotTo(BeNil())
Expect(result.Name).To(Equal("new-tag"))
})

It("should create tag with category:value format", func() {
input := model.CreateTagInput{
Name: "priority:high",
}
result, err := mr.CreateTag_domain(ctx, input)
Expect(err).To(BeNil())
Expect(result).NotTo(BeNil())
Expect(result.Name).To(Equal("priority:high"))
})
})

// Project mutation tests
Describe("CreateProject_domain", func() {
It("should require authentication", func() {
input := model.CreateProjectInput{
ProjectID: "new-proj",
Name:      "New Project",
}
_, err := mr.CreateProject_domain(ctx, input)
Expect(err).NotTo(BeNil())
})

It("should create project with admin user", func() {
user := &authDomain.User{
UserID: "admin1",
Role:   authDomain.RoleAdmin,
}
ctxWithUser := context.WithValue(ctx, "user", user)

input := model.CreateProjectInput{
ProjectID: "new-proj-123",
Name:      "New Project",
}
result, err := mr.CreateProject_domain(ctxWithUser, input)
Expect(err).To(BeNil())
Expect(result).NotTo(BeNil())
Expect(result.ProjectID).To(Equal("new-proj-123"))
Expect(result.Name).To(Equal("New Project"))
})

It("should create project with manager user", func() {
user := &authDomain.User{
UserID: "manager1",
Role:   authDomain.RoleManager,
}
ctxWithUser := context.WithValue(ctx, "user", user)

input := model.CreateProjectInput{
ProjectID: "manager-proj",
Name:      "Manager Project",
}
result, err := mr.CreateProject_domain(ctxWithUser, input)
Expect(err).To(BeNil())
Expect(result).NotTo(BeNil())
Expect(result.ProjectID).To(Equal("manager-proj"))
})

It("should reject regular user", func() {
user := &authDomain.User{
UserID: "user1",
Role:   authDomain.RoleUser,
}
ctxWithUser := context.WithValue(ctx, "user", user)

input := model.CreateProjectInput{
ProjectID: "user-proj",
Name:      "User Project",
}
_, err := mr.CreateProject_domain(ctxWithUser, input)
Expect(err).NotTo(BeNil())
Expect(err.Error()).To(ContainSubstring("insufficient permissions"))
})

It("should create project with optional fields", func() {
user := &authDomain.User{
UserID: "admin1",
Role:   authDomain.RoleAdmin,
}
ctxWithUser := context.WithValue(ctx, "user", user)

desc := "Project description"
repo := "https://github.com/test/repo"
branch := "develop"
team := "dev-team"

input := model.CreateProjectInput{
ProjectID:     "full-proj",
Name:          "Full Project",
Description:   &desc,
Repository:    &repo,
DefaultBranch: &branch,
Team:          &team,
}
result, err := mr.CreateProject_domain(ctxWithUser, input)
Expect(err).To(BeNil())
Expect(result).NotTo(BeNil())
Expect(result.Description).NotTo(BeNil())
Expect(*result.Description).To(Equal("Project description"))
Expect(result.Repository).NotTo(BeNil())
Expect(*result.Repository).To(Equal("https://github.com/test/repo"))
})
})

Describe("UpdateProject_domain", func() {
It("should update project name", func() {
// Create project first
user := &authDomain.User{
UserID: "admin1",
Role:   authDomain.RoleAdmin,
}
ctxWithUser := context.WithValue(ctx, "user", user)

createInput := model.CreateProjectInput{
ProjectID: "update-proj",
Name:      "Original Name",
}
_, err := mr.CreateProject_domain(ctxWithUser, createInput)
Expect(err).To(BeNil())

// Update project
newName := "Updated Name"
updateInput := model.UpdateProjectInput{
Name: &newName,
}
result, err := mr.UpdateProject_domain(ctxWithUser, "update-proj", updateInput)
Expect(err).To(BeNil())
Expect(result).NotTo(BeNil())
Expect(result.Name).To(Equal("Updated Name"))
})

It("should update project description", func() {
user := &authDomain.User{
UserID: "admin1",
Role:   authDomain.RoleAdmin,
}
ctxWithUser := context.WithValue(ctx, "user", user)

createInput := model.CreateProjectInput{
ProjectID: "desc-proj",
Name:      "Project",
}
_, err := mr.CreateProject_domain(ctxWithUser, createInput)
Expect(err).To(BeNil())

newDesc := "New description"
updateInput := model.UpdateProjectInput{
Description: &newDesc,
}
result, err := mr.UpdateProject_domain(ctxWithUser, "desc-proj", updateInput)
Expect(err).To(BeNil())
Expect(result).NotTo(BeNil())
Expect(result.Description).NotTo(BeNil())
Expect(*result.Description).To(Equal("New description"))
})
})

Describe("DeleteProject_domain", func() {
It("should delete project with admin user", func() {
// Create project
user := &authDomain.User{
UserID: "admin1",
Role:   authDomain.RoleAdmin,
}
ctxWithUser := context.WithValue(ctx, "user", user)

createInput := model.CreateProjectInput{
ProjectID: "delete-proj",
Name:      "To Delete",
}
_, err := mr.CreateProject_domain(ctxWithUser, createInput)
Expect(err).To(BeNil())

// Delete project
result, err := mr.DeleteProject_domain(ctxWithUser, "delete-proj")
Expect(err).To(BeNil())
Expect(result).To(BeTrue())

// Verify deleted
_, err = qr.GetProject_domain(ctx, "delete-proj")
Expect(err).NotTo(BeNil())
})
})

Describe("Project_domain", func() {
It("should retrieve project by numeric ID", func() {
// Create project
project, err := projectService.CreateProject(ctx, "id-proj", "ID Project", "team1", "user1")
Expect(err).To(BeNil())

result, err := qr.Project_domain(ctx, strconv.FormatUint(uint64(project.ID()), 10))
Expect(err).To(BeNil())
Expect(result).NotTo(BeNil())
Expect(result.ProjectID).To(Equal("id-proj"))
})

})

Describe("ProjectByProjectID_domain", func() {
It("should retrieve project by project ID", func() {
_, err := projectService.CreateProject(ctx, "lookup-proj", "Lookup Project", "team1", "user1")
Expect(err).To(BeNil())

result, err := qr.ProjectByProjectID_domain(ctx, "lookup-proj")
Expect(err).To(BeNil())
Expect(result).NotTo(BeNil())
Expect(result.ProjectID).To(Equal("lookup-proj"))
})

It("should return error for non-existent project", func() {
_, err := qr.ProjectByProjectID_domain(ctx, "non-existent-proj")
Expect(err).NotTo(BeNil())
})
})

// Conversion function tests
Describe("convertProjectToGraphQL", func() {
It("should convert project with CanManage based on user role", func() {
project, err := projectsDomain.NewProject(
projectsDomain.ProjectID("test-proj"),
"Test Project",
projectsDomain.Team("team1"),
)
Expect(err).To(BeNil())
project.SetID(1)

// Without user - cannot manage
result := resolver.convertProjectToGraphQL(ctx, project)
Expect(result.CanManage).To(BeFalse())

// With admin - can manage
user := &authDomain.User{
UserID: "admin1",
Role:   authDomain.RoleAdmin,
}
ctxWithUser := context.WithValue(ctx, "user", user)
result = resolver.convertProjectToGraphQL(ctxWithUser, project)
Expect(result.CanManage).To(BeTrue())
})
})

Describe("ConvertTagToGraphQL", func() {
It("should convert tag", func() {
tag, err := tagsDomain.NewTag("test-tag")
Expect(err).To(BeNil())

result := resolver.ConvertTagToGraphQL(tag)
Expect(result).NotTo(BeNil())
Expect(result.Name).To(Equal("test-tag"))
})
})

// Connection-based functions
Describe("Projects_domain", func() {
It("should require authentication", func() {
_, err := qr.Projects_domain(ctx, nil, nil, nil)
Expect(err).NotTo(BeNil())
Expect(err.Error()).To(ContainSubstring("not authenticated"))
})

It("should return empty connection when no projects", func() {
user := &authDomain.User{
UserID: "user1",
Role:   authDomain.RoleUser,
}
ctxWithUser := context.WithValue(ctx, "user", user)

result, err := qr.Projects_domain(ctxWithUser, nil, nil, nil)
Expect(err).To(BeNil())
Expect(result).NotTo(BeNil())
Expect(result.Edges).To(HaveLen(0))
})

It("should list projects in connection format", func() {
user := &authDomain.User{
UserID: "user1",
Role:   authDomain.RoleUser,
Groups: []authDomain.UserGroup{
{GroupName: "team1"},
},
}
ctxWithUser := context.WithValue(ctx, "user", user)

// Create some projects
for i := 0; i < 3; i++ {
_, err := projectService.CreateProject(ctx,
projectsDomain.ProjectID("conn-proj-"+strconv.Itoa(i)),
"Connection Project "+strconv.Itoa(i),
"team1",
"user1")
Expect(err).To(BeNil())
}

result, err := qr.Projects_domain(ctxWithUser, nil, nil, nil)
Expect(err).To(BeNil())
Expect(result).NotTo(BeNil())
Expect(len(result.Edges)).To(Equal(3))
Expect(result.PageInfo).NotTo(BeNil())
})
})

Describe("TestRuns_domain", func() {
It("should return empty connection when no test runs", func() {
result, err := qr.TestRuns_domain(ctx, nil, nil, nil, nil, nil)
Expect(err).To(BeNil())
Expect(result).NotTo(BeNil())
Expect(result.Edges).To(HaveLen(0))
})

It("should list test runs in connection format", func() {
// Create test runs
now := time.Now()
projectID := "test-project"
for i := 0; i < 3; i++ {
dbTestRun := &database.TestRun{
RunID:     "conn-run-" + strconv.Itoa(i),
ProjectID: projectID,
Status:    "passed",
StartTime: now,
EndTime:   &now,
}
err := db.Create(dbTestRun).Error
Expect(err).To(BeNil())
}

// Provide filter with projectID since the service requires it
filter := &model.TestRunFilter{
ProjectID: &projectID,
}
result, err := qr.TestRuns_domain(ctx, filter, nil, nil, nil, nil)
Expect(err).To(BeNil())
Expect(result).NotTo(BeNil())
Expect(len(result.Edges)).To(BeNumerically(">=", 3))
Expect(result.PageInfo).NotTo(BeNil())
})

It("should filter by projectID", func() {
now := time.Now()
// Create test runs for specific project
for i := 0; i < 2; i++ {
dbTestRun := &database.TestRun{
RunID:     "filtered-run-" + strconv.Itoa(i),
ProjectID: "filter-project",
Status:    "passed",
StartTime: now,
EndTime:   &now,
}
err := db.Create(dbTestRun).Error
Expect(err).To(BeNil())
}

projectID := "filter-project"
filter := &model.TestRunFilter{
ProjectID: &projectID,
}
result, err := qr.TestRuns_domain(ctx, filter, nil, nil, nil, nil)
Expect(err).To(BeNil())
Expect(result).NotTo(BeNil())
Expect(len(result.Edges)).To(BeNumerically(">=", 2))
})
})

Describe("Tags_domain", func() {
It("should return empty connection when no tags", func() {
result, err := qr.Tags_domain(ctx, nil, nil, nil)
Expect(err).To(BeNil())
Expect(result).NotTo(BeNil())
Expect(result.Edges).To(HaveLen(0))
})

It("should list tags in connection format", func() {
// Create tags
for i := 0; i < 3; i++ {
_, err := tagService.GetOrCreateTag(ctx, "conn-tag-"+strconv.Itoa(i))
Expect(err).To(BeNil())
}

result, err := qr.Tags_domain(ctx, nil, nil, nil)
Expect(err).To(BeNil())
Expect(result).NotTo(BeNil())
Expect(len(result.Edges)).To(BeNumerically(">=", 3))
Expect(result.PageInfo).NotTo(BeNil())
})
})

Describe("DashboardSummary_domain", func() {
It("should return dashboard summary", func() {
// Create some data
_, err := projectService.CreateProject(ctx, "dash-proj", "Dashboard Project", "team1", "user1")
Expect(err).To(BeNil())

now := time.Now()
dbTestRun := &database.TestRun{
RunID:       "dash-run",
ProjectID:   "dash-proj",
Status:      "passed",
StartTime:   now,
EndTime:     &now,
TotalTests:  10,
PassedTests: 8,
FailedTests: 2,
}
err = db.Create(dbTestRun).Error
Expect(err).To(BeNil())

result, err := qr.DashboardSummary_domain(ctx)
Expect(err).To(BeNil())
Expect(result).NotTo(BeNil())
Expect(result.ProjectCount).To(BeNumerically(">=", 1))
})
})

Describe("TreemapData_domain", func() {
It("should return treemap data", func() {
// Create project and test run
_, err := projectService.CreateProject(ctx, "tree-proj", "Treemap Project", "team1", "user1")
Expect(err).To(BeNil())

now := time.Now()
dbTestRun := &database.TestRun{
RunID:       "tree-run",
ProjectID:   "tree-proj",
Status:      "passed",
StartTime:   now,
EndTime:     &now,
TotalTests:  10,
PassedTests: 10,
}
err = db.Create(dbTestRun).Error
Expect(err).To(BeNil())

result, err := qr.TreemapData_domain(ctx, nil, nil)
Expect(err).To(BeNil())
Expect(result).NotTo(BeNil())
})

It("should filter by projectID", func() {
_, err := projectService.CreateProject(ctx, "tree-filtered-proj", "Filtered Project", "team1", "user1")
Expect(err).To(BeNil())

projectID := "tree-filtered-proj"
result, err := qr.TreemapData_domain(ctx, &projectID, nil)
Expect(err).To(BeNil())
Expect(result).NotTo(BeNil())
})
})
})
