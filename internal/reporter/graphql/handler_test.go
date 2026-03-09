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

func TestNewHandler(t *testing.T) {
	logger, err := logging.NewLogger(&config.LoggingConfig{
		Level:  "error",
		Format: "json",
	})
	require.NoError(t, err)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	resolver := NewResolver(nil, nil, nil, nil, nil, db, logger)
	roleGroupNames := &RoleGroupNames{
		AdminGroup:   "admin",
		ManagerGroup: "manager",
		UserGroup:    "user",
	}

	handler := NewHandler(resolver, roleGroupNames)
	require.NotNil(t, handler)
	assert.NotNil(t, handler.server)
	assert.NotNil(t, handler.resolver)
	assert.NotNil(t, handler.roleGroupNames)
}

func TestNewHandler_NilRoleGroupNames(t *testing.T) {
	logger, err := logging.NewLogger(&config.LoggingConfig{
		Level:  "error",
		Format: "json",
	})
	require.NoError(t, err)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	resolver := NewResolver(nil, nil, nil, nil, nil, db, logger)
	handler := NewHandler(resolver, nil)
	require.NotNil(t, handler)
	assert.Nil(t, handler.roleGroupNames)
}
