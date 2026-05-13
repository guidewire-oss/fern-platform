package integrations

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
)

var defaultFernToJiraMapping = map[FernField]string{
	FernFieldRequirementID:     "issuekey",
	FernFieldRequirementTitle:  "summary",
	FernFieldDescription:       "description",
	FernFieldParentRequirement: "parent",
	FernFieldRequirementType:   "issuetype",
	FernFieldRequirementStatus: "status",
	FernFieldTags:              "labels",
	FernFieldReleaseVersion:    "",
}

func init() {
	for _, f := range AllFernFields() {
		if _, ok := defaultFernToJiraMapping[f]; !ok {
			panic(fmt.Sprintf("defaultFernToJiraMapping missing entry for FernField %q — update the map when adding new constants", f))
		}
	}
}

type JiraFieldMappingService struct {
	mappingRepo JiraFieldMappingRepository
	connRepo    JiraConnectionRepository
}

func NewJiraFieldMappingService(mappingRepo JiraFieldMappingRepository, connRepo JiraConnectionRepository) *JiraFieldMappingService {
	return &JiraFieldMappingService{
		mappingRepo: mappingRepo,
		connRepo:    connRepo,
	}
}

// Get returns the saved mapping for the project, or sensible defaults when none exists.
func (s *JiraFieldMappingService) Get(ctx context.Context, projectID string) (*JiraFieldMappingSnapshot, error) {
	mapping, err := s.mappingRepo.Get(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("retrieving field mapping for project %s: %w", projectID, err)
	}
	if mapping == nil {
		return s.defaultSnapshot(projectID), nil
	}
	snap := mapping.Snapshot()
	return &snap, nil
}

// Save validates and persists a field mapping for the project.
func (s *JiraFieldMappingService) Save(ctx context.Context, projectID string, entries []FieldMappingEntry, updatedBy string) (*JiraFieldMappingSnapshot, error) {
	conns, err := s.connRepo.FindByProjectID(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("checking JIRA connection: %w", err)
	}
	if len(conns) == 0 {
		logrus.WithField("project_id", projectID).Warn("save field mapping rejected: no JIRA connection configured")
		return nil, ErrNoJiraConnection
	}

	mapping, err := NewJiraFieldMapping(projectID, entries, updatedBy)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"project_id": projectID,
			"error":      err.Error(),
		}).Warn("save field mapping rejected: validation failed")
		return nil, err
	}

	if err := s.mappingRepo.Upsert(ctx, mapping); err != nil {
		logrus.WithError(err).WithField("project_id", projectID).Error("failed to persist JIRA field mapping")
		return nil, fmt.Errorf("persisting field mapping: %w", err)
	}

	snap := mapping.Snapshot()
	return &snap, nil
}

// Reset deletes any saved field mapping for the project and returns the default
// mapping snapshot.  It does NOT require an active JIRA connection.
func (s *JiraFieldMappingService) Reset(ctx context.Context, projectID string) (*JiraFieldMappingSnapshot, error) {
	if err := s.mappingRepo.Delete(ctx, projectID); err != nil {
		return nil, fmt.Errorf("failed to delete field mapping: %w", err)
	}
	return s.defaultSnapshot(projectID), nil
}

func (s *JiraFieldMappingService) defaultSnapshot(projectID string) *JiraFieldMappingSnapshot {
	entries := make([]FieldMappingEntry, 0, len(defaultFernToJiraMapping))
	for _, f := range AllFernFields() {
		entries = append(entries, FieldMappingEntry{
			FernField:   f,
			JiraFieldID: defaultFernToJiraMapping[f],
		})
	}
	return &JiraFieldMappingSnapshot{
		ProjectID: projectID,
		Entries:   entries,
	}
}
