package integrations_test

import (
	"testing"

	"github.com/guidewire-oss/fern-platform/internal/domains/integrations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// validEntries returns a minimal set of entries that satisfies all invariants:
// required fields (RequirementID, RequirementTitle) are mapped, no duplicates,
// and no single-value Fern field is mapped to a multi-value JIRA field without
// a reduction strategy.
func validEntries() []integrations.FieldMappingEntry {
	return []integrations.FieldMappingEntry{
		{FernField: integrations.FernFieldRequirementID, JiraFieldID: "summary"},
		{FernField: integrations.FernFieldRequirementTitle, JiraFieldID: "customfield_10001"},
	}
}

func TestNewJiraFieldMapping(t *testing.T) {
	t.Run("rejects empty projectID", func(t *testing.T) {
		mapping, err := integrations.NewJiraFieldMapping("", validEntries(), "user@example.com")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "project ID is required")
		assert.Nil(t, mapping)
	})

	t.Run("rejects duplicate JiraFieldID across entries", func(t *testing.T) {
		entries := []integrations.FieldMappingEntry{
			{FernField: integrations.FernFieldRequirementID, JiraFieldID: "summary"},
			{FernField: integrations.FernFieldRequirementTitle, JiraFieldID: "summary"}, // duplicate
		}
		mapping, err := integrations.NewJiraFieldMapping("proj-123", entries, "user@example.com")
		assert.ErrorIs(t, err, integrations.ErrDuplicateJiraField)
		assert.Nil(t, mapping)
	})

	t.Run("rejects empty JiraFieldID on a required FernField", func(t *testing.T) {
		entries := []integrations.FieldMappingEntry{
			{FernField: integrations.FernFieldRequirementID, JiraFieldID: ""},              // required, unmapped
			{FernField: integrations.FernFieldRequirementTitle, JiraFieldID: "customfield_10001"},
		}
		mapping, err := integrations.NewJiraFieldMapping("proj-123", entries, "user@example.com")
		assert.ErrorIs(t, err, integrations.ErrRequiredFieldUnmapped)
		assert.Nil(t, mapping)
	})

	t.Run("rejects single-value FernField mapped to multi-value JIRA field without ReductionStrategy", func(t *testing.T) {
		// FernFieldReleaseVersion is single-value; mapping it to a multi-value JIRA
		// field (e.g. "labels") without a reduction strategy must be rejected.
		entries := []integrations.FieldMappingEntry{
			{FernField: integrations.FernFieldRequirementID, JiraFieldID: "summary"},
			{FernField: integrations.FernFieldRequirementTitle, JiraFieldID: "customfield_10001"},
			{
				FernField:            integrations.FernFieldReleaseVersion,
				JiraFieldID:          "labels",
				JiraFieldIsMultiValue: true,
				// ReductionStrategy intentionally omitted
			},
		}
		mapping, err := integrations.NewJiraFieldMapping("proj-123", entries, "user@example.com")
		assert.ErrorIs(t, err, integrations.ErrMissingReductionStrategy)
		assert.Nil(t, mapping)
	})

	t.Run("succeeds with all required fields mapped", func(t *testing.T) {
		mapping, err := integrations.NewJiraFieldMapping("proj-123", validEntries(), "user@example.com")
		require.NoError(t, err)
		require.NotNil(t, mapping)
	})

	t.Run("allows unmapped optional fields", func(t *testing.T) {
		// Only required fields are mapped; optional fields with empty JiraFieldID are fine.
		entries := []integrations.FieldMappingEntry{
			{FernField: integrations.FernFieldRequirementID, JiraFieldID: "summary"},
			{FernField: integrations.FernFieldRequirementTitle, JiraFieldID: "customfield_10001"},
			{FernField: integrations.FernFieldDescription, JiraFieldID: ""},    // optional, unmapped — OK
			{FernField: integrations.FernFieldReleaseVersion, JiraFieldID: ""}, // optional, unmapped — OK
		}
		mapping, err := integrations.NewJiraFieldMapping("proj-123", entries, "user@example.com")
		require.NoError(t, err)
		require.NotNil(t, mapping)
	})

	t.Run("allows multi-value FernField mapped to multi-value JIRA field without ReductionStrategy", func(t *testing.T) {
		// FernFieldTags is multi-value so no reduction strategy is needed even when the
		// JIRA field is also multi-value.
		entries := []integrations.FieldMappingEntry{
			{FernField: integrations.FernFieldRequirementID, JiraFieldID: "summary"},
			{FernField: integrations.FernFieldRequirementTitle, JiraFieldID: "customfield_10001"},
			{
				FernField:            integrations.FernFieldTags,
				JiraFieldID:          "labels",
				JiraFieldIsMultiValue: true,
				// ReductionStrategy intentionally omitted — multi-value → multi-value is fine
			},
		}
		mapping, err := integrations.NewJiraFieldMapping("proj-123", entries, "user@example.com")
		require.NoError(t, err)
		require.NotNil(t, mapping)
	})
}

func TestJiraFieldMapping_UpdateEntries(t *testing.T) {
	t.Run("replaces entries and updates UpdatedBy", func(t *testing.T) {
		mapping, err := integrations.NewJiraFieldMapping("proj-123", validEntries(), "creator@example.com")
		require.NoError(t, err)

		newEntries := []integrations.FieldMappingEntry{
			{FernField: integrations.FernFieldRequirementID, JiraFieldID: "issueid"},
			{FernField: integrations.FernFieldRequirementTitle, JiraFieldID: "customfield_20001"},
		}
		err = mapping.UpdateEntries(newEntries, "updater@example.com")
		require.NoError(t, err)

		snap := mapping.Snapshot()
		assert.Equal(t, "updater@example.com", snap.UpdatedBy)
		require.Len(t, snap.Entries, 2)
		assert.Equal(t, integrations.FernFieldRequirementID, snap.Entries[0].FernField)
		assert.Equal(t, "issueid", snap.Entries[0].JiraFieldID)
	})

	t.Run("rejects update with duplicate JiraFieldID", func(t *testing.T) {
		mapping, err := integrations.NewJiraFieldMapping("proj-123", validEntries(), "creator@example.com")
		require.NoError(t, err)

		duplicateEntries := []integrations.FieldMappingEntry{
			{FernField: integrations.FernFieldRequirementID, JiraFieldID: "summary"},
			{FernField: integrations.FernFieldRequirementTitle, JiraFieldID: "summary"}, // duplicate
		}
		err = mapping.UpdateEntries(duplicateEntries, "updater@example.com")
		assert.ErrorIs(t, err, integrations.ErrDuplicateJiraField)
	})

	t.Run("rejects update with required field unmapped", func(t *testing.T) {
		mapping, err := integrations.NewJiraFieldMapping("proj-123", validEntries(), "creator@example.com")
		require.NoError(t, err)

		missingRequired := []integrations.FieldMappingEntry{
			{FernField: integrations.FernFieldRequirementID, JiraFieldID: ""},              // required, unmapped
			{FernField: integrations.FernFieldRequirementTitle, JiraFieldID: "customfield_10001"},
		}
		err = mapping.UpdateEntries(missingRequired, "updater@example.com")
		assert.ErrorIs(t, err, integrations.ErrRequiredFieldUnmapped)
	})
}

func TestValidateEntries_UnknownEnums(t *testing.T) {
	t.Run("rejects unknown FernField value", func(t *testing.T) {
		entries := []integrations.FieldMappingEntry{
			{FernField: integrations.FernField("not_a_real_field"), JiraFieldID: "summary"},
		}
		_, err := integrations.NewJiraFieldMapping("proj-1", entries, "user")
		assert.ErrorIs(t, err, integrations.ErrUnknownFernField)
	})

	t.Run("rejects unknown ReductionStrategy value", func(t *testing.T) {
		entries := []integrations.FieldMappingEntry{
			{FernField: integrations.FernFieldRequirementID, JiraFieldID: "issuekey"},
			{FernField: integrations.FernFieldRequirementTitle, JiraFieldID: "summary"},
			{
				FernField:             integrations.FernFieldDescription,
				JiraFieldID:           "labels",
				JiraFieldIsMultiValue: true,
				ReductionStrategy:     integrations.ReductionStrategy("bad_strategy"),
			},
		}
		_, err := integrations.NewJiraFieldMapping("proj-1", entries, "user")
		assert.ErrorIs(t, err, integrations.ErrUnknownReductionStrategy)
	})
}

func TestFernField_IsValid(t *testing.T) {
	for _, f := range integrations.AllFernFields() {
		assert.True(t, f.IsValid(), "expected %q to be valid", f)
	}
	assert.False(t, integrations.FernField("").IsValid())
	assert.False(t, integrations.FernField("unknown_field").IsValid())
}

func TestReductionStrategy_IsValid(t *testing.T) {
	valid := []integrations.ReductionStrategy{
		integrations.ReductionStrategyFirstValue,
		integrations.ReductionStrategyConcatenate,
		integrations.ReductionStrategySeparate,
	}
	for _, s := range valid {
		assert.True(t, s.IsValid(), "expected %q to be valid", s)
	}
	assert.False(t, integrations.ReductionStrategy("").IsValid())
	assert.False(t, integrations.ReductionStrategy("bad_strategy").IsValid())
}

func TestJiraFieldMapping_Snapshot(t *testing.T) {
	t.Run("snapshot reflects the current state", func(t *testing.T) {
		entries := []integrations.FieldMappingEntry{
			{FernField: integrations.FernFieldRequirementID, JiraFieldID: "summary"},
			{FernField: integrations.FernFieldRequirementTitle, JiraFieldID: "customfield_10001"},
			{FernField: integrations.FernFieldDescription, JiraFieldID: "description"},
		}
		mapping, err := integrations.NewJiraFieldMapping("proj-456", entries, "author@example.com")
		require.NoError(t, err)

		snap := mapping.Snapshot()
		assert.Equal(t, "proj-456", snap.ProjectID)
		assert.Equal(t, "author@example.com", snap.UpdatedBy)
		require.Len(t, snap.Entries, 3)
		assert.Equal(t, integrations.FernFieldRequirementID, snap.Entries[0].FernField)
		assert.Equal(t, "summary", snap.Entries[0].JiraFieldID)
	})

	t.Run("snapshot ProjectID matches constructor arg", func(t *testing.T) {
		mapping, err := integrations.NewJiraFieldMapping("my-project", validEntries(), "user@example.com")
		require.NoError(t, err)

		snap := mapping.Snapshot()
		assert.Equal(t, "my-project", snap.ProjectID)
	})
}
