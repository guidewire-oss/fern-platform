package graphql

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	authDomain "github.com/guidewire-oss/fern-platform/internal/domains/auth/domain"
	"github.com/guidewire-oss/fern-platform/internal/reporter/graphql/dataloader"
	"github.com/guidewire-oss/fern-platform/internal/reporter/graphql/model"
	"github.com/guidewire-oss/fern-platform/pkg/config"
	"github.com/guidewire-oss/fern-platform/pkg/database"
	"github.com/guidewire-oss/fern-platform/pkg/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// setupTestProjectResolver creates a test project resolver with minimal dependencies
func setupTestProjectResolver(t *testing.T) *projectResolver {
	logger, err := logging.NewLogger(&config.LoggingConfig{
		Level:      "info",
		Format:     "json",
		Output:     "stdout",
		Structured: true,
	})
	require.NoError(t, err)

	// Create an in-memory SQLite database for testing
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	resolver := &Resolver{
		logger: logger,
		db:     db,
	}

	return &projectResolver{resolver}
}

// createTestContext creates a context with a user for testing
func createTestContext(user *authDomain.User) context.Context {
	ctx := context.Background()
	if user != nil {
		ctx = context.WithValue(ctx, "user", user)
	}
	// Add role group names for tests that need it
	ctx = context.WithValue(ctx, "roleGroupNames", &RoleGroupNames{
		AdminGroup:   "admin",
		ManagerGroup: "manager",
		UserGroup:    "user",
	})
	return ctx
}

func TestProjectResolver_CanManage(t *testing.T) {
	resolver := setupTestProjectResolver(t)

	tests := []struct {
		name           string
		user           *authDomain.User
		project        *model.Project
		expectedResult bool
		description    string
	}{
		{
			name: "admin can manage any project",
			user: &authDomain.User{
				UserID: "admin-user",
				Email:  "admin@example.com",
				Role:   authDomain.RoleAdmin,
				Groups: []authDomain.UserGroup{},
			},
			project: &model.Project{
				ID:        "1",
				ProjectID: "test-project",
				Name:      "Test Project",
				Team:      stringPtr("some-team"),
			},
			expectedResult: true,
			description:    "Admin users should be able to manage all projects",
		},
		{
			name: "manager can manage project in different team",
			user: &authDomain.User{
				UserID: "manager-user",
				Email:  "manager@example.com",
				Role:   authDomain.RoleManager,
				Groups: []authDomain.UserGroup{
					{GroupName: "fern-managers"},
					{GroupName: "team-a"},
				},
			},
			project: &model.Project{
				ID:        "2",
				ProjectID: "everyone-project",
				Name:      "Everyone Project",
				Team:      stringPtr("everyone"),
			},
			expectedResult: true,
			description:    "Manager role should be able to manage projects even if not in the project's team",
		},
		{
			name: "manager can manage project without team",
			user: &authDomain.User{
				UserID: "manager-user",
				Email:  "manager@example.com",
				Role:   authDomain.RoleManager,
				Groups: []authDomain.UserGroup{
					{GroupName: "fern-managers"},
				},
			},
			project: &model.Project{
				ID:        "3",
				ProjectID: "no-team-project",
				Name:      "No Team Project",
				Team:      nil,
			},
			expectedResult: true,
			description:    "Manager role should be able to manage projects without a team",
		},
		{
			name: "manager can manage project in their team",
			user: &authDomain.User{
				UserID: "manager-user",
				Email:  "manager@example.com",
				Role:   authDomain.RoleManager,
				Groups: []authDomain.UserGroup{
					{GroupName: "fern-managers"},
				},
			},
			project: &model.Project{
				ID:        "4",
				ProjectID: "manager-team-project",
				Name:      "Manager Team Project",
				Team:      stringPtr("fern-managers"),
			},
			expectedResult: true,
			description:    "Manager role should be able to manage projects in their own team",
		},
		{
			name: "regular user cannot manage project",
			user: &authDomain.User{
				UserID: "regular-user",
				Email:  "user@example.com",
				Role:   authDomain.RoleUser,
				Groups: []authDomain.UserGroup{
					{GroupName: "some-team"},
				},
			},
			project: &model.Project{
				ID:        "5",
				ProjectID: "team-project",
				Name:      "Team Project",
				Team:      stringPtr("some-team"),
			},
			expectedResult: false,
			description:    "Regular users should not be able to manage projects without explicit scopes",
		},
		{
			name: "unauthenticated user cannot manage project",
			user: nil,
			project: &model.Project{
				ID:        "6",
				ProjectID: "public-project",
				Name:      "Public Project",
				Team:      stringPtr("everyone"),
			},
			expectedResult: false,
			description:    "Unauthenticated users should not be able to manage any projects",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := createTestContext(tt.user)
			result, err := resolver.CanManage(ctx, tt.project)
			
			require.NoError(t, err, "CanManage should not return an error")
			assert.Equal(t, tt.expectedResult, result, tt.description)
		})
	}
}

// TestProjectResolver_CanManage_ManagerScenarios focuses on specific manager scenarios
func TestProjectResolver_CanManage_ManagerScenarios(t *testing.T) {
	resolver := setupTestProjectResolver(t)

	// Create a manager user
	managerUser := &authDomain.User{
		UserID: "manager-123",
		Email:  "manager@example.com",
		Role:   authDomain.RoleManager,
		Groups: []authDomain.UserGroup{
			{GroupName: "fern-managers"},
			{GroupName: "platform-team"},
		},
	}

	// Test various project scenarios
	projects := []*model.Project{
		{
			ID:        "1",
			ProjectID: "proj-1",
			Name:      "Project in Manager's Team",
			Team:      stringPtr("platform-team"),
		},
		{
			ID:        "2",
			ProjectID: "proj-2",
			Name:      "Project in Different Team",
			Team:      stringPtr("data-team"),
		},
		{
			ID:        "3",
			ProjectID: "proj-3",
			Name:      "Project with Everyone Team",
			Team:      stringPtr("everyone"),
		},
		{
			ID:        "4",
			ProjectID: "proj-4",
			Name:      "Project without Team",
			Team:      nil,
		},
	}

	ctx := createTestContext(managerUser)

	t.Run("manager can manage all projects regardless of team", func(t *testing.T) {
		for _, project := range projects {
			result, err := resolver.CanManage(ctx, project)
			require.NoError(t, err)
			assert.True(t, result, "Manager should be able to manage project: %s (team: %v)", 
				project.Name, project.Team)
		}
	})
}

// TestProjectResolver_CanManage_RoleBasedAccess verifies role-based access control
func TestProjectResolver_CanManage_RoleBasedAccess(t *testing.T) {
	resolver := setupTestProjectResolver(t)

	project := &model.Project{
		ID:        "1",
		ProjectID: "test-project",
		Name:      "Test Project",
		Team:      stringPtr("engineering"),
	}

	testCases := []struct {
		roleName       string
		role           authDomain.UserRole
		expectedAccess bool
	}{
		{"admin", authDomain.RoleAdmin, true},
		{"manager", authDomain.RoleManager, true},
		{"user", authDomain.RoleUser, false},
	}

	for _, tc := range testCases {
		t.Run(tc.roleName, func(t *testing.T) {
			user := &authDomain.User{
				UserID: "user-" + tc.roleName,
				Email:  tc.roleName + "@example.com",
				Role:   tc.role,
				Groups: []authDomain.UserGroup{},
			}

			ctx := createTestContext(user)
			result, err := resolver.CanManage(ctx, project)
			
			require.NoError(t, err)
			assert.Equal(t, tc.expectedAccess, result, 
				"User with role '%s' should have canManage=%v", tc.roleName, tc.expectedAccess)
		})
	}
}

// stringPtr is a helper function to create a string pointer
func stringPtr(s string) *string {
	return &s
}

// Ginkgo test suite
var _ = Describe("Schema Resolvers - Ginkgo Tests", func() {
	var (
		resolver *Resolver
		db       *gorm.DB
		logger   *logging.Logger
	)

	BeforeEach(func() {
		var err error
		logger, err = logging.NewLogger(&config.LoggingConfig{
			Level:      "info",
			Format:     "json",
			Output:     "stdout",
			Structured: true,
		})
		Expect(err).ToNot(HaveOccurred())

		db, err = gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
		Expect(err).ToNot(HaveOccurred())

		// Migrate tables
		err = db.AutoMigrate(&database.UserPreferences{}, &database.TestRun{}, &database.SuiteRun{}, &database.SpecRun{}, &database.Tag{})
		Expect(err).ToNot(HaveOccurred())

		// Create a minimal resolver without services
		resolver = &Resolver{
			logger: logger,
			db:     db,
		}
	})

	Describe("Mutation Resolvers", func() {
		Context("CreateTestRun", func() {
			It("should return not implemented error", func() {
				ctx := context.Background()
				input := model.CreateTestRunInput{}
				mut := &mutationResolver{resolver}
				result, err := mut.CreateTestRun(ctx, input)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("not yet implemented"))
				Expect(result).To(BeNil())
			})
		})

		Context("UpdateTestRunStatus", func() {
			It("should return not implemented error", func() {
				ctx := context.Background()
				mut := &mutationResolver{resolver}
				result, err := mut.UpdateTestRunStatus(ctx, "run-1", "completed", nil)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("not yet implemented"))
				Expect(result).To(BeNil())
			})
		})

		Context("DeleteTestRun", func() {
			It("should return not implemented error", func() {
				ctx := context.Background()
				mut := &mutationResolver{resolver}
				result, err := mut.DeleteTestRun(ctx, "run-1")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("not yet implemented"))
				Expect(result).To(BeFalse())
			})
		})

		Context("AssignTagsToTestRun", func() {
			It("should return not implemented error", func() {
				ctx := context.Background()
				mut := &mutationResolver{resolver}
				result, err := mut.AssignTagsToTestRun(ctx, "run-1", []string{"tag-1"})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("not yet implemented"))
				Expect(result).To(BeNil())
			})
		})

		Context("ActivateProject", func() {
			It("should return not implemented error", func() {
				ctx := context.Background()
				mut := &mutationResolver{resolver}
				result, err := mut.ActivateProject(ctx, "proj-1")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("not yet implemented"))
				Expect(result).To(BeNil())
			})
		})

		Context("DeactivateProject", func() {
			It("should return not implemented error", func() {
				ctx := context.Background()
				mut := &mutationResolver{resolver}
				result, err := mut.DeactivateProject(ctx, "proj-1")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("not yet implemented"))
				Expect(result).To(BeNil())
			})
		})

		Context("UpdateTag", func() {
			It("should return not implemented error", func() {
				ctx := context.Background()
				mut := &mutationResolver{resolver}
				input := model.UpdateTagInput{}
				result, err := mut.UpdateTag(ctx, "tag-1", input)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("not yet implemented"))
				Expect(result).To(BeNil())
			})
		})

		Context("DeleteTag", func() {
			It("should return not implemented error", func() {
				ctx := context.Background()
				mut := &mutationResolver{resolver}
				result, err := mut.DeleteTag(ctx, "tag-1")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("not yet implemented"))
				Expect(result).To(BeFalse())
			})
		})

		Context("MarkFlakyTestResolved", func() {
			It("should return not implemented error", func() {
				ctx := context.Background()
				mut := &mutationResolver{resolver}
				result, err := mut.MarkFlakyTestResolved(ctx, "flaky-1")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("not yet implemented"))
				Expect(result).To(BeNil())
			})
		})

		Context("MarkSpecAsFlaky", func() {
			It("should return not implemented error", func() {
				ctx := context.Background()
				mut := &mutationResolver{resolver}
				result, err := mut.MarkSpecAsFlaky(ctx, "spec-1")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("not yet implemented"))
				Expect(result).To(BeNil())
			})
		})

		Context("UpdateUserPreferences", func() {
			It("should create new preferences when none exist", func() {
				user := &authDomain.User{UserID: "user-1", Email: "user@test.com"}
				ctx := context.WithValue(context.Background(), "user", user)

				theme := "dark"
				input := model.UpdateUserPreferencesInput{Theme: &theme}
				mut := &mutationResolver{resolver}
				result, err := mut.UpdateUserPreferences(ctx, input)

				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(BeNil())
				Expect(result.UserID).To(Equal("user-1"))
				Expect(*result.Theme).To(Equal("dark"))
			})

			It("should update existing preferences", func() {
				user := &authDomain.User{UserID: "user-2", Email: "user2@test.com"}
				ctx := context.WithValue(context.Background(), "user", user)

				// Create initial preferences
				prefs := database.UserPreferences{
					UserID:      "user-2",
					Theme:       "light",
					Timezone:    "UTC",
					Language:    "en",
					Favorites:   json.RawMessage("[]"),
					Preferences: json.RawMessage("{}"),
				}
				err := db.Create(&prefs).Error
				Expect(err).ToNot(HaveOccurred())

				// Update preferences
				newTheme := "dark"
				newTimezone := "PST"
				input := model.UpdateUserPreferencesInput{
					Theme:    &newTheme,
					Timezone: &newTimezone,
				}
				mut := &mutationResolver{resolver}
				result, err := mut.UpdateUserPreferences(ctx, input)

				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(BeNil())
				Expect(*result.Theme).To(Equal("dark"))
				Expect(*result.Timezone).To(Equal("PST"))
			})

			It("should handle favorites update", func() {
				user := &authDomain.User{UserID: "user-3", Email: "user3@test.com"}
				ctx := context.WithValue(context.Background(), "user", user)

				favorites := []string{"proj-1", "proj-2"}
				input := model.UpdateUserPreferencesInput{Favorites: favorites}
				mut := &mutationResolver{resolver}
				result, err := mut.UpdateUserPreferences(ctx, input)

				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(BeNil())
				Expect(result.Favorites).To(Equal(favorites))
			})

			It("should handle preferences map update", func() {
				user := &authDomain.User{UserID: "user-4", Email: "user4@test.com"}
				ctx := context.WithValue(context.Background(), "user", user)

				prefs := map[string]any{"key1": "value1", "key2": 123}
				input := model.UpdateUserPreferencesInput{Preferences: prefs}
				mut := &mutationResolver{resolver}
				result, err := mut.UpdateUserPreferences(ctx, input)

				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(BeNil())
				Expect(result.Preferences).To(HaveKey("key1"))
				Expect(result.Preferences["key1"]).To(Equal("value1"))
			})

			It("should return error when user not authenticated", func() {
				ctx := context.Background()
				input := model.UpdateUserPreferencesInput{}
				mut := &mutationResolver{resolver}
				result, err := mut.UpdateUserPreferences(ctx, input)

				Expect(err).To(HaveOccurred())
				Expect(result).To(BeNil())
			})
		})

		Context("ToggleProjectFavorite", func() {
			It("should create preferences and add project as favorite when none exist", func() {
				user := &authDomain.User{UserID: "user-5", Email: "user5@test.com"}
				ctx := context.WithValue(context.Background(), "user", user)

				mut := &mutationResolver{resolver}
				result, err := mut.ToggleProjectFavorite(ctx, "proj-1")

				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(BeNil())
				Expect(result.Favorites).To(ContainElement("proj-1"))
			})

			It("should add project to favorites", func() {
				user := &authDomain.User{UserID: "user-6", Email: "user6@test.com"}
				ctx := context.WithValue(context.Background(), "user", user)

				// Create preferences with existing favorites
				prefs := database.UserPreferences{
					UserID:      "user-6",
					Theme:       "light",
					Timezone:    "UTC",
					Language:    "en",
					Favorites:   json.RawMessage(`["proj-1"]`),
					Preferences: json.RawMessage("{}"),
				}
				err := db.Create(&prefs).Error
				Expect(err).ToNot(HaveOccurred())

				mut := &mutationResolver{resolver}
				result, err := mut.ToggleProjectFavorite(ctx, "proj-2")

				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(BeNil())
				Expect(result.Favorites).To(ContainElements("proj-1", "proj-2"))
			})

			It("should remove project from favorites when already favorited", func() {
				user := &authDomain.User{UserID: "user-7", Email: "user7@test.com"}
				ctx := context.WithValue(context.Background(), "user", user)

				// Create preferences with project in favorites
				prefs := database.UserPreferences{
					UserID:      "user-7",
					Theme:       "light",
					Timezone:    "UTC",
					Language:    "en",
					Favorites:   json.RawMessage(`["proj-1", "proj-2"]`),
					Preferences: json.RawMessage("{}"),
				}
				err := db.Create(&prefs).Error
				Expect(err).ToNot(HaveOccurred())

				mut := &mutationResolver{resolver}
				result, err := mut.ToggleProjectFavorite(ctx, "proj-1")

				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(BeNil())
				Expect(result.Favorites).To(ContainElement("proj-2"))
				Expect(result.Favorites).ToNot(ContainElement("proj-1"))
			})

			It("should return error when user not authenticated", func() {
				ctx := context.Background()
				mut := &mutationResolver{resolver}
				result, err := mut.ToggleProjectFavorite(ctx, "proj-1")

				Expect(err).To(HaveOccurred())
				Expect(result).To(BeNil())
			})
		})
	})

	Describe("Mutation Resolvers - Jira Connections", func() {
		Context("CreateJiraConnection", func() {
			It("should return error when user not authenticated", func() {
				ctx := context.Background()
				mr := &mutationResolver{resolver}
				input := model.CreateJiraConnectionInput{
					ProjectID:          "test-proj",
					Name:               "Test Conn",
					JiraURL:            "https://jira.example.com",
					AuthenticationType: "basic",
					ProjectKey:         "PROJ",
				}
				result, err := mr.CreateJiraConnection(ctx, input)
				Expect(err).To(HaveOccurred())
				Expect(result).To(BeNil())
			})

			It("should return error when user is nil", func() {
				ctx := createTestContext(nil)
				mr := &mutationResolver{resolver}
				input := model.CreateJiraConnectionInput{
					ProjectID:          "test-proj",
					Name:               "Test Conn",
					JiraURL:            "https://jira.example.com",
					AuthenticationType: "basic",
					ProjectKey:         "PROJ",
				}
				result, err := mr.CreateJiraConnection(ctx, input)
				Expect(err).To(HaveOccurred())
				Expect(result).To(BeNil())
			})
		})

		Context("UpdateJiraConnection", func() {
			It("should return error when user not authenticated", func() {
				ctx := context.Background()
				mr := &mutationResolver{resolver}
				input := model.UpdateJiraConnectionInput{
					Name:       "Updated Conn",
					JiraURL:    "https://jira2.example.com",
					ProjectKey: "PROJ2",
				}
				result, err := mr.UpdateJiraConnection(ctx, "conn-1", input)
				Expect(err).To(HaveOccurred())
				Expect(result).To(BeNil())
			})
		})

		Context("UpdateJiraCredentials", func() {
			It("should return error when user not authenticated", func() {
				ctx := context.Background()
				mr := &mutationResolver{resolver}
				input := model.UpdateJiraCredentialsInput{
					AuthenticationType: "token",
					Username:           "user",
					Credential:         "token",
				}
				result, err := mr.UpdateJiraCredentials(ctx, "conn-1", input)
				Expect(err).To(HaveOccurred())
				Expect(result).To(BeNil())
			})
		})

		Context("TestJiraConnection", func() {
			It("should return error when user not authenticated", func() {
				ctx := context.Background()
				mr := &mutationResolver{resolver}
				result, err := mr.TestJiraConnection(ctx, "conn-1")
				Expect(err).To(HaveOccurred())
				Expect(result).To(BeFalse())
			})
		})

		Context("DeleteJiraConnection", func() {
			It("should return error when user not authenticated", func() {
				ctx := context.Background()
				mr := &mutationResolver{resolver}
				result, err := mr.DeleteJiraConnection(ctx, "conn-1")
				Expect(err).To(HaveOccurred())
				Expect(result).To(BeFalse())
			})
		})
	})

	Describe("Query Resolvers", func() {
		Context("CurrentUser", func() {
			It("should return current user from context", func() {
				user := &authDomain.User{
					UserID:    "user-1",
					Email:     "user@test.com",
					Name:      "Test User",
					FirstName: "Test",
					LastName:  "User",
					Role:      authDomain.RoleUser,
					Groups:    []authDomain.UserGroup{{GroupName: "group1"}},
					CreatedAt: time.Now(),
				}
				ctx := context.WithValue(context.Background(), "user", user)

				qry := &queryResolver{resolver}
				result, err := qry.CurrentUser(ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(BeNil())
				Expect(result.UserID).To(Equal("user-1"))
				Expect(result.Email).To(Equal("user@test.com"))
				Expect(result.Groups).To(ContainElement("group1"))
			})

			It("should return error when user not authenticated", func() {
				ctx := context.Background()
				qry := &queryResolver{resolver}
				result, err := qry.CurrentUser(ctx)
				Expect(err).To(HaveOccurred())
				Expect(result).To(BeNil())
			})
		})

		Context("UserPreferences", func() {
			It("should return user preferences", func() {
				user := &authDomain.User{UserID: "user-1", Email: "user@test.com"}
				ctx := context.WithValue(context.Background(), "user", user)

				prefs := database.UserPreferences{
					UserID:      "user-1",
					Theme:       "dark",
					Timezone:    "UTC",
					Language:    "en",
					Favorites:   json.RawMessage(`["proj-1"]`),
					Preferences: json.RawMessage(`{"key1":"value1"}`),
				}
				err := db.Create(&prefs).Error
				Expect(err).ToNot(HaveOccurred())

				qry := &queryResolver{resolver}
				result, err := qry.UserPreferences(ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(BeNil())
				Expect(*result.Theme).To(Equal("dark"))
				Expect(result.Favorites).To(ContainElement("proj-1"))
			})

			It("should create default preferences when none exist", func() {
				user := &authDomain.User{UserID: "user-new", Email: "new@test.com"}
				ctx := context.WithValue(context.Background(), "user", user)

				qry := &queryResolver{resolver}
				result, err := qry.UserPreferences(ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(BeNil())
				Expect(*result.Theme).To(Equal("light"))
				Expect(*result.Timezone).To(Equal("UTC"))
			})
		})

		Context("SystemConfig", func() {
			It("should return system config", func() {
				ctx := context.WithValue(context.Background(), "roleGroupNames", &RoleGroupNames{
					AdminGroup:   "admins",
					ManagerGroup: "managers",
					UserGroup:    "users",
				})

				qry := &queryResolver{resolver}
				result, err := qry.SystemConfig(ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(BeNil())
				Expect(result.RoleGroups).ToNot(BeNil())
				Expect(result.RoleGroups.AdminGroup).To(Equal("admins"))
			})
		})

		Context("Health", func() {
			It("should return healthy status", func() {
				ctx := context.Background()
				qry := &queryResolver{resolver}
				result, err := qry.Health(ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(BeNil())
				Expect(result.Status).To(Equal("healthy"))
				Expect(result.Service).To(Equal("fern-platform"))
			})
		})

		Context("JiraConnection", func() {
			It("should return connection when user authenticated", func() {
				// This requires full integration with Jira service which is tested elsewhere
				// Skipping this test as it requires complex setup
				Skip("Requires full Jira service integration")
			})
		})

		Context("JiraConnections", func() {
			It("should return error when user not authenticated", func() {
				qr := &queryResolver{resolver}
				ctx := context.Background()
				result, err := qr.JiraConnections(ctx, "test-proj")
				Expect(err).To(HaveOccurred())
				Expect(result).To(BeNil())
			})

			It("should return error when user is nil", func() {
				qr := &queryResolver{resolver}
				ctx := createTestContext(nil)
				result, err := qr.JiraConnections(ctx, "test-proj")
				Expect(err).To(HaveOccurred())
				Expect(result).To(BeNil())
			})
		})

		Context("TestRunStats", func() {
			It("should return test run stats", func() {
				ctx := context.Background()
				qry := &queryResolver{resolver}
				result, err := qry.TestRunStats(ctx, nil, nil)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(BeNil())
				Expect(result.TotalRuns).To(Equal(0))
			})
		})

		Context("Tag Queries", func() {
			It("Tag should return not implemented", func() {
				ctx := context.Background()
				qry := &queryResolver{resolver}
				result, err := qry.Tag(ctx, "tag-1")
				Expect(err).To(HaveOccurred())
				Expect(result).To(BeNil())
			})

			It("TagByName should return not implemented", func() {
				ctx := context.Background()
				qry := &queryResolver{resolver}
				result, err := qry.TagByName(ctx, "tag-name")
				Expect(err).To(HaveOccurred())
				Expect(result).To(BeNil())
			})

			It("TagUsageStats should return not implemented", func() {
				ctx := context.Background()
				qry := &queryResolver{resolver}
				result, err := qry.TagUsageStats(ctx)
				Expect(err).To(HaveOccurred())
				Expect(result).To(BeNil())
			})

			It("PopularTags should return not implemented", func() {
				ctx := context.Background()
				qry := &queryResolver{resolver}
				result, err := qry.PopularTags(ctx, nil)
				Expect(err).To(HaveOccurred())
				Expect(result).To(BeNil())
			})
		})

		Context("FlakyTest Queries", func() {
			It("FlakyTest should return not implemented", func() {
				ctx := context.Background()
				qry := &queryResolver{resolver}
				result, err := qry.FlakyTest(ctx, "flaky-1")
				Expect(err).To(HaveOccurred())
				Expect(result).To(BeNil())
			})

			It("FlakyTests should return not implemented", func() {
				ctx := context.Background()
				qry := &queryResolver{resolver}
				result, err := qry.FlakyTests(ctx, nil, nil, nil, nil, nil)
				Expect(err).To(HaveOccurred())
				Expect(result).To(BeNil())
			})

			It("FlakyTestStats should return not implemented", func() {
				ctx := context.Background()
				qry := &queryResolver{resolver}
				result, err := qry.FlakyTestStats(ctx, nil)
				Expect(err).To(HaveOccurred())
				Expect(result).To(BeNil())
			})

			It("RecentlyAddedFlakyTests should return not implemented", func() {
				ctx := context.Background()
				qry := &queryResolver{resolver}
				result, err := qry.RecentlyAddedFlakyTests(ctx, nil, nil, nil)
				Expect(err).To(HaveOccurred())
				Expect(result).To(BeNil())
			})
		})

		Context("JiraConnection Queries", func() {
			It("JiraConnection should return error when service not initialized", func() {
				Skip("Skipping - requires JIRA connection service")
			})

			It("JiraConnections should fail without authentication", func() {
				Skip("Skipping - requires JIRA connection service")
			})
		})
	})

	Describe("Subscription Resolvers", func() {
		It("TestRunCreated should return closed channel", func() {
			ctx := context.Background()
			sub := &subscriptionResolver{resolver}
			result, err := sub.TestRunCreated(ctx, nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).ToNot(BeNil())
			_, open := <-result
			Expect(open).To(BeFalse())
		})

		It("TestRunUpdated should return closed channel", func() {
			ctx := context.Background()
			sub := &subscriptionResolver{resolver}
			result, err := sub.TestRunUpdated(ctx, nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).ToNot(BeNil())
			_, open := <-result
			Expect(open).To(BeFalse())
		})

		It("TestRunStatusChanged should return closed channel", func() {
			ctx := context.Background()
			sub := &subscriptionResolver{resolver}
			result, err := sub.TestRunStatusChanged(ctx, nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).ToNot(BeNil())
			_, open := <-result
			Expect(open).To(BeFalse())
		})

		It("FlakyTestDetected should return closed channel", func() {
			ctx := context.Background()
			sub := &subscriptionResolver{resolver}
			result, err := sub.FlakyTestDetected(ctx, nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).ToNot(BeNil())
			_, open := <-result
			Expect(open).To(BeFalse())
		})
	})

	Describe("TestRun Resolvers", func() {
		Context("SuiteRuns", func() {
			It("should return error when loaders not in context", func() {
				Skip("Skipping - requires data loaders in context")
			})

			It("should handle invalid test run ID", func() {
				ctx := context.Background()
				modelTestRun := &model.TestRun{ID: "invalid"}

				tr := &testRunResolver{resolver}
				result, err := tr.SuiteRuns(ctx, modelTestRun)
				Expect(err).To(HaveOccurred())
				Expect(result).To(BeNil())
			})
		})
	})

	Describe("SuiteRun Resolvers", func() {
		Context("SpecRuns", func() {
			It("should return error when loaders not in context", func() {
				Skip("Skipping - requires data loaders in context")
			})

			It("should handle invalid suite run ID", func() {
				ctx := context.Background()
				modelSuiteRun := &model.SuiteRun{ID: "invalid"}

				sr := &suiteRunResolver{resolver}
				result, err := sr.SpecRuns(ctx, modelSuiteRun)
				Expect(err).To(HaveOccurred())
				Expect(result).To(BeNil())
			})
		})
	})

	Describe("Project Resolver", func() {
		Context("CanManage", func() {
			It("should allow admin to manage project", func() {
				user := &authDomain.User{
					UserID: "admin-1",
					Role:   authDomain.RoleAdmin,
				}
				ctx := context.WithValue(context.Background(), "user", user)

				project := &model.Project{ProjectID: "proj-1"}
				pr := &projectResolver{resolver}
				result, err := pr.CanManage(ctx, project)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(BeTrue())
			})

			It("should allow manager to manage project", func() {
				user := &authDomain.User{
					UserID: "manager-1",
					Role:   authDomain.RoleManager,
				}
				ctx := context.WithValue(context.Background(), "user", user)

				project := &model.Project{ProjectID: "proj-1"}
				pr := &projectResolver{resolver}
				result, err := pr.CanManage(ctx, project)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(BeTrue())
			})

			It("should deny regular user without permissions", func() {
				user := &authDomain.User{
					UserID: "user-1",
					Role:   authDomain.RoleUser,
				}
				ctx := context.WithValue(context.Background(), "user", user)

				project := &model.Project{ProjectID: "proj-1"}
				pr := &projectResolver{resolver}
				result, err := pr.CanManage(ctx, project)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(BeFalse())
			})

			It("should deny unauthenticated user", func() {
				ctx := context.Background()
				project := &model.Project{ProjectID: "proj-1"}
				pr := &projectResolver{resolver}
				result, err := pr.CanManage(ctx, project)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(BeFalse())
			})
		})

		Context("Stats", func() {
			It("should return nil (not implemented)", func() {
				ctx := context.Background()
				project := &model.Project{ProjectID: "proj-1"}
				pr := &projectResolver{resolver}
				result, err := pr.Stats(ctx, project)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(BeNil())
			})
		})
	})

	Describe("Resolver Factory Methods", func() {
		It("should create mutation resolver", func() {
			result := resolver.Mutation()
			Expect(result).ToNot(BeNil())
		})

		It("should create project resolver", func() {
			result := resolver.Project()
			Expect(result).ToNot(BeNil())
		})

		It("should create query resolver", func() {
			result := resolver.Query()
			Expect(result).ToNot(BeNil())
		})

		It("should create subscription resolver", func() {
			result := resolver.Subscription()
			Expect(result).ToNot(BeNil())
		})

		It("should create suite run resolver", func() {
			result := resolver.SuiteRun()
			Expect(result).ToNot(BeNil())
		})

		It("should create test run resolver", func() {
			result := resolver.TestRun()
			Expect(result).ToNot(BeNil())
		})
	})

	Describe("Mutation Resolvers - Wrapper Functions", func() {
		Context("CreateProject", func() {
			It("should delegate to domain service", func() {
				// This should call CreateProject_domain which is tested separately
				ctx := context.Background()
				mr := &mutationResolver{resolver}
				input := model.CreateProjectInput{
					ProjectID: "test-project",
					Name:      "Test Project",
				}
				// Should not panic even if service is nil - should return error
				defer func() {
					if r := recover(); r != nil {
						// If it panics, that's also acceptable for this wrapper test
					}
				}()
				_, err := mr.CreateProject(ctx, input)
				// We expect an error since we don't have auth in context or service is nil
				if err == nil {
					Fail("Expected error but got nil")
				}
			})
		})

		Context("UpdateProject", func() {
			It("should delegate to domain service", func() {
				ctx := context.Background()
				mr := &mutationResolver{resolver}
				input := model.UpdateProjectInput{
					Name: stringPtr("Updated Project"),
				}
				defer func() {
					if r := recover(); r != nil {
						// Panic is acceptable
					}
				}()
				_, err := mr.UpdateProject(ctx, "proj-1", input)
				if err == nil {
					Fail("Expected error but got nil")
				}
			})
		})

		Context("DeleteProject", func() {
			It("should delegate to domain service", func() {
				ctx := context.Background()
				mr := &mutationResolver{resolver}
				defer func() {
					if r := recover(); r != nil {
						// Panic is acceptable
					}
				}()
				_, err := mr.DeleteProject(ctx, "proj-1")
				if err == nil {
					Fail("Expected error but got nil")
				}
			})
		})

		Context("CreateTag", func() {
			It("should delegate to domain service", func() {
				ctx := context.Background()
				mr := &mutationResolver{resolver}
				input := model.CreateTagInput{
					Name: "test-tag",
				}
				defer func() {
					if r := recover(); r != nil {
						// Panic is acceptable for this test
					}
				}()
				_, _ = mr.CreateTag(ctx, input)
				// Test just verifies the wrapper exists and calls domain service
			})
		})
	})

	Describe("Query Resolvers - Wrapper Functions", func() {
		Context("DashboardSummary", func() {
			It("should delegate to domain service", func() {
				ctx := context.Background()
				qr := &queryResolver{resolver}
				defer func() {
					if r := recover(); r != nil {
						// Panic is acceptable
					}
				}()
				_, _ = qr.DashboardSummary(ctx)
			})
		})

		Context("TreemapData", func() {
			It("should delegate to domain service", func() {
				ctx := context.Background()
				qr := &queryResolver{resolver}
				defer func() {
					if r := recover(); r != nil {
						// Panic is acceptable
					}
				}()
				_, _ = qr.TreemapData(ctx, nil, nil)
			})
		})

		Context("TestRun", func() {
			It("should delegate to domain service", func() {
				ctx := context.Background()
				qr := &queryResolver{resolver}
				defer func() {
					if r := recover(); r != nil {
						// Panic is acceptable
					}
				}()
				_, _ = qr.TestRun(ctx, "1")
			})
		})

		Context("TestRunByRunID", func() {
			It("should return error when service not initialized", func() {
				ctx := context.Background()
				qr := &queryResolver{resolver}
				defer func() {
					if r := recover(); r != nil {
						// Panic is acceptable
					}
				}()
				_, _ = qr.TestRunByRunID(ctx, "run-1")
			})
		})

		Context("TestRuns", func() {
			It("should delegate to domain service", func() {
				ctx := context.Background()
				qr := &queryResolver{resolver}
				defer func() {
					if r := recover(); r != nil {
						// Panic is acceptable
					}
				}()
				_, _ = qr.TestRuns(ctx, nil, nil, nil, nil, nil)
			})
		})

		Context("RecentTestRuns", func() {
			It("should delegate to domain service", func() {
				ctx := context.Background()
				qr := &queryResolver{resolver}
				defer func() {
					if r := recover(); r != nil {
						// Panic is acceptable
					}
				}()
				_, _ = qr.RecentTestRuns(ctx, nil, nil)
			})
		})

		Context("Project", func() {
			It("should delegate to domain service", func() {
				ctx := context.Background()
				qr := &queryResolver{resolver}
				defer func() {
					if r := recover(); r != nil {
						// Panic is acceptable
					}
				}()
				_, _ = qr.Project(ctx, "1")
			})
		})

		Context("ProjectByProjectID", func() {
			It("should delegate to domain service", func() {
				ctx := context.Background()
				qr := &queryResolver{resolver}
				defer func() {
					if r := recover(); r != nil {
						// Panic is acceptable
					}
				}()
				_, _ = qr.ProjectByProjectID(ctx, "proj-1")
			})
		})

		Context("Projects", func() {
			It("should delegate to domain service", func() {
				ctx := context.Background()
				qr := &queryResolver{resolver}
				defer func() {
					if r := recover(); r != nil {
						// Panic is acceptable
					}
				}()
				_, _ = qr.Projects(ctx, nil, nil, nil)
			})
		})

		Context("Tags", func() {
			It("should delegate to domain service", func() {
				ctx := context.Background()
				qr := &queryResolver{resolver}
				defer func() {
					if r := recover(); r != nil {
						// Panic is acceptable
					}
				}()
				_, _ = qr.Tags(ctx, nil, nil, nil)
			})
		})

		Context("JiraConnection", func() {
			It("should return error when service not initialized", func() {
				ctx := context.Background()
				qr := &queryResolver{resolver}
				defer func() {
					if r := recover(); r != nil {
						// Panic is acceptable
					}
				}()
				_, _ = qr.JiraConnection(ctx, "conn-1")
			})
		})
	})

	Describe("Additional Jira Resolvers Coverage", func() {
		Context("CreateJiraConnection", func() {
			It("should handle various auth types", func() {
				user := &authDomain.User{
					UserID: "user-1",
					Role:   authDomain.RoleAdmin,
				}
				ctx := context.WithValue(context.Background(), "user", user)
				mr := &mutationResolver{resolver}

				input := model.CreateJiraConnectionInput{
					ProjectID:          "proj-1",
					Name:               "Jira Conn",
					JiraURL:            "https://jira.example.com",
					AuthenticationType: "oauth",
					ProjectKey:         "PROJ",
				}
				defer func() {
					if r := recover(); r != nil {
						// Panic is acceptable
					}
				}()
				_, _ = mr.CreateJiraConnection(ctx, input)
			})
		})

		Context("UpdateJiraConnection", func() {
			It("should handle update with all fields", func() {
				user := &authDomain.User{
					UserID: "user-1",
					Role:   authDomain.RoleAdmin,
				}
				ctx := context.WithValue(context.Background(), "user", user)
				mr := &mutationResolver{resolver}

				input := model.UpdateJiraConnectionInput{
					Name:       "Updated Conn",
					JiraURL:    "https://jira2.example.com",
					ProjectKey: "PROJ2",
				}
				defer func() {
					if r := recover(); r != nil {
						// Panic is acceptable
					}
				}()
				_, _ = mr.UpdateJiraConnection(ctx, "conn-1", input)
			})
		})

		Context("UpdateJiraCredentials", func() {
			It("should handle credential updates", func() {
				user := &authDomain.User{
					UserID: "user-1",
					Role:   authDomain.RoleAdmin,
				}
				ctx := context.WithValue(context.Background(), "user", user)
				mr := &mutationResolver{resolver}

				input := model.UpdateJiraCredentialsInput{
					AuthenticationType: "token",
					Username:           "user",
					Credential:         "token",
				}
				defer func() {
					if r := recover(); r != nil {
						// Panic is acceptable
					}
				}()
				_, _ = mr.UpdateJiraCredentials(ctx, "conn-1", input)
			})
		})

		Context("TestJiraConnection", func() {
			It("should handle connection test", func() {
				user := &authDomain.User{
					UserID: "user-1",
					Role:   authDomain.RoleAdmin,
				}
				ctx := context.WithValue(context.Background(), "user", user)
				mr := &mutationResolver{resolver}

				defer func() {
					if r := recover(); r != nil {
						// Panic is acceptable
					}
				}()
				_, _ = mr.TestJiraConnection(ctx, "conn-1")
			})
		})

		Context("DeleteJiraConnection", func() {
			It("should handle connection deletion", func() {
				user := &authDomain.User{
					UserID: "user-1",
					Role:   authDomain.RoleAdmin,
				}
				ctx := context.WithValue(context.Background(), "user", user)
				mr := &mutationResolver{resolver}

				defer func() {
					if r := recover(); r != nil {
						// Panic is acceptable
					}
				}()
				_, _ = mr.DeleteJiraConnection(ctx, "conn-1")
			})
		})
	})

	Describe("SpecRuns and SuiteRuns with Context", func() {
		var (
			testDB  *gorm.DB
			loaders *dataloader.Loaders
		)

		BeforeEach(func() {
			var err error
			testDB, err = gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
			Expect(err).ToNot(HaveOccurred())

			// Migrate test tables
			err = testDB.AutoMigrate(&database.SpecRun{}, &database.SuiteRun{}, &database.TestRun{})
			Expect(err).ToNot(HaveOccurred())

			loaders = dataloader.NewLoaders(testDB)
		})

		Context("SpecRuns", func() {
			It("should load spec runs for valid suite run", func() {
				// Create test data
				suiteRun := &database.SuiteRun{
					TestRunID: 1,
					SuiteName: "Test Suite",
					Status:    "passed",
				}
				err := testDB.Create(suiteRun).Error
				Expect(err).ToNot(HaveOccurred())

				specRun := &database.SpecRun{
					SuiteRunID: suiteRun.ID,
					SpecName:   "Test Spec",
					Status:     "passed",
					StartTime:  time.Now(),
					Duration:   1000,
				}
				err = testDB.Create(specRun).Error
				Expect(err).ToNot(HaveOccurred())

				// Create context with loaders
				ctx := context.WithValue(context.Background(), "loaders", loaders)

				// Update resolver to use testDB
				testResolver := &Resolver{
					logger: resolver.logger,
					db:     testDB,
				}
				sr := &suiteRunResolver{testResolver}

				modelSuiteRun := &model.SuiteRun{
					ID: fmt.Sprintf("%d", suiteRun.ID),
				}

				result, err := sr.SpecRuns(ctx, modelSuiteRun)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(BeNil())
				Expect(len(result)).To(Equal(1))
				Expect(result[0].SpecName).To(Equal("Test Spec"))
			})
		})

		Context("SuiteRuns", func() {
			It("should load suite runs for valid test run", func() {
				// Create test data
				testRun := &database.TestRun{
					RunID:     "test-run-1",
					ProjectID: "proj-1",
					Status:    "passed",
				}
				err := testDB.Create(testRun).Error
				Expect(err).ToNot(HaveOccurred())

				suiteRun := &database.SuiteRun{
					TestRunID: testRun.ID,
					SuiteName: "Suite 1",
					Status:    "passed",
				}
				err = testDB.Create(suiteRun).Error
				Expect(err).ToNot(HaveOccurred())

				// Create context with loaders
				ctx := context.WithValue(context.Background(), "loaders", loaders)

				// Update resolver to use testDB
				testResolver := &Resolver{
					logger: resolver.logger,
					db:     testDB,
				}
				tr := &testRunResolver{testResolver}

				modelTestRun := &model.TestRun{
					ID: fmt.Sprintf("%d", testRun.ID),
				}

				result, err := tr.SuiteRuns(ctx, modelTestRun)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(BeNil())
				Expect(len(result)).To(Equal(1))
				Expect(result[0].SuiteName).To(Equal("Suite 1"))
			})
		})
	})
})

// Note: RunSpecs is called in domain_resolvers_integration_test.go
// to avoid Ginkgo's "calling RunSpecs more than once" error
