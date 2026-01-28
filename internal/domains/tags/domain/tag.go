package domain

import (
	"errors"
	"strings"
	"time"
)

// TagID represents a unique identifier for a tag
type TagID string

// Tag represents a label that can be applied to test runs
type Tag struct {
	id        TagID
	name      string
	category  string
	value     string
	createdAt time.Time
}

// TagSnapshot represents a point-in-time view of a tag
type TagSnapshot struct {
	ID        TagID     `json:"id"`
	Name      string    `json:"name"`
	Category  string    `json:"category"`
	Value     string    `json:"value"`
	CreatedAt time.Time `json:"created_at"`
}

// NewTag creates a new tag
func NewTag(name string) (*Tag, error) {
	if name == "" {
		return nil, errors.New("tag name cannot be empty")
	}

	// Normalize tag name
	normalizedName := strings.TrimSpace(strings.ToLower(name))
	if normalizedName == "" {
		return nil, errors.New("tag name cannot be empty after normalization")
	}

	// Parse category and value from name
	// If there is no colon, put the whole input into Value
	// Example: "priority:high" -> Category: "priority", Value: "high"
	var category, value string
	if idx := strings.Index(normalizedName, ":"); idx == -1 {
		value = normalizedName
	} else {
		category = strings.TrimSpace(normalizedName[:idx])
		value = strings.TrimSpace(normalizedName[idx+1:])
	}

	return &Tag{
		id:        TagID(""), // ID will be assigned by database
		name:      normalizedName,
		category:  category,
		value:     value,
		createdAt: time.Now(),
	}, nil
}

// ID returns the tag ID
func (t *Tag) ID() TagID {
	return t.id
}

// Name returns the tag name
func (t *Tag) Name() string {
	return t.name
}

// Category returns the tag category
func (t *Tag) Category() string {
	return t.category
}

// Value returns the tag value
func (t *Tag) Value() string {
	return t.value
}

// CreatedAt returns the creation time
func (t *Tag) CreatedAt() time.Time {
	return t.createdAt
}

// ToSnapshot returns a snapshot of the tag
func (t *Tag) ToSnapshot() TagSnapshot {
	return TagSnapshot{
		ID:        t.id,
		Name:      t.name,
		Category:  t.category,
		Value:     t.value,
		CreatedAt: t.createdAt,
	}
}

// ReconstructTag reconstructs a tag from persistence data (for use by repository)
func ReconstructTag(id TagID, name, category, value string, createdAt time.Time) *Tag {
	return &Tag{
		id:        id,
		name:      name,
		category:  category,
		value:     value,
		createdAt: createdAt,
	}
}
