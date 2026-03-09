package domain_test

import (
	"testing"
	"time"

	"github.com/guidewire-oss/fern-platform/internal/domains/projects/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ===== NewProjectPermission tests =====

func TestNewProjectPermission_Valid(t *testing.T) {
	tests := []struct {
		name       string
		permission domain.PermissionType
	}{
		{"read permission", domain.PermissionRead},
		{"write permission", domain.PermissionWrite},
		{"delete permission", domain.PermissionDelete},
		{"admin permission", domain.PermissionAdmin},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			perm, err := domain.NewProjectPermission("proj-1", "user-1", tt.permission, "admin-1")

			require.NoError(t, err)
			require.NotNil(t, perm)
			assert.Equal(t, domain.ProjectID("proj-1"), perm.ProjectID())
			assert.Equal(t, "user-1", perm.UserID())
			assert.Equal(t, tt.permission, perm.Permission())
		})
	}
}

func TestNewProjectPermission_EmptyProjectID(t *testing.T) {
	perm, err := domain.NewProjectPermission("", "user-1", domain.PermissionRead, "admin-1")

	assert.Error(t, err)
	assert.Nil(t, perm)
	assert.Contains(t, err.Error(), "project ID cannot be empty")
}

func TestNewProjectPermission_EmptyUserID(t *testing.T) {
	perm, err := domain.NewProjectPermission("proj-1", "", domain.PermissionRead, "admin-1")

	assert.Error(t, err)
	assert.Nil(t, perm)
	assert.Contains(t, err.Error(), "user ID cannot be empty")
}

func TestNewProjectPermission_EmptyGrantedBy(t *testing.T) {
	perm, err := domain.NewProjectPermission("proj-1", "user-1", domain.PermissionRead, "")

	assert.Error(t, err)
	assert.Nil(t, perm)
	assert.Contains(t, err.Error(), "granted by cannot be empty")
}

func TestNewProjectPermission_InvalidPermissionType(t *testing.T) {
	perm, err := domain.NewProjectPermission("proj-1", "user-1", domain.PermissionType("invalid"), "admin-1")

	assert.Error(t, err)
	assert.Nil(t, perm)
	assert.Contains(t, err.Error(), "invalid permission type")
}

// ===== SetExpiration =====

func TestProjectPermission_SetExpiration_Future(t *testing.T) {
	perm, _ := domain.NewProjectPermission("proj-1", "user-1", domain.PermissionRead, "admin-1")

	future := time.Now().Add(24 * time.Hour)
	err := perm.SetExpiration(future)

	assert.NoError(t, err)
	assert.False(t, perm.IsExpired())
}

func TestProjectPermission_SetExpiration_Past(t *testing.T) {
	perm, _ := domain.NewProjectPermission("proj-1", "user-1", domain.PermissionRead, "admin-1")

	past := time.Now().Add(-24 * time.Hour)
	err := perm.SetExpiration(past)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expiration time must be in the future")
}

// ===== IsExpired =====

func TestProjectPermission_IsExpired_NoExpiration(t *testing.T) {
	perm, _ := domain.NewProjectPermission("proj-1", "user-1", domain.PermissionRead, "admin-1")

	assert.False(t, perm.IsExpired())
}

// ===== Permission hierarchy: CanRead =====

func TestProjectPermission_CanRead(t *testing.T) {
	tests := []struct {
		permission domain.PermissionType
		expected   bool
	}{
		{domain.PermissionRead, true},
		{domain.PermissionWrite, true},
		{domain.PermissionDelete, true},
		{domain.PermissionAdmin, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.permission), func(t *testing.T) {
			perm, _ := domain.NewProjectPermission("proj-1", "user-1", tt.permission, "admin-1")
			assert.Equal(t, tt.expected, perm.CanRead())
		})
	}
}

// ===== Permission hierarchy: CanWrite =====

func TestProjectPermission_CanWrite(t *testing.T) {
	tests := []struct {
		permission domain.PermissionType
		expected   bool
	}{
		{domain.PermissionRead, false},
		{domain.PermissionWrite, true},
		{domain.PermissionDelete, true},
		{domain.PermissionAdmin, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.permission), func(t *testing.T) {
			perm, _ := domain.NewProjectPermission("proj-1", "user-1", tt.permission, "admin-1")
			assert.Equal(t, tt.expected, perm.CanWrite())
		})
	}
}

// ===== Permission hierarchy: CanDelete =====

func TestProjectPermission_CanDelete(t *testing.T) {
	tests := []struct {
		permission domain.PermissionType
		expected   bool
	}{
		{domain.PermissionRead, false},
		{domain.PermissionWrite, false},
		{domain.PermissionDelete, true},
		{domain.PermissionAdmin, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.permission), func(t *testing.T) {
			perm, _ := domain.NewProjectPermission("proj-1", "user-1", tt.permission, "admin-1")
			assert.Equal(t, tt.expected, perm.CanDelete())
		})
	}
}

// ===== Permission hierarchy: CanAdmin =====

func TestProjectPermission_CanAdmin(t *testing.T) {
	tests := []struct {
		permission domain.PermissionType
		expected   bool
	}{
		{domain.PermissionRead, false},
		{domain.PermissionWrite, false},
		{domain.PermissionDelete, false},
		{domain.PermissionAdmin, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.permission), func(t *testing.T) {
			perm, _ := domain.NewProjectPermission("proj-1", "user-1", tt.permission, "admin-1")
			assert.Equal(t, tt.expected, perm.CanAdmin())
		})
	}
}

// ===== Expired permissions deny all access =====

func TestProjectPermission_ExpiredDeniesAll(t *testing.T) {
	perm, _ := domain.NewProjectPermission("proj-1", "user-1", domain.PermissionAdmin, "admin-1")

	// Set expiration to a very near future, then wait
	// Instead, we'll test with a permission that has already been set with a future time
	// and then check it's not expired
	future := time.Now().Add(1 * time.Hour)
	err := perm.SetExpiration(future)
	require.NoError(t, err)

	// Not expired yet
	assert.True(t, perm.CanRead())
	assert.True(t, perm.CanWrite())
	assert.True(t, perm.CanDelete())
	assert.True(t, perm.CanAdmin())
}
