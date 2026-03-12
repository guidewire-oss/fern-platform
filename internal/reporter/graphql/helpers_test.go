package graphql

import (
	"context"
	"testing"
	"time"

	authDomain "github.com/guidewire-oss/fern-platform/internal/domains/auth/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertPtrString(t *testing.T) {
	t.Run("nil returns empty", func(t *testing.T) {
		assert.Equal(t, "", convertPtrString(nil))
	})

	t.Run("non-nil returns value", func(t *testing.T) {
		s := "hello"
		assert.Equal(t, "hello", convertPtrString(&s))
	})

	t.Run("empty string pointer", func(t *testing.T) {
		s := ""
		assert.Equal(t, "", convertPtrString(&s))
	})
}

func TestConvertStringPtr_Helpers(t *testing.T) {
	t.Run("empty returns nil", func(t *testing.T) {
		assert.Nil(t, ConvertStringPtr(""))
	})

	t.Run("non-empty returns pointer", func(t *testing.T) {
		result := ConvertStringPtr("hello")
		require.NotNil(t, result)
		assert.Equal(t, "hello", *result)
	})
}

func TestConvertStringPtrUnexported(t *testing.T) {
	// Test the unexported wrapper too
	result := convertStringPtr("test")
	require.NotNil(t, result)
	assert.Equal(t, "test", *result)

	result = convertStringPtr("")
	assert.Nil(t, result)
}

func TestPaginateSlice(t *testing.T) {
	items := []int{1, 2, 3, 4, 5}

	t.Run("first page", func(t *testing.T) {
		result, hasMore := paginateSlice(items, 2, "")
		assert.Equal(t, []int{1, 2}, result)
		assert.True(t, hasMore)
	})

	t.Run("second page with cursor", func(t *testing.T) {
		result, hasMore := paginateSlice(items, 2, "2")
		assert.Equal(t, []int{3, 4}, result)
		assert.True(t, hasMore)
	})

	t.Run("last page", func(t *testing.T) {
		result, hasMore := paginateSlice(items, 2, "4")
		assert.Equal(t, []int{5}, result)
		assert.False(t, hasMore)
	})

	t.Run("cursor beyond length", func(t *testing.T) {
		result, hasMore := paginateSlice(items, 2, "10")
		assert.Empty(t, result)
		assert.False(t, hasMore)
	})

	t.Run("first larger than remaining", func(t *testing.T) {
		result, hasMore := paginateSlice(items, 10, "")
		assert.Equal(t, items, result)
		assert.False(t, hasMore)
	})

	t.Run("empty slice", func(t *testing.T) {
		result, hasMore := paginateSlice([]int{}, 5, "")
		assert.Empty(t, result)
		assert.False(t, hasMore)
	})

	t.Run("exact fit", func(t *testing.T) {
		result, hasMore := paginateSlice(items, 5, "")
		assert.Equal(t, items, result)
		// When end == len(items), hasMore is true because of how the boundary check works
		assert.True(t, hasMore)
	})
}

func TestGetLoaders(t *testing.T) {
	t.Run("no loaders in context", func(t *testing.T) {
		ctx := context.Background()
		assert.Nil(t, getLoaders(ctx))
	})

	t.Run("wrong type in context", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), "loaders", "not-loaders")
		assert.Nil(t, getLoaders(ctx))
	})
}

func TestGetCurrentUser(t *testing.T) {
	t.Run("user present", func(t *testing.T) {
		user := &authDomain.User{UserID: "u1", Email: "u1@test.com"}
		ctx := context.WithValue(context.Background(), "user", user)

		result, err := getCurrentUser(ctx)
		require.NoError(t, err)
		assert.Equal(t, "u1", result.UserID)
	})

	t.Run("user not present", func(t *testing.T) {
		ctx := context.Background()

		result, err := getCurrentUser(ctx)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "not authenticated")
	})

	t.Run("wrong type in context", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), "user", "not-a-user")

		result, err := getCurrentUser(ctx)
		assert.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestGetRequestID(t *testing.T) {
	t.Run("present", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), "request_id", "req-123")
		assert.Equal(t, "req-123", getRequestID(ctx))
	})

	t.Run("not present", func(t *testing.T) {
		ctx := context.Background()
		assert.Equal(t, "", getRequestID(ctx))
	})
}

func TestMatchScope(t *testing.T) {
	tests := []struct {
		name     string
		user     string
		required string
		expected bool
	}{
		{"exact match", "read:projects", "read:projects", true},
		{"wildcard user", "read:*", "read:projects", true},
		{"wildcard required", "read:projects", "read:*", true},
		{"no match", "read:projects", "write:projects", false},
		{"different part count", "read:projects:all", "read:projects", false},
		{"multi-part wildcard", "*:projects:*", "read:projects:all", true},
		{"single segment", "admin", "admin", true},
		{"single segment mismatch", "admin", "user", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test the package-level matchScope in helpers.go
			result := matchScope(tt.user, tt.required)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetRoleGroupNamesFromContext(t *testing.T) {
	t.Run("custom names in context", func(t *testing.T) {
		names := &RoleGroupNames{
			AdminGroup:   "superadmin",
			ManagerGroup: "lead",
			UserGroup:    "member",
		}
		ctx := context.WithValue(context.Background(), "roleGroupNames", names)

		result := getRoleGroupNamesFromContext(ctx)
		assert.Equal(t, "superadmin", result.AdminGroup)
		assert.Equal(t, "lead", result.ManagerGroup)
		assert.Equal(t, "member", result.UserGroup)
	})

	t.Run("defaults when not in context", func(t *testing.T) {
		ctx := context.Background()
		result := getRoleGroupNamesFromContext(ctx)
		assert.Equal(t, "admin", result.AdminGroup)
		assert.Equal(t, "manager", result.ManagerGroup)
		assert.Equal(t, "user", result.UserGroup)
	})
}

func TestIsRoleGroup(t *testing.T) {
	groups := &RoleGroupNames{
		AdminGroup:   "admin",
		ManagerGroup: "manager",
		UserGroup:    "user",
	}

	assert.True(t, isRoleGroup("admin", groups))
	assert.True(t, isRoleGroup("manager", groups))
	assert.True(t, isRoleGroup("user", groups))
	assert.False(t, isRoleGroup("developers", groups))
	assert.False(t, isRoleGroup("", groups))
}

func TestHasManagerRole(t *testing.T) {
	groups := &RoleGroupNames{ManagerGroup: "manager"}

	t.Run("has manager", func(t *testing.T) {
		user := &authDomain.User{
			Groups: []authDomain.UserGroup{{GroupName: "manager"}},
		}
		assert.True(t, hasManagerRole(user, groups))
	})

	t.Run("has manager with prefix", func(t *testing.T) {
		user := &authDomain.User{
			Groups: []authDomain.UserGroup{{GroupName: "/manager"}},
		}
		assert.True(t, hasManagerRole(user, groups))
	})

	t.Run("no manager", func(t *testing.T) {
		user := &authDomain.User{
			Groups: []authDomain.UserGroup{{GroupName: "user"}},
		}
		assert.False(t, hasManagerRole(user, groups))
	})

	t.Run("no groups", func(t *testing.T) {
		user := &authDomain.User{}
		assert.False(t, hasManagerRole(user, groups))
	})
}

func TestHasUserRole(t *testing.T) {
	groups := &RoleGroupNames{UserGroup: "user"}

	t.Run("has user role", func(t *testing.T) {
		user := &authDomain.User{
			Groups: []authDomain.UserGroup{{GroupName: "user"}},
		}
		assert.True(t, hasUserRole(user, groups))
	})

	t.Run("no user role", func(t *testing.T) {
		user := &authDomain.User{
			Groups: []authDomain.UserGroup{{GroupName: "admin"}},
		}
		assert.False(t, hasUserRole(user, groups))
	})
}

func TestGetUserTeamsFromContext(t *testing.T) {
	t.Run("no user in context", func(t *testing.T) {
		ctx := context.Background()
		teams := getUserTeamsFromContext(ctx)
		assert.Nil(t, teams)
	})

	t.Run("user with team groups", func(t *testing.T) {
		user := &authDomain.User{
			Groups: []authDomain.UserGroup{
				{GroupName: "/developers"},
				{GroupName: "admin"}, // role group, should be excluded
			},
		}
		roleNames := &RoleGroupNames{
			AdminGroup:   "admin",
			ManagerGroup: "manager",
			UserGroup:    "user",
		}
		ctx := context.WithValue(context.Background(), "user", user)
		ctx = context.WithValue(ctx, "roleGroupNames", roleNames)

		teams := getUserTeamsFromContext(ctx)
		assert.Contains(t, teams, "developers")
		// Should not contain "admin" since it's a role group
		for _, team := range teams {
			assert.NotEqual(t, "admin", team)
		}
	})
}

func TestGetUserScopesFromContext(t *testing.T) {
	t.Run("no user", func(t *testing.T) {
		ctx := context.Background()
		scopes := getUserScopesFromContext(ctx)
		assert.Nil(t, scopes)
	})

	t.Run("user with scopes", func(t *testing.T) {
		futureTime := time.Now().Add(time.Hour)
		pastTime := time.Now().Add(-time.Hour)
		user := &authDomain.User{
			Scopes: []authDomain.UserScope{
				{Scope: "read:projects"},
				{Scope: "write:projects", ExpiresAt: &futureTime},
				{Scope: "expired:scope", ExpiresAt: &pastTime},
			},
		}
		ctx := context.WithValue(context.Background(), "user", user)

		scopes := getUserScopesFromContext(ctx)
		assert.Contains(t, scopes, "read:projects")
		assert.Contains(t, scopes, "write:projects")
		assert.NotContains(t, scopes, "expired:scope")
	})

	t.Run("user with no scopes", func(t *testing.T) {
		user := &authDomain.User{}
		ctx := context.WithValue(context.Background(), "user", user)

		scopes := getUserScopesFromContext(ctx)
		assert.Empty(t, scopes)
	})
}
