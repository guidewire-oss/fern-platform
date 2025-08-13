package graphql

import (
	"context"
	"fmt"
	"strings"
	"time"

	authDomain "github.com/guidewire-oss/fern-platform/internal/domains/auth/domain"
	"github.com/guidewire-oss/fern-platform/internal/domains/integrations"
	"github.com/guidewire-oss/fern-platform/internal/reporter/graphql/dataloader"
	"github.com/guidewire-oss/fern-platform/internal/reporter/graphql/model"
)

// getLoaders gets the dataloader from context
func getLoaders(ctx context.Context) *dataloader.Loaders {
	if ctx == nil {
		return nil
	}

	loadersVal := ctx.Value("loaders")
	if loadersVal == nil {
		return nil
	}

	loaders, ok := loadersVal.(*dataloader.Loaders)
	if !ok {
		return nil
	}
	return loaders
}

// getCurrentUser gets the current user from context
func getCurrentUser(ctx context.Context) (*authDomain.User, error) {
	user, ok := ctx.Value("user").(*authDomain.User)
	if !ok {
		return nil, fmt.Errorf("user not authenticated")
	}
	return user, nil
}

// getRequestID gets the request ID from context
func getRequestID(ctx context.Context) string {
	reqID, _ := ctx.Value("request_id").(string)
	return reqID
}

// convertPtrString converts a *string to string
func convertPtrString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// convertStringPtr converts a string to *string
func convertStringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// paginateSlice applies pagination to a slice
func paginateSlice[T any](items []T, first int, after string) ([]T, bool) {
	start := 0
	if after != "" {
		// Simple cursor implementation - in production, use proper cursor encoding
		fmt.Sscanf(after, "%d", &start)
	}

	if start >= len(items) {
		return []T{}, false
	}

	end := start + first
	hasMore := false

	if end > len(items) {
		end = len(items)
	} else {
		hasMore = true
	}

	return items[start:end], hasMore
}

// getUserTeamsFromContext extracts user teams from context
func getUserTeamsFromContext(ctx context.Context) []string {
	user, err := getCurrentUser(ctx)
	if err != nil {
		return nil
	}

	// Get role group names from context (set by resolver)
	roleGroups := getRoleGroupNamesFromContext(ctx)

	var teams []string
	teamMap := make(map[string]bool)

	for _, group := range user.Groups {
		groupName := strings.TrimPrefix(group.GroupName, "/")

		// Check if this is a team group (not a role group)
		if !isRoleGroup(groupName, roleGroups) {
			teamMap[groupName] = true
		}
	}

	// Convert map to slice
	for team := range teamMap {
		teams = append(teams, team)
	}

	return teams
}

// getUserScopesFromContext extracts user scopes from context
func getUserScopesFromContext(ctx context.Context) []string {
	user, err := getCurrentUser(ctx)
	if err != nil {
		return nil
	}

	scopes := make([]string, 0, len(user.Scopes))
	now := time.Now()

	for _, scope := range user.Scopes {
		// Skip expired scopes
		if scope.ExpiresAt != nil && scope.ExpiresAt.Before(now) {
			continue
		}
		scopes = append(scopes, scope.Scope)
	}

	return scopes
}

// matchScope matches a scope pattern with wildcards
func matchScope(userScope, requiredScope string) bool {
	// Exact match
	if userScope == requiredScope {
		return true
	}

	// Split scopes into parts
	userParts := strings.Split(userScope, ":")
	requiredParts := strings.Split(requiredScope, ":")

	// Must have same number of parts
	if len(userParts) != len(requiredParts) {
		return false
	}

	// Check each part
	for i := range userParts {
		if userParts[i] == "*" || requiredParts[i] == "*" {
			continue
		}
		if userParts[i] != requiredParts[i] {
			return false
		}
	}

	return true
}

// RoleGroupNames holds the configurable role group names
type RoleGroupNames struct {
	AdminGroup   string
	ManagerGroup string
	UserGroup    string
}

// getRoleGroupNamesFromContext gets role group names from context
func getRoleGroupNamesFromContext(ctx context.Context) *RoleGroupNames {
	if names, ok := ctx.Value("roleGroupNames").(*RoleGroupNames); ok {
		return names
	}
	// Return defaults if not found
	return &RoleGroupNames{
		AdminGroup:   "admin",
		ManagerGroup: "manager",
		UserGroup:    "user",
	}
}

// isRoleGroup checks if a group name is a role group
func isRoleGroup(groupName string, roleGroups *RoleGroupNames) bool {
	return groupName == roleGroups.AdminGroup ||
		groupName == roleGroups.ManagerGroup ||
		groupName == roleGroups.UserGroup
}

// hasManagerRole checks if user has the manager role group
func hasManagerRole(user *authDomain.User, roleGroups *RoleGroupNames) bool {
	for _, group := range user.Groups {
		groupName := strings.TrimPrefix(group.GroupName, "/")
		if groupName == roleGroups.ManagerGroup {
			return true
		}
	}
	return false
}

// hasUserRole checks if user has the user role group
func hasUserRole(user *authDomain.User, roleGroups *RoleGroupNames) bool {
	for _, group := range user.Groups {
		groupName := strings.TrimPrefix(group.GroupName, "/")
		if groupName == roleGroups.UserGroup {
			return true
		}
	}
	return false
}


// convertJiraConnectionToModel converts a domain JIRA connection to GraphQL model
func (r *Resolver) convertJiraConnectionToModel(conn *integrations.JiraConnection) *model.JiraConnection {
	if conn == nil {
		return nil
	}

	// Convert time pointers
	var lastTestedAt *time.Time
	if conn.LastTestedAt() != nil {
		t := *conn.LastTestedAt()
		lastTestedAt = &t
	}

	createdAt := conn.CreatedAt()
	updatedAt := conn.UpdatedAt()

	return &model.JiraConnection{
		ID:                 conn.ID(),
		ProjectID:          conn.ProjectID(),
		Name:               conn.Name(),
		JiraURL:            conn.JiraURL(),
		AuthenticationType: string(conn.AuthenticationType()),
		ProjectKey:         conn.ProjectKey(),
		Username:           conn.Username(),
		Status:             string(conn.Status()),
		IsActive:           conn.IsActive(),
		LastTestedAt:       lastTestedAt,
		CreatedAt:          createdAt,
		UpdatedAt:          updatedAt,
	}
}
