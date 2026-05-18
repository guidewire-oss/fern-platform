package integrations

import (
	"errors"
	"time"
)

// FieldMappingEntry maps a single Fern field to a JIRA field.
type FieldMappingEntry struct {
	FernField            FernField
	JiraFieldID          string
	JiraFieldIsMultiValue bool
	ReductionStrategy    ReductionStrategy
}

// JiraFieldMappingSnapshot is a read-only view of a JiraFieldMapping.
type JiraFieldMappingSnapshot struct {
	ProjectID string
	Entries   []FieldMappingEntry
	UpdatedBy string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// JiraFieldMapping is the aggregate for a project's Fern-to-JIRA field mapping.
type JiraFieldMapping struct {
	projectID string
	entries   []FieldMappingEntry
	updatedBy string
	createdAt time.Time
	updatedAt time.Time
}

// NewJiraFieldMapping creates a validated JiraFieldMapping aggregate.
func NewJiraFieldMapping(projectID string, entries []FieldMappingEntry, updatedBy string) (*JiraFieldMapping, error) {
	if projectID == "" {
		return nil, errors.New("project ID is required")
	}
	if err := validateEntries(entries); err != nil {
		return nil, err
	}
	copied := make([]FieldMappingEntry, len(entries))
	copy(copied, entries)
	now := time.Now()
	return &JiraFieldMapping{
		projectID: projectID,
		entries:   copied,
		updatedBy: updatedBy,
		createdAt: now,
		updatedAt: now,
	}, nil
}

// ReconstructJiraFieldMapping rebuilds the aggregate from persisted data without
// running constructor validation (used by the repository layer only).
func ReconstructJiraFieldMapping(projectID string, entries []FieldMappingEntry, updatedBy string, createdAt, updatedAt time.Time) *JiraFieldMapping {
	copied := make([]FieldMappingEntry, len(entries))
	copy(copied, entries)
	return &JiraFieldMapping{
		projectID: projectID,
		entries:   copied,
		updatedBy: updatedBy,
		createdAt: createdAt,
		updatedAt: updatedAt,
	}
}

// UpdateEntries replaces the mapping entries after validating the new set.
func (m *JiraFieldMapping) UpdateEntries(entries []FieldMappingEntry, updatedBy string) error {
	if err := validateEntries(entries); err != nil {
		return err
	}
	copied := make([]FieldMappingEntry, len(entries))
	copy(copied, entries)
	m.entries = copied
	m.updatedBy = updatedBy
	m.updatedAt = time.Now()
	return nil
}

// Snapshot returns a read-only copy of the aggregate state.
func (m *JiraFieldMapping) Snapshot() JiraFieldMappingSnapshot {
	entries := make([]FieldMappingEntry, len(m.entries))
	copy(entries, m.entries)
	return JiraFieldMappingSnapshot{
		ProjectID: m.projectID,
		Entries:   entries,
		UpdatedBy: m.updatedBy,
		CreatedAt: m.createdAt,
		UpdatedAt: m.updatedAt,
	}
}

// validateEntries checks all business invariants for a set of mapping entries.
func validateEntries(entries []FieldMappingEntry) error {
	seen := map[string]bool{}
	seenFernFields := map[FernField]bool{}
	for _, e := range entries {
		if !e.FernField.IsValid() {
			return ErrUnknownFernField
		}
		if e.ReductionStrategy != "" && !e.ReductionStrategy.IsValid() {
			return ErrUnknownReductionStrategy
		}
		if seenFernFields[e.FernField] {
			return ErrDuplicateFernField
		}
		seenFernFields[e.FernField] = true
		if e.JiraFieldID == "" {
			if e.FernField.IsRequired() {
				return ErrRequiredFieldUnmapped
			}
			continue
		}
		if seen[e.JiraFieldID] {
			return ErrDuplicateJiraField
		}
		seen[e.JiraFieldID] = true
		if e.JiraFieldIsMultiValue && !e.FernField.IsMultiValue() && e.ReductionStrategy == "" {
			return ErrMissingReductionStrategy
		}
	}
	for _, f := range AllFernFields() {
		if f.IsRequired() && !seenFernFields[f] {
			return ErrRequiredFieldUnmapped
		}
	}
	return nil
}
