package domains

import (
	"gorm.io/gorm"

	// Auth domain
	authApp "github.com/guidewire-oss/fern-platform/internal/domains/auth/application"
	authInfra "github.com/guidewire-oss/fern-platform/internal/domains/auth/infrastructure"
	authInterfaces "github.com/guidewire-oss/fern-platform/internal/domains/auth/interfaces"

	// Analytics domain
	analyticsApp "github.com/guidewire-oss/fern-platform/internal/domains/analytics/application"
	analyticsDomain "github.com/guidewire-oss/fern-platform/internal/domains/analytics/domain"
	analyticsInfra "github.com/guidewire-oss/fern-platform/internal/domains/analytics/infrastructure"
	analyticsInterfaces "github.com/guidewire-oss/fern-platform/internal/domains/analytics/interfaces"

	// Testing domain
	testingApp "github.com/guidewire-oss/fern-platform/internal/domains/testing/application"
	testingInfra "github.com/guidewire-oss/fern-platform/internal/domains/testing/infrastructure"
	testingInterfaces "github.com/guidewire-oss/fern-platform/internal/domains/testing/interfaces"

	// Projects domain
	projectsApp "github.com/guidewire-oss/fern-platform/internal/domains/projects/application"
	projectsInfra "github.com/guidewire-oss/fern-platform/internal/domains/projects/infrastructure"

	// Tags domain
	tagsApp "github.com/guidewire-oss/fern-platform/internal/domains/tags/application"
	tagsInfra "github.com/guidewire-oss/fern-platform/internal/domains/tags/infrastructure"

	// Integrations domain
	"github.com/guidewire-oss/fern-platform/internal/domains/integrations"
	integrationsInfra "github.com/guidewire-oss/fern-platform/internal/infrastructure/repositories"

	"github.com/guidewire-oss/fern-platform/pkg/config"
	"github.com/guidewire-oss/fern-platform/pkg/logging"
)

// DomainFactory creates and wires all domain components
type DomainFactory struct {
	db         *gorm.DB
	logger     *logging.Logger
	authConfig *config.AuthConfig

	// Auth domain
	authService    *authApp.AuthenticationService
	authzService   *authApp.AuthorizationService
	authMiddleware *authInterfaces.AuthMiddlewareAdapter

	// Analytics domain
	flakyDetectionService *analyticsApp.FlakyDetectionService
	flakyDetectionAdapter *analyticsInterfaces.FlakyDetectionAdapter

	// Testing domain
	testRunService *testingApp.TestRunService
	testingAdapter *testingInterfaces.TestServiceAdapter

	// Projects domain
	projectService *projectsApp.ProjectService

	// Tags domain
	tagService *tagsApp.TagService

	// Integrations domain
	jiraConnectionService *integrations.JiraConnectionService
}

// NewDomainFactory creates a new domain factory
func NewDomainFactory(db *gorm.DB, logger *logging.Logger, authConfig *config.AuthConfig) *DomainFactory {
	factory := &DomainFactory{
		db:         db,
		logger:     logger,
		authConfig: authConfig,
	}

	// Initialize Auth domain (must be first as others may depend on it)
	factory.initAuthDomain()

	// Initialize Analytics domain
	factory.initAnalyticsDomain()

	// Initialize Testing domain
	factory.initTestingDomain()

	// Initialize Projects domain
	factory.initProjectsDomain()

	// Initialize Tags domain
	factory.initTagsDomain()

	// Initialize Integrations domain
	factory.initIntegrationsDomain()

	return factory
}

// initTestingDomain initializes the testing domain components
func (f *DomainFactory) initTestingDomain() {
	// Create repositories
	testRunRepo := testingInfra.NewGormTestRunRepository(f.db)
	suiteRunRepo := testingInfra.NewGormSuiteRunRepository(f.db)
	specRunRepo := testingInfra.NewGormSpecRunRepository(f.db)

	// Create application service
	f.testRunService = testingApp.NewTestRunService(
		testRunRepo,
		suiteRunRepo,
		specRunRepo,
	)

	// Create adapter
	f.testingAdapter = testingInterfaces.NewTestServiceAdapter(
		f.testRunService,
		f.logger,
	)
}

// initProjectsDomain initializes the projects domain components
func (f *DomainFactory) initProjectsDomain() {
	// Create repositories
	projectRepo := projectsInfra.NewGormProjectRepository(f.db)
	permissionRepo := projectsInfra.NewGormProjectPermissionRepository(f.db)

	// Create application service
	f.projectService = projectsApp.NewProjectService(projectRepo, permissionRepo)

}

// GetTestingService returns the new domain test run service
func (f *DomainFactory) GetTestingService() *testingApp.TestRunService {
	return f.testRunService
}

// GetTestingAdapter returns the testing adapter for HTTP/GraphQL
func (f *DomainFactory) GetTestingAdapter() *testingInterfaces.TestServiceAdapter {
	return f.testingAdapter
}

// GetProjectDomainService returns the new domain project service
func (f *DomainFactory) GetProjectDomainService() *projectsApp.ProjectService {
	return f.projectService
}

// initTagsDomain initializes the tags domain components
func (f *DomainFactory) initTagsDomain() {
	// Create repositories
	tagRepo := tagsInfra.NewGormTagRepository(f.db)

	// Create application service
	f.tagService = tagsApp.NewTagService(tagRepo)

}

// GetTagDomainService returns the new domain tag service
func (f *DomainFactory) GetTagDomainService() *tagsApp.TagService {
	return f.tagService
}

// initAuthDomain initializes the auth domain components
func (f *DomainFactory) initAuthDomain() {
	// Create repositories
	userRepo := authInfra.NewGormUserRepository(f.db)
	sessionRepo := authInfra.NewGormSessionRepository(f.db)

	// Create application services
	f.authService = authApp.NewAuthenticationService(userRepo, sessionRepo)
	f.authzService = authApp.NewAuthorizationService(userRepo)

	// Create OAuth adapter
	oauthAdapter := authInterfaces.NewOAuthAdapter(f.authConfig, f.logger)

	// Create middleware adapter
	f.authMiddleware = authInterfaces.NewAuthMiddlewareAdapter(
		f.authService,
		f.authzService,
		oauthAdapter,
		f.authConfig,
		f.logger,
	)
}

// GetAuthService returns the authentication service
func (f *DomainFactory) GetAuthService() *authApp.AuthenticationService {
	return f.authService
}

// GetAuthorizationService returns the authorization service
func (f *DomainFactory) GetAuthorizationService() *authApp.AuthorizationService {
	return f.authzService
}

// GetAuthMiddleware returns the auth middleware adapter
func (f *DomainFactory) GetAuthMiddleware() *authInterfaces.AuthMiddlewareAdapter {
	return f.authMiddleware
}

// initAnalyticsDomain initializes the analytics domain components
func (f *DomainFactory) initAnalyticsDomain() {
	// Create repository
	flakyRepo := analyticsInfra.NewGormFlakyDetectionRepository(f.db)

	// Create service with default config
	config := analyticsDomain.DefaultFlakyTestDetectionConfig()
	f.flakyDetectionService = analyticsApp.NewFlakyDetectionService(flakyRepo, config)

	// Create adapter
	f.flakyDetectionAdapter = analyticsInterfaces.NewFlakyDetectionAdapter(f.flakyDetectionService, f.logger)
}

// GetFlakyDetectionService returns the flaky detection service
func (f *DomainFactory) GetFlakyDetectionService() *analyticsApp.FlakyDetectionService {
	return f.flakyDetectionService
}

// GetFlakyDetectionAdapter returns the flaky detection adapter
func (f *DomainFactory) GetFlakyDetectionAdapter() *analyticsInterfaces.FlakyDetectionAdapter {
	return f.flakyDetectionAdapter
}

// initIntegrationsDomain initializes the integrations domain components
func (f *DomainFactory) initIntegrationsDomain() {
	// Create repository
	jiraConnRepo := integrationsInfra.NewGormJiraConnectionRepository(f.db)

	// Create JIRA client
	jiraClient := integrations.NewDefaultJiraClient()

	// Get encryption key from config (or generate one)
	// For now, use a placeholder - in production this should come from secure config
	encryptionKey := []byte("your-32-byte-encryption-key-here") // TODO: Load from secure config

	// Create service
	f.jiraConnectionService = integrations.NewJiraConnectionService(
		jiraConnRepo,
		jiraClient,
		encryptionKey,
	)
}

// GetJiraConnectionService returns the JIRA connection service
func (f *DomainFactory) GetJiraConnectionService() *integrations.JiraConnectionService {
	return f.jiraConnectionService
}
