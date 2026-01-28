package database

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestJSONMap_Value(t *testing.T) {
	tests := []struct {
		name     string
		input    JSONMap
		wantNil  bool
		wantErr  bool
	}{
		{
			name:    "nil map returns nil",
			input:   nil,
			wantNil: true,
			wantErr: false,
		},
		{
			name:    "empty map",
			input:   JSONMap{},
			wantNil: false,
			wantErr: false,
		},
		{
			name: "map with string values",
			input: JSONMap{
				"key1": "value1",
				"key2": "value2",
			},
			wantNil: false,
			wantErr: false,
		},
		{
			name: "map with mixed types",
			input: JSONMap{
				"string": "value",
				"number": 42,
				"bool":   true,
				"nested": map[string]interface{}{"key": "val"},
			},
			wantNil: false,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.input.Value()
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			if tt.wantNil {
				assert.Nil(t, got)
			} else {
				assert.NotNil(t, got)
				// Verify it's valid JSON
				if bytes, ok := got.([]byte); ok {
					var m map[string]interface{}
					err := json.Unmarshal(bytes, &m)
					assert.NoError(t, err)
				}
			}
		})
	}
}

func TestJSONMap_Scan(t *testing.T) {
	tests := []struct {
		name    string
		input   interface{}
		want    JSONMap
		wantErr bool
	}{
		{
			name:  "nil value",
			input: nil,
			want:  nil,
			wantErr: false,
		},
		{
			name:  "byte slice",
			input: []byte(`{"key":"value"}`),
			want:  JSONMap{"key": "value"},
			wantErr: false,
		},
		{
			name:  "string",
			input: `{"key":"value"}`,
			want:  JSONMap{"key": "value"},
			wantErr: false,
		},
		{
			name:  "empty object",
			input: []byte(`{}`),
			want:  JSONMap{},
			wantErr: false,
		},
		{
			name:  "complex nested object",
			input: []byte(`{"key1":"value1","nested":{"key2":"value2"},"array":[1,2,3]}`),
			want:  JSONMap{"key1": "value1", "nested": map[string]interface{}{"key2": "value2"}, "array": []interface{}{float64(1), float64(2), float64(3)}},
			wantErr: false,
		},
		{
			name:    "invalid type",
			input:   123,
			want:    nil,
			wantErr: true,
		},
		{
			name:    "invalid JSON",
			input:   []byte(`{invalid json}`),
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got JSONMap
			err := got.Scan(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.name == "invalid type" {
					assert.Equal(t, errors.New("failed to scan JSONMap: invalid type"), err)
				}
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestUserPreferences_TableName(t *testing.T) {
	up := UserPreferences{}
	assert.Equal(t, "user_preferences", up.TableName())
}

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// Auto migrate all models
	err = db.AutoMigrate(
		&BaseModel{},
		&UserPreferences{},
		&UserPreference{},
		&UserScope{},
		&TestRun{},
		&SuiteRun{},
		&SpecRun{},
		&Tag{},
		&ProjectDetails{},
		&JiraConnection{},
		&ProjectPermission{},
		&TestRunTag{},
		&FlakyTest{},
		&User{},
		&UserGroup{},
		&UserSession{},
		&ProjectAccess{},
	)
	require.NoError(t, err)

	return db
}

func TestBaseRepository_Create(t *testing.T) {
	db := setupTestDB(t)
	repo := NewBaseRepository(db)

	user := &User{
		UserID: "test-user-1",
		Email:  "test@example.com",
		Name:   "Test User",
		Role:   string(RoleUser),
	}

	err := repo.Create(user)
	assert.NoError(t, err)
	assert.NotZero(t, user.ID)
}

func TestBaseRepository_GetByID(t *testing.T) {
	db := setupTestDB(t)
	repo := NewBaseRepository(db)

	// Create a user first
	user := &User{
		UserID: "test-user-2",
		Email:  "test2@example.com",
		Name:   "Test User 2",
	}
	err := repo.Create(user)
	require.NoError(t, err)

	// Get by ID
	var retrieved User
	err = repo.GetByID(user.ID, &retrieved)
	assert.NoError(t, err)
	assert.Equal(t, user.UserID, retrieved.UserID)
	assert.Equal(t, user.Email, retrieved.Email)
	assert.Equal(t, user.Name, retrieved.Name)
}

func TestBaseRepository_GetByID_NotFound(t *testing.T) {
	db := setupTestDB(t)
	repo := NewBaseRepository(db)

	var user User
	err := repo.GetByID(99999, &user)
	assert.Error(t, err)
	assert.Equal(t, gorm.ErrRecordNotFound, err)
}

func TestBaseRepository_Update(t *testing.T) {
	db := setupTestDB(t)
	repo := NewBaseRepository(db)

	// Create a user
	user := &User{
		UserID: "test-user-3",
		Email:  "test3@example.com",
		Name:   "Test User 3",
		Role:   string(RoleUser),
	}
	err := repo.Create(user)
	require.NoError(t, err)

	// Update the user
	user.Name = "Updated Name"
	user.Role = string(RoleAdmin)
	err = repo.Update(user)
	assert.NoError(t, err)

	// Verify the update
	var retrieved User
	err = repo.GetByID(user.ID, &retrieved)
	require.NoError(t, err)
	assert.Equal(t, "Updated Name", retrieved.Name)
	assert.Equal(t, string(RoleAdmin), retrieved.Role)
}

func TestBaseRepository_Delete(t *testing.T) {
	db := setupTestDB(t)
	repo := NewBaseRepository(db)

	// Create a user
	user := &User{
		UserID: "test-user-4",
		Email:  "test4@example.com",
		Name:   "Test User 4",
	}
	err := repo.Create(user)
	require.NoError(t, err)

	// Delete the user
	err = repo.Delete(user.ID, &User{})
	assert.NoError(t, err)

	// Verify deletion (soft delete)
	var retrieved User
	err = db.Unscoped().First(&retrieved, user.ID).Error
	require.NoError(t, err)
	assert.NotNil(t, retrieved.DeletedAt.Time)
}

func TestBaseRepository_List(t *testing.T) {
	db := setupTestDB(t)
	repo := NewBaseRepository(db)

	// Create multiple users
	users := []*User{
		{UserID: "user1", Email: "user1@example.com", Name: "User 1", Role: string(RoleUser)},
		{UserID: "user2", Email: "user2@example.com", Name: "User 2", Role: string(RoleAdmin)},
		{UserID: "user3", Email: "user3@example.com", Name: "User 3", Role: string(RoleUser)},
	}
	for _, u := range users {
		err := repo.Create(u)
		require.NoError(t, err)
	}

	// List all users
	var allUsers []User
	err := repo.List(&allUsers, nil)
	assert.NoError(t, err)
	assert.Len(t, allUsers, 3)

	// List with filter
	var adminUsers []User
	err = repo.List(&adminUsers, map[string]interface{}{"role": string(RoleAdmin)})
	assert.NoError(t, err)
	assert.Len(t, adminUsers, 1)
	assert.Equal(t, "user2", adminUsers[0].UserID)
}

func TestBaseRepository_Count(t *testing.T) {
	db := setupTestDB(t)
	repo := NewBaseRepository(db)

	// Create multiple users
	users := []*User{
		{UserID: "count1", Email: "count1@example.com", Name: "Count 1", Role: string(RoleUser)},
		{UserID: "count2", Email: "count2@example.com", Name: "Count 2", Role: string(RoleAdmin)},
		{UserID: "count3", Email: "count3@example.com", Name: "Count 3", Role: string(RoleUser)},
	}
	for _, u := range users {
		err := repo.Create(u)
		require.NoError(t, err)
	}

	// Count all
	count, err := repo.Count(&User{}, nil)
	assert.NoError(t, err)
	assert.Equal(t, int64(3), count)

	// Count with filter
	count, err = repo.Count(&User{}, map[string]interface{}{"role": string(RoleUser)})
	assert.NoError(t, err)
	assert.Equal(t, int64(2), count)
}

func TestModelStructs(t *testing.T) {
	// Test that all model structs can be instantiated
	now := time.Now()

	t.Run("BaseModel", func(t *testing.T) {
		bm := BaseModel{
			ID:        1,
			CreatedAt: now,
			UpdatedAt: now,
		}
		assert.Equal(t, uint(1), bm.ID)
	})

	t.Run("UserPreferences", func(t *testing.T) {
		up := UserPreferences{
			UserID:   "test-user",
			Theme:    "dark",
			Timezone: "America/New_York",
			Language: "en",
		}
		assert.Equal(t, "test-user", up.UserID)
		assert.Equal(t, "user_preferences", up.TableName())
	})

	t.Run("TestRun", func(t *testing.T) {
		tr := TestRun{
			ProjectID:   "proj-1",
			RunID:       "run-1",
			Branch:      "main",
			CommitSHA:   "abc123",
			Status:      "running",
			StartTime:   now,
			TotalTests:  100,
			PassedTests: 90,
			FailedTests: 10,
		}
		assert.Equal(t, "proj-1", tr.ProjectID)
	})

	t.Run("SuiteRun", func(t *testing.T) {
		sr := SuiteRun{
			TestRunID:   1,
			SuiteName:   "integration",
			Status:      "passed",
			StartTime:   now,
			TotalSpecs:  50,
			PassedSpecs: 48,
		}
		assert.Equal(t, uint(1), sr.TestRunID)
	})

	t.Run("SpecRun", func(t *testing.T) {
		sr := SpecRun{
			SuiteRunID: 1,
			SpecName:   "test spec",
			Status:     "passed",
			StartTime:  now,
			Duration:   1000,
		}
		assert.Equal(t, uint(1), sr.SuiteRunID)
	})

	t.Run("Tag", func(t *testing.T) {
		tag := Tag{
			Name:        "smoke",
			Category:    "test-type",
			Value:       "smoke-test",
			Description: "Smoke tests",
		}
		assert.Equal(t, "smoke", tag.Name)
	})

	t.Run("ProjectDetails", func(t *testing.T) {
		pd := ProjectDetails{
			ProjectID:     "proj-1",
			Name:          "My Project",
			Description:   "Test project",
			DefaultBranch: "main",
			IsActive:      true,
		}
		assert.Equal(t, "proj-1", pd.ProjectID)
	})

	t.Run("UserPreference", func(t *testing.T) {
		up := UserPreference{
			UserID:   "user-1",
			Theme:    "light",
			Timezone: "UTC",
			Language: "en",
		}
		assert.Equal(t, "user-1", up.UserID)
	})

	t.Run("UserScope", func(t *testing.T) {
		us := UserScope{
			UserID:    "user-1",
			Scope:     "read:projects",
			GrantedBy: "admin",
		}
		assert.Equal(t, "user-1", us.UserID)
	})

	t.Run("JiraConnection", func(t *testing.T) {
		jc := JiraConnection{
			ProjectID:           "proj-1",
			Name:                "JIRA Prod",
			JiraURL:             "https://jira.example.com",
			AuthenticationType:  "basic",
			ProjectKey:          "PROJ",
			Username:            "user@example.com",
			EncryptedCredential: "encrypted",
			Status:              "active",
			IsActive:            true,
		}
		assert.Equal(t, "proj-1", jc.ProjectID)
	})

	t.Run("ProjectPermission", func(t *testing.T) {
		pp := ProjectPermission{
			ProjectID:  "proj-1",
			UserID:     "user-1",
			Permission: "read",
			GrantedBy:  "admin",
		}
		assert.Equal(t, "proj-1", pp.ProjectID)
	})

	t.Run("TestRunTag", func(t *testing.T) {
		trt := TestRunTag{
			TestRunID: 1,
			TagID:     2,
			CreatedAt: now,
		}
		assert.Equal(t, uint(1), trt.TestRunID)
	})

	t.Run("FlakyTest", func(t *testing.T) {
		ft := FlakyTest{
			ProjectID:       "proj-1",
			TestName:        "flaky test",
			SuiteName:       "integration",
			FlakeRate:       0.25,
			TotalExecutions: 100,
			FlakyExecutions: 25,
			LastSeenAt:      now,
			FirstSeenAt:     now,
			Status:          "active",
			Severity:        "high",
		}
		assert.Equal(t, "proj-1", ft.ProjectID)
	})

	t.Run("User", func(t *testing.T) {
		user := User{
			UserID:        "oauth-user-1",
			Email:         "user@example.com",
			Name:          "Test User",
			Role:          string(RoleUser),
			Status:        "active",
			EmailVerified: true,
		}
		assert.Equal(t, "oauth-user-1", user.UserID)
	})

	t.Run("UserGroup", func(t *testing.T) {
		ug := UserGroup{
			UserID:    "user-1",
			GroupName: "developers",
		}
		assert.Equal(t, "user-1", ug.UserID)
	})

	t.Run("UserSession", func(t *testing.T) {
		us := UserSession{
			UserID:       "user-1",
			SessionID:    "session-1",
			AccessToken:  "token",
			RefreshToken: "refresh",
			ExpiresAt:    now.Add(time.Hour),
			IsActive:     true,
			LastActivity: now,
		}
		assert.Equal(t, "user-1", us.UserID)
	})

	t.Run("ProjectAccess", func(t *testing.T) {
		pa := ProjectAccess{
			UserID:    "user-1",
			ProjectID: "proj-1",
			Role:      string(ProjectRoleEditor),
			GrantedBy: "admin",
			GrantedAt: now,
		}
		assert.Equal(t, "user-1", pa.UserID)
	})
}

func TestConstants(t *testing.T) {
	t.Run("UserRole constants", func(t *testing.T) {
		assert.Equal(t, UserRole("user"), RoleUser)
		assert.Equal(t, UserRole("admin"), RoleAdmin)
	})

	t.Run("ProjectRole constants", func(t *testing.T) {
		assert.Equal(t, ProjectRole("viewer"), ProjectRoleViewer)
		assert.Equal(t, ProjectRole("editor"), ProjectRoleEditor)
		assert.Equal(t, ProjectRole("admin"), ProjectRoleAdmin)
	})
}

func TestNewBaseRepository(t *testing.T) {
	db := setupTestDB(t)
	repo := NewBaseRepository(db)

	assert.NotNil(t, repo)
	assert.NotNil(t, repo.db)
	assert.Equal(t, db, repo.db)
}

func TestJSONMap_RoundTrip(t *testing.T) {
	// Test that we can marshal and unmarshal a JSONMap
	original := JSONMap{
		"string": "value",
		"number": float64(42),
		"bool":   true,
		"nested": map[string]interface{}{
			"key": "val",
		},
	}

	// Marshal to driver.Value
	value, err := original.Value()
	require.NoError(t, err)
	require.NotNil(t, value)

	// Unmarshal back to JSONMap
	var result JSONMap
	err = result.Scan(value)
	require.NoError(t, err)

	// Verify they match
	assert.Equal(t, original["string"], result["string"])
	assert.Equal(t, original["number"], result["number"])
	assert.Equal(t, original["bool"], result["bool"])
}

func TestBaseRepository_Integration(t *testing.T) {
	db := setupTestDB(t)
	repo := NewBaseRepository(db)

	// Create a complex entity with relationships
	tag1 := &Tag{Name: "tag1", Category: "type", Value: "smoke"}
	tag2 := &Tag{Name: "tag2", Category: "priority", Value: "high"}

	err := repo.Create(tag1)
	require.NoError(t, err)
	err = repo.Create(tag2)
	require.NoError(t, err)

	// Create a test run with metadata
	testRun := &TestRun{
		ProjectID:   "integration-proj",
		RunID:       "run-123",
		Branch:      "feature/test",
		CommitSHA:   "abc123def456",
		Status:      "completed",
		StartTime:   time.Now(),
		TotalTests:  150,
		PassedTests: 140,
		FailedTests: 10,
		Environment: "staging",
		Metadata: JSONMap{
			"ci_provider": "github",
			"build_url":   "https://example.com/build/123",
		},
	}

	err = repo.Create(testRun)
	require.NoError(t, err)
	assert.NotZero(t, testRun.ID)

	// Retrieve and verify
	var retrieved TestRun
	err = repo.GetByID(testRun.ID, &retrieved)
	require.NoError(t, err)
	assert.Equal(t, "integration-proj", retrieved.ProjectID)
	assert.Equal(t, "run-123", retrieved.RunID)
	assert.NotNil(t, retrieved.Metadata)
}
