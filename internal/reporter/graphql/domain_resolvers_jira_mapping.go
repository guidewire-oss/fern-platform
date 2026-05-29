package graphql

import (
	"context"
	"errors"
	"fmt"

	authDomain "github.com/guidewire-oss/fern-platform/internal/domains/auth/domain"
	"github.com/guidewire-oss/fern-platform/internal/domains/integrations"
	projectsDomain "github.com/guidewire-oss/fern-platform/internal/domains/projects/domain"
	"github.com/guidewire-oss/fern-platform/internal/reporter/graphql/model"
)

// authorizeProjectManagement enforces the canonical per-project authorization
// used by every JIRA-related resolver. The caller must be authenticated AND
// hold at least one project-level permission row for `projectID` — global
// roles alone are not sufficient. Mirrors the pattern in
// CreateJiraConnection / UpdateJiraConnection (schema.resolvers.go).
func (r *Resolver) authorizeProjectManagement(ctx context.Context, projectID string) (*authDomain.User, error) {
	user, err := getCurrentUser(ctx)
	if err != nil || user == nil {
		return nil, fmt.Errorf("unauthorized")
	}

	project, err := r.projectService.GetProject(ctx, projectsDomain.ProjectID(projectID))
	if err != nil {
		if errors.Is(err, projectsDomain.ErrProjectNotFound) {
			return nil, err
		}
		return nil, fmt.Errorf("failed to get project: %w", err)
	}

	permissions, err := r.projectService.GetUserPermissions(ctx, project.ProjectID(), user.UserID)
	if err != nil || len(permissions) == 0 {
		return nil, fmt.Errorf("forbidden")
	}

	return user, nil
}

// JiraFieldMapping_domain returns the field mapping snapshot for the project.
func (r *queryResolver) JiraFieldMapping_domain(ctx context.Context, projectID string) (*model.JiraFieldMapping, error) {
	if _, err := r.authorizeProjectManagement(ctx, projectID); err != nil {
		return nil, err
	}

	snap, err := r.jiraFieldMappingService.Get(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to get JIRA field mapping: %w", err)
	}

	return mappingSnapshotToModel(snap)
}

// JiraFields_domain lists all JIRA fields available for a given connection.
// Authorization is scoped to the project that owns the connection.
func (r *queryResolver) JiraFields_domain(ctx context.Context, connectionID string) ([]*model.JiraFieldGql, error) {
	connection, err := r.jiraConnectionService.GetConnection(ctx, connectionID)
	if err != nil || connection == nil {
		return nil, fmt.Errorf("connection not found")
	}

	if _, err := r.authorizeProjectManagement(ctx, connection.ProjectID()); err != nil {
		return nil, err
	}

	fields, err := r.jiraConnectionService.ListJiraFields(ctx, connectionID)
	if err != nil {
		return nil, fmt.Errorf("failed to list JIRA fields: %w", err)
	}

	result := make([]*model.JiraFieldGql, len(fields))
	for i, f := range fields {
		result[i] = &model.JiraFieldGql{
			ID:         f.ID,
			Name:       f.Name,
			Custom:     f.Custom,
			MultiValue: f.MultiValue,
		}
	}
	return result, nil
}

// SaveJiraFieldMapping_domain validates and persists a field mapping for the project.
func (r *mutationResolver) SaveJiraFieldMapping_domain(ctx context.Context, input model.SaveJiraFieldMappingInput) (*model.JiraFieldMapping, error) {
	user, err := r.authorizeProjectManagement(ctx, input.ProjectID)
	if err != nil {
		return nil, err
	}

	entries := modelEntriesToDomain(input.Entries)
	snap, err := r.jiraFieldMappingService.Save(ctx, input.ProjectID, entries, user.UserID)
	if err != nil {
		switch {
		case errors.Is(err, integrations.ErrNoJiraConnection):
			return nil, fmt.Errorf("no JIRA connection configured for this project: %w", err)
		case errors.Is(err, integrations.ErrRequiredFieldUnmapped):
			return nil, fmt.Errorf("required Fern field is unmapped: %w", err)
		case errors.Is(err, integrations.ErrDuplicateJiraField):
			return nil, fmt.Errorf("duplicate JIRA field in mapping: %w", err)
		case errors.Is(err, integrations.ErrDuplicateFernField):
			return nil, fmt.Errorf("Fern field appears more than once in the mapping: %w", err)
		case errors.Is(err, integrations.ErrMissingReductionStrategy):
			return nil, fmt.Errorf("multi-value JIRA field requires a reduction strategy: %w", err)
		case errors.Is(err, integrations.ErrUnknownFernField):
			return nil, fmt.Errorf("unknown Fern field in mapping: %w", err)
		case errors.Is(err, integrations.ErrUnknownReductionStrategy):
			return nil, fmt.Errorf("unknown reduction strategy in mapping: %w", err)
		default:
			return nil, fmt.Errorf("failed to save JIRA field mapping: %w", err)
		}
	}

	return mappingSnapshotToModel(snap)
}

// ResetJiraFieldMapping_domain deletes any saved mapping for the project and
// returns the default mapping snapshot.
func (r *mutationResolver) ResetJiraFieldMapping_domain(ctx context.Context, projectID string) (*model.JiraFieldMapping, error) {
	if _, err := r.authorizeProjectManagement(ctx, projectID); err != nil {
		return nil, err
	}

	snap, err := r.jiraFieldMappingService.Reset(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to reset JIRA field mapping: %w", err)
	}

	return mappingSnapshotToModel(snap)
}

// ---------------------------------------------------------------------------
// Model conversion helpers
// ---------------------------------------------------------------------------

// mappingSnapshotToModel converts a domain snapshot to the GraphQL model type.
func mappingSnapshotToModel(snap *integrations.JiraFieldMappingSnapshot) (*model.JiraFieldMapping, error) {
	entries := make([]*model.FieldMappingEntry, len(snap.Entries))
	for i, e := range snap.Entries {
		mf, err := fernFieldToModel(e.FernField)
		if err != nil {
			return nil, err
		}
		entries[i] = &model.FieldMappingEntry{
			FernField:             mf,
			JiraFieldID:           e.JiraFieldID,
			JiraFieldIsMultiValue: e.JiraFieldIsMultiValue,
			ReductionStrategy:     reductionStrategyToModel(e.ReductionStrategy),
		}
	}
	result := &model.JiraFieldMapping{
		ProjectID: snap.ProjectID,
		Entries:   entries,
	}
	if snap.UpdatedBy != "" {
		result.UpdatedBy = &snap.UpdatedBy
	}
	if !snap.UpdatedAt.IsZero() {
		result.UpdatedAt = &snap.UpdatedAt
	}
	return result, nil
}

// fernFieldToModel maps a domain FernField constant to its GraphQL model enum.
func fernFieldToModel(f integrations.FernField) (model.FernField, error) {
	switch f {
	case integrations.FernFieldRequirementID:
		return model.FernFieldRequirementID, nil
	case integrations.FernFieldRequirementTitle:
		return model.FernFieldRequirementTitle, nil
	case integrations.FernFieldDescription:
		return model.FernFieldDescription, nil
	case integrations.FernFieldParentRequirement:
		return model.FernFieldParentRequirement, nil
	case integrations.FernFieldRequirementType:
		return model.FernFieldRequirementType, nil
	case integrations.FernFieldReleaseVersion:
		return model.FernFieldReleaseVersion, nil
	case integrations.FernFieldRequirementStatus:
		return model.FernFieldRequirementStatus, nil
	case integrations.FernFieldTags:
		return model.FernFieldTags, nil
	default:
		return "", fmt.Errorf("unhandled FernField %q — update fernFieldToModel when adding new constants", f)
	}
}

// reductionStrategyToModel maps a domain ReductionStrategy to the GraphQL model enum pointer.
// Returns nil when the domain value is the zero value (no strategy set).
func reductionStrategyToModel(r integrations.ReductionStrategy) *model.ReductionStrategy {
	var s model.ReductionStrategy
	switch r {
	case integrations.ReductionStrategyFirstValue:
		s = model.ReductionStrategyFirstValue
	case integrations.ReductionStrategyConcatenate:
		s = model.ReductionStrategyConcatenate
	case integrations.ReductionStrategySeparate:
		s = model.ReductionStrategySeparateEntries
	default:
		return nil
	}
	return &s
}

// modelToFernField maps a GraphQL model FernField enum to the domain constant.
func modelToFernField(f model.FernField) integrations.FernField {
	switch f {
	case model.FernFieldRequirementID:
		return integrations.FernFieldRequirementID
	case model.FernFieldRequirementTitle:
		return integrations.FernFieldRequirementTitle
	case model.FernFieldDescription:
		return integrations.FernFieldDescription
	case model.FernFieldParentRequirement:
		return integrations.FernFieldParentRequirement
	case model.FernFieldRequirementType:
		return integrations.FernFieldRequirementType
	case model.FernFieldReleaseVersion:
		return integrations.FernFieldReleaseVersion
	case model.FernFieldRequirementStatus:
		return integrations.FernFieldRequirementStatus
	case model.FernFieldTags:
		return integrations.FernFieldTags
	default:
		panic(fmt.Sprintf("unhandled model.FernField %q — update modelToFernField when adding new constants", f))
	}
}

// modelToReductionStrategy maps a GraphQL model ReductionStrategy pointer to the domain constant.
// Returns "" when r is nil (no strategy set).
func modelToReductionStrategy(r *model.ReductionStrategy) integrations.ReductionStrategy {
	if r == nil {
		return ""
	}
	switch *r {
	case model.ReductionStrategyFirstValue:
		return integrations.ReductionStrategyFirstValue
	case model.ReductionStrategyConcatenate:
		return integrations.ReductionStrategyConcatenate
	case model.ReductionStrategySeparateEntries:
		return integrations.ReductionStrategySeparate
	default:
		panic(fmt.Sprintf("unhandled model.ReductionStrategy %q — update modelToReductionStrategy when adding new constants", *r))
	}
}

// modelEntriesToDomain converts a slice of GraphQL input entries to domain entries.
func modelEntriesToDomain(entries []*model.FieldMappingEntryInput) []integrations.FieldMappingEntry {
	result := make([]integrations.FieldMappingEntry, len(entries))
	for i, e := range entries {
		result[i] = integrations.FieldMappingEntry{
			FernField:             modelToFernField(e.FernField),
			JiraFieldID:           e.JiraFieldID,
			JiraFieldIsMultiValue: e.JiraFieldIsMultiValue,
			ReductionStrategy:     modelToReductionStrategy(e.ReductionStrategy),
		}
	}
	return result
}
