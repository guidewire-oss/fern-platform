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

	normalizedName, category, value := ParseTagName(name)
	if normalizedName == "" {
		return nil, errors.New("tag name cannot be empty after normalization")
	}

	return &Tag{
		id:        TagID(""), // ID will be assigned by database
		name:      normalizedName,
		category:  category,
		value:     value,
		createdAt: time.Now(),
	}, nil
}

// ParseTagName normalizes a raw tag string into its stored name, category and
// value. It is the single place tag names are interpreted, so lookups and
// creation agree on what an incoming tag is called.
//
// The name is trimmed and lowercased. It is split on the first colon into
// category and value; with no colon the whole input is the value and the
// category is empty. A single pair of matching quotes (' or ") wrapping the
// value is stripped, e.g. labels:"a || b" -> labels / a || b. The name is only
// rebuilt as category:value when a quote was stripped, so existing names keep
// their original spelling.
func ParseTagName(raw string) (name, category, value string) {
	name = strings.TrimSpace(strings.ToLower(raw))
	if name == "" {
		return "", "", ""
	}

	hasColon := false
	rawValue := name
	if idx := strings.Index(name, ":"); idx != -1 {
		hasColon = true
		category = strings.TrimSpace(name[:idx])
		rawValue = strings.TrimSpace(name[idx+1:])
	}

	value = stripWrappingQuotes(rawValue)
	if value == rawValue {
		return name, category, value
	}

	if !hasColon {
		return value, "", value
	}
	return category + ":" + value, category, value
}

// stripWrappingQuotes removes one pair of matching quotes around the whole
// string. Inner quotes, unmatched quotes and an empty pair ("") are left alone,
// so a value never silently becomes empty.
func stripWrappingQuotes(s string) string {
	if len(s) < 3 {
		return s
	}
	first, last := s[0], s[len(s)-1]
	if first != last || (first != '"' && first != '\'') {
		return s
	}
	if inner := strings.TrimSpace(s[1 : len(s)-1]); inner != "" {
		return inner
	}
	return s
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
