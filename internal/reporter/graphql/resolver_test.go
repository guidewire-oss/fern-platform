package graphql

import (
	"testing"

	"github.com/guidewire-oss/fern-platform/pkg/config"
	"github.com/guidewire-oss/fern-platform/pkg/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestNewResolver(t *testing.T) {
	logger, err := logging.NewLogger(&config.LoggingConfig{
		Level:  "error",
		Format: "json",
	})
	require.NoError(t, err)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	resolver := NewResolver(nil, nil, nil, nil, nil, db, logger)
	require.NotNil(t, resolver)
	assert.NotNil(t, resolver.db)
	assert.NotNil(t, resolver.logger)
	assert.NotNil(t, resolver.loaders)
}

func TestNewResolver_AllNilServices(t *testing.T) {
	logger, err := logging.NewLogger(&config.LoggingConfig{
		Level:  "error",
		Format: "json",
	})
	require.NoError(t, err)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	resolver := NewResolver(nil, nil, nil, nil, nil, db, logger)
	require.NotNil(t, resolver)
	assert.Nil(t, resolver.testingService)
	assert.Nil(t, resolver.projectService)
	assert.Nil(t, resolver.tagService)
	assert.Nil(t, resolver.flakyDetectionService)
	assert.Nil(t, resolver.jiraConnectionService)
}
