// Package database contains shared database models and interfaces
package database

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"
)

// JSONMap is a custom type for handling JSONB columns
type JSONMap map[string]interface{}

// Value implements the driver.Valuer interface for JSONMap
func (j JSONMap) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

// Scan implements the sql.Scanner interface for JSONMap
func (j *JSONMap) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}

	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return errors.New("failed to scan JSONMap: invalid type")
	}

	return json.Unmarshal(bytes, j)
}

// BaseModel provides common fields for all database models
type BaseModel struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}

// UserPreferences stores user-specific preferences
type UserPreferences struct {
	ID          uint            `gorm:"primarykey" json:"id"`
	UserID      string          `gorm:"uniqueIndex;not null" json:"user_id"`
	Theme       string          `gorm:"default:'light'" json:"theme"`
	Timezone    string          `gorm:"default:'UTC'" json:"timezone"`
	Language    string          `gorm:"default:'en'" json:"language"`
	Favorites   json.RawMessage `gorm:"type:jsonb" json:"favorites"`
	Preferences json.RawMessage `gorm:"type:jsonb" json:"preferences"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
	DeletedAt   gorm.DeletedAt  `gorm:"index" json:"deleted_at,omitempty"`
}

// TableName returns the table name for UserPreferences
func (UserPreferences) TableName() string {
	return "user_preferences"
}

// TestRun represents a test execution instance
type TestRun struct {
	BaseModel
	ProjectID    string     `gorm:"not null;index" json:"project_id"`
	RunID        string     `gorm:"uniqueIndex;not null" json:"run_id"`
	Branch       string     `gorm:"index" json:"branch"`
	CommitSHA    string     `gorm:"index" json:"commit_sha"`
	Status       string     `gorm:"index;default:'running'" json:"status"`
	StartTime    time.Time  `gorm:"index" json:"start_time"`
	EndTime      *time.Time `json:"end_time,omitempty"`
	TotalTests   int        `json:"total_tests"`
	PassedTests  int        `json:"passed_tests"`
	FailedTests  int        `json:"failed_tests"`
	SkippedTests int        `json:"skipped_tests"`
	Duration     int64      `gorm:"column:duration_ms" json:"duration_ms"` // Duration in milliseconds
	Environment  string     `gorm:"index" json:"environment"`
	Tags         []Tag      `gorm:"many2many:test_run_tags;" json:"tags"`
	SuiteRuns    []SuiteRun `gorm:"foreignKey:TestRunID" json:"suite_runs,omitempty"`
	Metadata     JSONMap    `gorm:"type:jsonb" json:"metadata,omitempty"`
}

// SuiteRun represents a test suite execution within a test run
type SuiteRun struct {
	BaseModel
	TestRunID    uint       `gorm:"not null;index" json:"test_run_id"`
	SuiteName    string     `gorm:"not null;index" json:"suite_name"`
	Status       string     `gorm:"index;default:'running'" json:"status"`
	StartTime    time.Time  `json:"start_time"`
	EndTime      *time.Time `json:"end_time,omitempty"`
	TotalSpecs   int        `json:"total_specs"`
	PassedSpecs  int        `json:"passed_specs"`
	FailedSpecs  int        `json:"failed_specs"`
	SkippedSpecs int        `json:"skipped_specs"`
	Duration     int64      `gorm:"column:duration_ms" json:"duration_ms"`
	SpecRuns     []SpecRun  `gorm:"foreignKey:SuiteRunID" json:"spec_runs,omitempty"`
}

// SpecRun represents an individual test spec execution
type SpecRun struct {
	BaseModel
	SuiteRunID   uint       `gorm:"not null;index" json:"suite_run_id"`
	SpecName     string     `gorm:"not null;index" json:"spec_name"`
	Status       string     `gorm:"index" json:"status"`
	StartTime    time.Time  `json:"start_time"`
	EndTime      *time.Time `json:"end_time,omitempty"`
	Duration     int64      `gorm:"column:duration_ms" json:"duration_ms"`
	ErrorMessage string     `gorm:"type:text" json:"error_message,omitempty"`
	StackTrace   string     `gorm:"type:text" json:"stack_trace,omitempty"`
	RetryCount   int        `json:"retry_count"`
	IsFlaky      bool       `gorm:"index" json:"is_flaky"`
}

// Tag represents a test run tag for categorization
type Tag struct {
	BaseModel
	Name        string    `gorm:"uniqueIndex;not null" json:"name"`
	Description string    `json:"description,omitempty"`
	Color       string    `json:"color,omitempty"`
	TestRuns    []TestRun `gorm:"many2many:test_run_tags;" json:"test_runs,omitempty"`
}

// ProjectDetails represents project configuration and metadata
type ProjectDetails struct {
	BaseModel
	ProjectID     string `gorm:"uniqueIndex;not null" json:"project_id"`
	Name          string `gorm:"not null" json:"name"`
	Description   string `json:"description,omitempty"`
	Repository    string `json:"repository,omitempty"`
	DefaultBranch string `json:"default_branch"`
	Settings      string `gorm:"type:jsonb" json:"settings,omitempty"`
	IsActive      bool   `gorm:"default:true" json:"is_active"`
	Team          string `gorm:"index" json:"team,omitempty"` // Team that owns this project
}

// UserPreference represents user-specific settings and preferences
type UserPreference struct {
	BaseModel
	UserID      string `gorm:"uniqueIndex;not null" json:"user_id"`
	Theme       string `gorm:"default:'light'" json:"theme"`
	Timezone    string `gorm:"default:'UTC'" json:"timezone"`
	Language    string `gorm:"default:'en'" json:"language"`
	Favorites   string `gorm:"type:jsonb" json:"favorites,omitempty"`
	Preferences string `gorm:"type:jsonb" json:"preferences,omitempty"`
}

// UserScope represents a scope granted to a user
type UserScope struct {
	BaseModel
	UserID    string     `gorm:"not null;index;index:idx_user_scope,unique" json:"user_id"`
	Scope     string     `gorm:"not null;index:idx_user_scope,unique" json:"scope"`
	GrantedBy string     `json:"granted_by"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// JiraConnection represents a JIRA integration connection in the database
type JiraConnection struct {
	BaseModel
	ProjectID           string    `gorm:"type:varchar(36);not null;index" json:"project_id"`
	Name                string    `gorm:"type:varchar(255);not null" json:"name"`
	JiraURL             string    `gorm:"type:varchar(500);not null" json:"jira_url"`
	AuthenticationType  string    `gorm:"type:varchar(50);not null" json:"authentication_type"`
	ProjectKey          string    `gorm:"type:varchar(50);not null" json:"project_key"`
	Username            string    `gorm:"type:varchar(255);not null" json:"username"`
	EncryptedCredential string    `gorm:"type:text;not null" json:"-"`
	Status              string    `gorm:"type:varchar(50);not null;default:'pending'" json:"status"`
	IsActive            bool      `gorm:"not null;default:false" json:"is_active"`
	LastTestedAt        *time.Time `json:"last_tested_at,omitempty"`
}

// ProjectPermission represents explicit project permissions for a user
type ProjectPermission struct {
	BaseModel
	ProjectID  string     `gorm:"not null;index;index:idx_project_user_perm,unique" json:"project_id"`
	UserID     string     `gorm:"not null;index;index:idx_project_user_perm,unique" json:"user_id"`
	Permission string     `gorm:"not null;index:idx_project_user_perm,unique" json:"permission"` // read, write, delete, admin
	GrantedBy  string     `json:"granted_by"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
}

// TestRunTag represents the many-to-many relationship between test runs and tags
type TestRunTag struct {
	TestRunID uint      `gorm:"primaryKey" json:"test_run_id"`
	TagID     uint      `gorm:"primaryKey" json:"tag_id"`
	CreatedAt time.Time `json:"created_at"`
}

// FlakyTest represents test flakiness analysis data
type FlakyTest struct {
	BaseModel
	ProjectID        string    `gorm:"not null;index" json:"project_id"`
	TestName         string    `gorm:"not null;index" json:"test_name"`
	SuiteName        string    `gorm:"index" json:"suite_name"`
	FlakeRate        float64   `json:"flake_rate"` // Percentage of flaky executions
	TotalExecutions  int       `json:"total_executions"`
	FlakyExecutions  int       `json:"flaky_executions"`
	LastSeenAt       time.Time `json:"last_seen_at"`
	FirstSeenAt      time.Time `json:"first_seen_at"`
	Status           string    `gorm:"default:'active'" json:"status"`
	Severity         string    `json:"severity"` // low, medium, high, critical
	LastErrorMessage string    `gorm:"type:text" json:"last_error_message,omitempty"`
}

// User represents a system user with OAuth authentication
type User struct {
	BaseModel
	UserID        string          `gorm:"uniqueIndex;not null" json:"user_id"`  // OAuth provider user ID
	Email         string          `gorm:"uniqueIndex;not null" json:"email"`    // User email
	Name          string          `gorm:"not null" json:"name"`                 // Display name
	Role          string          `gorm:"default:'user';index" json:"role"`     // user, admin
	Status        string          `gorm:"default:'active';index" json:"status"` // active, suspended, inactive
	LastLoginAt   *time.Time      `json:"last_login_at,omitempty"`
	ProfileURL    string          `json:"profile_url,omitempty"`               // Avatar/profile picture URL
	FirstName     string          `json:"first_name,omitempty"`                // First name from OAuth
	LastName      string          `json:"last_name,omitempty"`                 // Last name from OAuth
	EmailVerified bool            `gorm:"default:false" json:"email_verified"` // Email verification status
	ProjectAccess []ProjectAccess `gorm:"foreignKey:UserID;references:UserID" json:"project_access,omitempty"`
	UserGroups    []UserGroup     `gorm:"foreignKey:UserID;references:UserID" json:"user_groups,omitempty"`
	UserScopes    []UserScope     `gorm:"foreignKey:UserID;references:UserID" json:"user_scopes,omitempty"`
}

// UserGroup represents a user's group membership
type UserGroup struct {
	BaseModel
	UserID    string `gorm:"not null;index;references:users(user_id)" json:"user_id"`
	GroupName string `gorm:"not null;index" json:"group_name"`
}

// UserSession represents an active user session
type UserSession struct {
	BaseModel
	UserID       string    `gorm:"not null;index;references:users(user_id)" json:"user_id"`
	SessionID    string    `gorm:"uniqueIndex;not null" json:"session_id"`
	AccessToken  string    `gorm:"type:text" json:"-"` // Don't serialize to JSON
	RefreshToken string    `gorm:"type:text" json:"-"` // Don't serialize to JSON
	IDToken      string    `gorm:"type:text" json:"-"` // ID token for logout
	ExpiresAt    time.Time `gorm:"index" json:"expires_at"`
	IsActive     bool      `gorm:"default:true;index" json:"is_active"`
	IPAddress    string    `json:"ip_address,omitempty"`
	UserAgent    string    `gorm:"type:text" json:"user_agent,omitempty"`
	LastActivity time.Time `gorm:"index" json:"last_activity"`
}

// ProjectAccess represents user access permissions for specific projects
type ProjectAccess struct {
	BaseModel
	UserID    string     `gorm:"not null;index;references:users(user_id)" json:"user_id"`
	ProjectID string     `gorm:"not null;index" json:"project_id"`
	Role      string     `gorm:"not null" json:"role"` // viewer, editor, admin
	GrantedBy string     `json:"granted_by,omitempty"` // Who granted this access
	GrantedAt time.Time  `json:"granted_at"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"` // Optional expiration
}

// UserRole represents possible user roles
type UserRole string

const (
	RoleUser  UserRole = "user"
	RoleAdmin UserRole = "admin"
)

// ProjectRole represents possible project-level roles
type ProjectRole string

const (
	ProjectRoleViewer ProjectRole = "viewer"
	ProjectRoleEditor ProjectRole = "editor"
	ProjectRoleAdmin  ProjectRole = "admin"
)

// Repository interface defines common database operations
type Repository interface {
	Create(entity interface{}) error
	GetByID(id uint, entity interface{}) error
	Update(entity interface{}) error
	Delete(id uint, entity interface{}) error
	List(entities interface{}, filters map[string]interface{}) error
	Count(entity interface{}, filters map[string]interface{}) (int64, error)
}

// BaseRepository provides common database operations
type BaseRepository struct {
	db *gorm.DB
}

// NewBaseRepository creates a new base repository
func NewBaseRepository(db *gorm.DB) *BaseRepository {
	return &BaseRepository{db: db}
}

// Create creates a new entity
func (r *BaseRepository) Create(entity interface{}) error {
	return r.db.Create(entity).Error
}

// GetByID retrieves an entity by ID
func (r *BaseRepository) GetByID(id uint, entity interface{}) error {
	return r.db.First(entity, id).Error
}

// Update updates an entity
func (r *BaseRepository) Update(entity interface{}) error {
	return r.db.Save(entity).Error
}

// Delete soft deletes an entity by ID
func (r *BaseRepository) Delete(id uint, entity interface{}) error {
	return r.db.Delete(entity, id).Error
}

// List retrieves entities with optional filters
func (r *BaseRepository) List(entities interface{}, filters map[string]interface{}) error {
	query := r.db
	for key, value := range filters {
		query = query.Where(key, value)
	}
	return query.Find(entities).Error
}

// Count returns the count of entities matching the filters
func (r *BaseRepository) Count(entity interface{}, filters map[string]interface{}) (int64, error) {
	var count int64
	query := r.db.Model(entity)
	for key, value := range filters {
		query = query.Where(key, value)
	}
	err := query.Count(&count).Error
	return count, err
}
