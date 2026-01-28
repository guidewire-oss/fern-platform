package domain

// Repository defines the interface for summary data access
type Repository interface {
	// GetTestRunsBySeed retrieves all test runs for a project and seed
	GetTestRunsBySeed(projectUUID string, seed string) ([]TestRunData, error)
}
