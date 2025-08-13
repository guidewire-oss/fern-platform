// Package graphql provides GraphQL API for the fern-reporter service
package graphql

import (
	analyticsApp "github.com/guidewire-oss/fern-platform/internal/domains/analytics/application"
	"github.com/guidewire-oss/fern-platform/internal/domains/integrations"
	projectsApp "github.com/guidewire-oss/fern-platform/internal/domains/projects/application"
	tagsApp "github.com/guidewire-oss/fern-platform/internal/domains/tags/application"
	testingApp "github.com/guidewire-oss/fern-platform/internal/domains/testing/application"
	"github.com/guidewire-oss/fern-platform/internal/reporter/graphql/dataloader"
	"github.com/guidewire-oss/fern-platform/pkg/logging"
	"gorm.io/gorm"
)

// This file will not be regenerated automatically.
//
// It serves as dependency injection for your app, add any dependencies you require here.

// Resolver is the root GraphQL resolver
type Resolver struct {
	testingService        *testingApp.TestRunService
	projectService        *projectsApp.ProjectService
	tagService            *tagsApp.TagService
	flakyDetectionService *analyticsApp.FlakyDetectionService
	jiraConnectionService *integrations.JiraConnectionService
	loaders               *dataloader.Loaders
	db                    *gorm.DB
	logger                *logging.Logger
}

// NewResolver creates a new GraphQL resolver
func NewResolver(
	testingService *testingApp.TestRunService,
	projectService *projectsApp.ProjectService,
	tagService *tagsApp.TagService,
	flakyDetectionService *analyticsApp.FlakyDetectionService,
	jiraConnectionService *integrations.JiraConnectionService,
	db *gorm.DB,
	logger *logging.Logger,
) *Resolver {
	return &Resolver{
		testingService:        testingService,
		projectService:        projectService,
		tagService:            tagService,
		flakyDetectionService: flakyDetectionService,
		jiraConnectionService: jiraConnectionService,
		loaders:               dataloader.NewLoaders(db),
		db:                    db,
		logger:                logger,
	}
}
