package infrastructure

import (
	"time"

	"github.com/guidewire-oss/fern-platform/internal/domains/testing/domain"
	"github.com/guidewire-oss/fern-platform/pkg/database"
)

// DatabaseConverter handles conversions between domain models and database models
type DatabaseConverter struct{}

// NewDatabaseConverter creates a new database converter
func NewDatabaseConverter() *DatabaseConverter {
	return &DatabaseConverter{}
}

// Domain to Database conversions

// ConvertTestRunToDatabase converts a domain TestRun to database model
func (c *DatabaseConverter) ConvertTestRunToDatabase(domainTestRun *domain.TestRun) *database.TestRun {
	dbSuiteRuns := c.ConvertDomainSuiteRunsToDatabase(domainTestRun.SuiteRuns)
	dbTags := c.ConvertDomainTagsToDatabase(domainTestRun.Tags)

	return &database.TestRun{
		ProjectID:    domainTestRun.ProjectID,
		RunID:        domainTestRun.RunID,
		Status:       domainTestRun.Status,
		Branch:       domainTestRun.Branch,
		CommitSHA:    domainTestRun.GitCommit,
		StartTime:    domainTestRun.StartTime,
		EndTime:      domainTestRun.EndTime,
		Duration:     int64(domainTestRun.Duration / time.Millisecond),
		TotalTests:   domainTestRun.TotalTests,
		PassedTests:  domainTestRun.PassedTests,
		FailedTests:  domainTestRun.FailedTests,
		SkippedTests: domainTestRun.SkippedTests,
		Environment:  domainTestRun.Environment,
		Metadata:     database.JSONMap(domainTestRun.Metadata),
		Tags:         dbTags,
		SuiteRuns:    dbSuiteRuns,
	}
}

// ConvertDomainSuiteRunsToDatabase converts domain SuiteRuns to database SuiteRuns
func (c *DatabaseConverter) ConvertDomainSuiteRunsToDatabase(domainSuiteRuns []domain.SuiteRun) []database.SuiteRun {
	dbSuiteRuns := make([]database.SuiteRun, len(domainSuiteRuns))

	for i, domainSuite := range domainSuiteRuns {
		// Convert SpecRuns
		dbSpecRuns := c.ConvertDomainSpecRunsToDatabase(domainSuite.SpecRuns)

		// Convert tags
		dbTags := c.ConvertDomainTagsToDatabase(domainSuite.Tags)

		dbSuiteRuns[i] = database.SuiteRun{
			TestRunID:    domainSuite.TestRunID,
			SuiteName:    domainSuite.Name,
			Status:       domainSuite.Status,
			StartTime:    domainSuite.StartTime,
			EndTime:      domainSuite.EndTime,
			TotalSpecs:   domainSuite.TotalTests,                         // TotalTests -> TotalSpecs
			PassedSpecs:  domainSuite.PassedTests,                        // PassedTests -> PassedSpecs
			FailedSpecs:  domainSuite.FailedTests,                        // FailedTests -> FailedSpecs
			SkippedSpecs: domainSuite.SkippedTests,                       // SkippedTests -> SkippedSpecs
			Duration:     int64(domainSuite.Duration / time.Millisecond), // Convert to milliseconds
			Tags:         dbTags,
			SpecRuns:     dbSpecRuns,
		}
	}

	return dbSuiteRuns
}

// ConvertDomainSpecRunsToDatabase converts domain SpecRuns to database SpecRuns
func (c *DatabaseConverter) ConvertDomainSpecRunsToDatabase(domainSpecRuns []*domain.SpecRun) []database.SpecRun {
	dbSpecRuns := make([]database.SpecRun, len(domainSpecRuns))

	for i, domainSpec := range domainSpecRuns {
		// Combine ErrorMessage and FailureMessage into ErrorMessage
		errorMessage := domainSpec.ErrorMessage
		if errorMessage == "" && domainSpec.FailureMessage != "" {
			errorMessage = domainSpec.FailureMessage
		}

		// Convert tags
		dbTags := c.ConvertDomainTagsToDatabase(domainSpec.Tags)

		dbSpecRuns[i] = database.SpecRun{
			SuiteRunID:   domainSpec.SuiteRunID,
			SpecName:     domainSpec.Name, // Name -> SpecName
			Status:       domainSpec.Status,
			StartTime:    domainSpec.StartTime,
			EndTime:      domainSpec.EndTime,
			Duration:     int64(domainSpec.Duration / time.Millisecond), // Convert to milliseconds
			ErrorMessage: errorMessage,                                  // Combine ErrorMessage and FailureMessage
			StackTrace:   domainSpec.StackTrace,
			RetryCount:   domainSpec.RetryCount,
			IsFlaky:      domainSpec.IsFlaky,
			Tags:         dbTags,
		}
	}

	return dbSpecRuns
}

// Database to Domain conversions

// ConvertTestRunToDomain converts a database TestRun to domain model
func (c *DatabaseConverter) ConvertTestRunToDomain(dbTestRun *database.TestRun) *domain.TestRun {
	// Convert metadata
	metadata := make(map[string]interface{})
	if dbTestRun.Metadata != nil {
		metadata = map[string]interface{}(dbTestRun.Metadata)
	}

	// Convert suite runs
	suiteRuns := make([]domain.SuiteRun, len(dbTestRun.SuiteRuns))
	for i, dbSuite := range dbTestRun.SuiteRuns {
		suiteRuns[i] = c.ConvertSuiteRunToDomain(&dbSuite)
	}

	// Convert tags
	tags := c.ConvertDatabaseTagsToDomain(dbTestRun.Tags)

	return &domain.TestRun{
		ID:           dbTestRun.ID,
		RunID:        dbTestRun.RunID,
		ProjectID:    dbTestRun.ProjectID,
		Name:         "", // Not stored in database model
		Status:       dbTestRun.Status,
		Branch:       dbTestRun.Branch,
		GitBranch:    dbTestRun.Branch, // Use same value
		GitCommit:    dbTestRun.CommitSHA,
		StartTime:    dbTestRun.StartTime,
		EndTime:      dbTestRun.EndTime,
		Duration:     time.Duration(dbTestRun.Duration) * time.Millisecond,
		TotalTests:   dbTestRun.TotalTests,
		PassedTests:  dbTestRun.PassedTests,
		FailedTests:  dbTestRun.FailedTests,
		SkippedTests: dbTestRun.SkippedTests,
		Environment:  dbTestRun.Environment,
		Source:       "", // Not stored in database model
		SessionID:    "", // Not stored in database model
		Metadata:     metadata,
		Tags:         tags,
		SuiteRuns:    suiteRuns,
	}
}

// ConvertSuiteRunToDomain converts a database SuiteRun to domain model
func (c *DatabaseConverter) ConvertSuiteRunToDomain(dbSuite *database.SuiteRun) domain.SuiteRun {
	// Convert spec runs
	specRuns := make([]*domain.SpecRun, len(dbSuite.SpecRuns))
	for i, dbSpec := range dbSuite.SpecRuns {
		specRuns[i] = c.ConvertSpecRunToDomain(&dbSpec)
	}

	// Convert tags
	tags := c.ConvertDatabaseTagsToDomain(dbSuite.Tags)

	return domain.SuiteRun{
		ID:           dbSuite.ID,
		TestRunID:    dbSuite.TestRunID,
		Name:         dbSuite.SuiteName,
		PackageName:  "", // Not in database model
		ClassName:    "", // Not in database model
		Status:       dbSuite.Status,
		StartTime:    dbSuite.StartTime,
		EndTime:      dbSuite.EndTime,
		TotalTests:   dbSuite.TotalSpecs,
		PassedTests:  dbSuite.PassedSpecs,
		FailedTests:  dbSuite.FailedSpecs,
		SkippedTests: dbSuite.SkippedSpecs,
		Duration:     time.Duration(dbSuite.Duration) * time.Millisecond,
		Tags:         tags,
		SpecRuns:     specRuns,
	}
}

// ConvertSpecRunToDomain converts a database SpecRun to domain model
func (c *DatabaseConverter) ConvertSpecRunToDomain(dbSpec *database.SpecRun) *domain.SpecRun {
	// Convert tags
	tags := c.ConvertDatabaseTagsToDomain(dbSpec.Tags)

	return &domain.SpecRun{
		ID:             dbSpec.ID,
		SuiteRunID:     dbSpec.SuiteRunID,
		Name:           dbSpec.SpecName,
		ClassName:      "", // Not in database model
		Status:         dbSpec.Status,
		StartTime:      dbSpec.StartTime,
		EndTime:        dbSpec.EndTime,
		Duration:       time.Duration(dbSpec.Duration) * time.Millisecond,
		ErrorMessage:   dbSpec.ErrorMessage,
		FailureMessage: "", // Not in database model
		StackTrace:     dbSpec.StackTrace,
		RetryCount:     dbSpec.RetryCount,
		IsFlaky:        dbSpec.IsFlaky,
		Tags:           tags,
	}
}

// ConvertDomainTagsToDatabase converts domain tags to database tags
func (c *DatabaseConverter) ConvertDomainTagsToDatabase(domainTags []domain.Tag) []database.Tag {
	if len(domainTags) == 0 {
		return nil
	}

	dbTags := make([]database.Tag, len(domainTags))
	for i, tag := range domainTags {
		dbTags[i] = database.Tag{
			BaseModel: database.BaseModel{
				ID: tag.ID,
			},
			Name:     tag.Name,
			Category: tag.Category,
			Value:    tag.Value,
		}
	}
	return dbTags
}

// ConvertDatabaseTagsToDomain converts database tags to domain tags
func (c *DatabaseConverter) ConvertDatabaseTagsToDomain(dbTags []database.Tag) []domain.Tag {
	if len(dbTags) == 0 {
		return nil
	}

	domainTags := make([]domain.Tag, len(dbTags))
	for i, tag := range dbTags {
		domainTags[i] = domain.Tag{
			ID:       tag.ID,
			Name:     tag.Name,
			Category: tag.Category,
			Value:    tag.Value,
		}
	}
	return domainTags
}
