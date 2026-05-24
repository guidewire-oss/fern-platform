package domain

import "context"

// TagRepository defines the interface for tag persistence
type TagRepository interface {
	// Save persists a tag
	Save(ctx context.Context, tag *Tag) error

	// FindByID retrieves a tag by ID
	FindByID(ctx context.Context, id TagID) (*Tag, error)

	// FindByName retrieves a tag by name
	FindByName(ctx context.Context, name string) (*Tag, error)

	// FindAll retrieves all tags
	FindAll(ctx context.Context) ([]*Tag, error)

	// Delete removes a tag
	Delete(ctx context.Context, id TagID) error

	// AssignToTestRun assigns tags to a test run
	AssignToTestRun(ctx context.Context, testRunID string, tagIDs []TagID) error

	// UsageCounts returns the count of test_run_tags rows for every tag
	// in the system. Result is keyed by string tag ID. Tags that have
	// never been used are omitted from the map (callers default missing
	// keys to 0). Single SQL aggregate — used by both the popular-tags
	// endpoint and the usage-stats endpoint.
	UsageCounts(ctx context.Context) (map[string]int, error)
}
