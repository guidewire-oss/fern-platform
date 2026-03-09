package domain_test

import (
	"testing"

	"github.com/guidewire-oss/fern-platform/internal/domains/projects/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ===== NewProject tests =====

func TestNewProject_Valid(t *testing.T) {
	project, err := domain.NewProject("proj-1", "My Project", "engineering")

	require.NoError(t, err)
	require.NotNil(t, project)
	assert.Equal(t, domain.ProjectID("proj-1"), project.ProjectID())
	assert.Equal(t, "My Project", project.Name())
	assert.Equal(t, domain.Team("engineering"), project.Team())
	assert.True(t, project.IsActive())
}

func TestNewProject_EmptyProjectID(t *testing.T) {
	project, err := domain.NewProject("", "My Project", "engineering")

	assert.Error(t, err)
	assert.Nil(t, project)
	assert.Contains(t, err.Error(), "project ID cannot be empty")
}

func TestNewProject_EmptyName(t *testing.T) {
	project, err := domain.NewProject("proj-1", "", "engineering")

	assert.Error(t, err)
	assert.Nil(t, project)
	assert.Contains(t, err.Error(), "project name cannot be empty")
}

func TestNewProject_EmptyTeam(t *testing.T) {
	project, err := domain.NewProject("proj-1", "My Project", "")

	assert.Error(t, err)
	assert.Nil(t, project)
	assert.Contains(t, err.Error(), "team cannot be empty")
}

func TestNewProject_DefaultBranch(t *testing.T) {
	project, err := domain.NewProject("proj-1", "My Project", "team-a")
	require.NoError(t, err)

	snapshot := project.ToSnapshot()
	assert.Equal(t, "main", snapshot.DefaultBranch)
}

// ===== UpdateName =====

func TestProject_UpdateName_Valid(t *testing.T) {
	project, _ := domain.NewProject("proj-1", "Old Name", "team-a")

	err := project.UpdateName("New Name")

	assert.NoError(t, err)
	assert.Equal(t, "New Name", project.Name())
}

func TestProject_UpdateName_Empty(t *testing.T) {
	project, _ := domain.NewProject("proj-1", "Old Name", "team-a")

	err := project.UpdateName("")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "project name cannot be empty")
	assert.Equal(t, "Old Name", project.Name()) // unchanged
}

// ===== UpdateDescription =====

func TestProject_UpdateDescription(t *testing.T) {
	project, _ := domain.NewProject("proj-1", "My Project", "team-a")

	project.UpdateDescription("A detailed description")

	snapshot := project.ToSnapshot()
	assert.Equal(t, "A detailed description", snapshot.Description)
}

// ===== UpdateRepository =====

func TestProject_UpdateRepository(t *testing.T) {
	project, _ := domain.NewProject("proj-1", "My Project", "team-a")

	project.UpdateRepository("https://github.com/org/repo")

	snapshot := project.ToSnapshot()
	assert.Equal(t, "https://github.com/org/repo", snapshot.Repository)
}

// ===== UpdateDefaultBranch =====

func TestProject_UpdateDefaultBranch_Valid(t *testing.T) {
	project, _ := domain.NewProject("proj-1", "My Project", "team-a")

	err := project.UpdateDefaultBranch("develop")

	assert.NoError(t, err)
	snapshot := project.ToSnapshot()
	assert.Equal(t, "develop", snapshot.DefaultBranch)
}

func TestProject_UpdateDefaultBranch_Empty(t *testing.T) {
	project, _ := domain.NewProject("proj-1", "My Project", "team-a")

	err := project.UpdateDefaultBranch("")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "default branch cannot be empty")
	snapshot := project.ToSnapshot()
	assert.Equal(t, "main", snapshot.DefaultBranch) // unchanged
}

// ===== UpdateTeam =====

func TestProject_UpdateTeam_Valid(t *testing.T) {
	project, _ := domain.NewProject("proj-1", "My Project", "team-a")

	err := project.UpdateTeam("team-b")

	assert.NoError(t, err)
	assert.Equal(t, domain.Team("team-b"), project.Team())
}

func TestProject_UpdateTeam_Empty(t *testing.T) {
	project, _ := domain.NewProject("proj-1", "My Project", "team-a")

	err := project.UpdateTeam("")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "team cannot be empty")
	assert.Equal(t, domain.Team("team-a"), project.Team()) // unchanged
}

// ===== Activate / Deactivate =====

func TestProject_Activate_Deactivate(t *testing.T) {
	project, _ := domain.NewProject("proj-1", "My Project", "team-a")
	assert.True(t, project.IsActive())

	project.Deactivate()
	assert.False(t, project.IsActive())

	project.Activate()
	assert.True(t, project.IsActive())
}

// ===== Settings =====

func TestProject_Settings(t *testing.T) {
	project, _ := domain.NewProject("proj-1", "My Project", "team-a")

	project.SetSetting("feature_x", true)
	project.SetSetting("max_retries", 3)

	val, exists := project.GetSetting("feature_x")
	assert.True(t, exists)
	assert.Equal(t, true, val)

	val, exists = project.GetSetting("max_retries")
	assert.True(t, exists)
	assert.Equal(t, 3, val)

	_, exists = project.GetSetting("nonexistent")
	assert.False(t, exists)
}

// ===== SetID / ID =====

func TestProject_SetID(t *testing.T) {
	project, _ := domain.NewProject("proj-1", "My Project", "team-a")
	assert.Equal(t, uint(0), project.ID())

	project.SetID(42)
	assert.Equal(t, uint(42), project.ID())
}

// ===== ToSnapshot =====

func TestProject_ToSnapshot(t *testing.T) {
	project, _ := domain.NewProject("proj-1", "My Project", "team-a")
	project.SetID(10)
	project.UpdateDescription("desc")
	project.UpdateRepository("https://github.com/org/repo")
	project.UpdateDefaultBranch("develop")
	project.SetSetting("key1", "val1")

	snapshot := project.ToSnapshot()

	assert.Equal(t, uint(10), snapshot.ID)
	assert.Equal(t, domain.ProjectID("proj-1"), snapshot.ProjectID)
	assert.Equal(t, "My Project", snapshot.Name)
	assert.Equal(t, "desc", snapshot.Description)
	assert.Equal(t, "https://github.com/org/repo", snapshot.Repository)
	assert.Equal(t, "develop", snapshot.DefaultBranch)
	assert.Equal(t, domain.Team("team-a"), snapshot.Team)
	assert.True(t, snapshot.IsActive)
	assert.Equal(t, "val1", snapshot.Settings["key1"])
	assert.False(t, snapshot.CreatedAt.IsZero())
	assert.False(t, snapshot.UpdatedAt.IsZero())
}
