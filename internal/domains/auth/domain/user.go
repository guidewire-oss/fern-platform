package domain

import (
	"time"
)

// User represents a user in the auth domain
type User struct {
	UserID        string
	Email         string
	Name          string
	FirstName     string
	LastName      string
	Role          UserRole
	Status        UserStatus
	ProfileURL    string
	EmailVerified bool
	LastLoginAt   *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time

	// Relationships
	Groups []UserGroup
	Scopes []UserScope
}

// UserRole represents the role of a user
type UserRole string

const (
	RoleAdmin   UserRole = "admin"
	RoleManager UserRole = "manager"
	RoleUser    UserRole = "user"
)

// UserStatus represents the status of a user account
type UserStatus string

const (
	StatusActive    UserStatus = "active"
	StatusInactive  UserStatus = "inactive"
	StatusSuspended UserStatus = "suspended"
)

// UserGroup represents a user's group membership
type UserGroup struct {
	UserID    string
	GroupName string
	CreatedAt time.Time
}

// UserScope represents a permission scope assigned to a user
type UserScope struct {
	UserID    string
	Scope     string
	ExpiresAt *time.Time
	GrantedBy string
	GrantedAt time.Time
}

// IsAdmin checks if the user has admin role
func (u *User) IsAdmin() bool {
	return u.Role == RoleAdmin
}

// IsManager checks if the user has manager role
func (u *User) IsManager() bool {
	return u.Role == RoleManager
}

// IsActive checks if the user account is active
func (u *User) IsActive() bool {
	return u.Status == StatusActive
}

// HasGroup checks if the user belongs to a specific group
func (u *User) HasGroup(groupName string) bool {
	for _, group := range u.Groups {
		if group.GroupName == groupName {
			return true
		}
	}
	return false
}

// GetTeams extracts team names from user groups
func (u *User) GetTeams() []string {
	teams := make([]string, 0)
	for _, group := range u.Groups {
		if team := extractTeamFromGroup(group.GroupName); team != "" {
			teams = append(teams, team)
		}
	}
	return teams
}

// IsTeamManager checks if user is a manager for any team
func (u *User) IsTeamManager() bool {
	// Admins have all permissions
	if u.IsAdmin() {
		return true
	}

	// Users with manager role are managers
	if u.IsManager() {
		return true
	}

	// Check if user belongs to team-specific manager groups
	for _, group := range u.Groups {
		if isManagerGroup(group.GroupName) {
			return true
		}
	}
	return false
}

// IsManagerForTeam checks if user is a manager for a specific team
func (u *User) IsManagerForTeam(team string) bool {
	// Admins have all permissions
	if u.IsAdmin() {
		return true
	}

	// Users with manager role can manage all teams
	if u.IsManager() {
		return true
	}

	// Check if user belongs to team-specific manager group
	managerGroup := team + "-managers"
	return u.HasGroup(managerGroup)
}

// Helper functions
func extractTeamFromGroup(groupName string) string {
	// Remove leading slash if present
	groupName = trimPrefix(groupName, "/")

	// Extract team name from group pattern
	if hasSuffix(groupName, "-managers") {
		return trimSuffix(groupName, "-managers")
	} else if hasSuffix(groupName, "-users") {
		return trimSuffix(groupName, "-users")
	}
	return ""
}

func isManagerGroup(groupName string) bool {
	groupName = trimPrefix(groupName, "/")
	return hasSuffix(groupName, "-managers")
}

// String helper functions to avoid importing strings
func trimPrefix(s, prefix string) string {
	if len(s) >= len(prefix) && s[:len(prefix)] == prefix {
		return s[len(prefix):]
	}
	return s
}

func trimSuffix(s, suffix string) string {
	if len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix {
		return s[:len(s)-len(suffix)]
	}
	return s
}

func hasSuffix(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}
