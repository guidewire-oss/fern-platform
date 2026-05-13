package integrations_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/guidewire-oss/fern-platform/internal/domains/integrations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Fakes
// ---------------------------------------------------------------------------

type fakeFieldMappingRepo struct {
	mapping *integrations.JiraFieldMapping
	err     error
	upserted *integrations.JiraFieldMapping
	deleted  string
}

func (f *fakeFieldMappingRepo) Get(ctx context.Context, projectID string) (*integrations.JiraFieldMapping, error) {
	return f.mapping, f.err
}

func (f *fakeFieldMappingRepo) Upsert(ctx context.Context, mapping *integrations.JiraFieldMapping) error {
	if f.err != nil {
		return f.err
	}
	f.upserted = mapping
	return nil
}

func (f *fakeFieldMappingRepo) Delete(ctx context.Context, projectID string) error {
	if f.err != nil {
		return f.err
	}
	f.deleted = projectID
	return nil
}

// fakeConnectionRepo implements JiraConnectionRepository.
type fakeConnectionRepo struct {
	connection *integrations.JiraConnection
	err        error
}

func (f *fakeConnectionRepo) Create(ctx context.Context, connection *integrations.JiraConnection) error {
	return f.err
}

func (f *fakeConnectionRepo) Update(ctx context.Context, connection *integrations.JiraConnection) error {
	return f.err
}

func (f *fakeConnectionRepo) Delete(ctx context.Context, connectionID string) error {
	return f.err
}

func (f *fakeConnectionRepo) FindByID(ctx context.Context, connectionID string) (*integrations.JiraConnection, error) {
	return f.connection, f.err
}

func (f *fakeConnectionRepo) FindByProjectID(ctx context.Context, projectID string) ([]*integrations.JiraConnection, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.connection == nil {
		return nil, nil
	}
	return []*integrations.JiraConnection{f.connection}, nil
}

func (f *fakeConnectionRepo) FindActiveByProjectID(ctx context.Context, projectID string) ([]*integrations.JiraConnection, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.connection == nil || !f.connection.IsActive() {
		return nil, nil
	}
	return []*integrations.JiraConnection{f.connection}, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// activeConnection builds a minimal JiraConnection that IsActive() == true.
func activeConnection(t *testing.T) *integrations.JiraConnection {
	t.Helper()
	conn, err := integrations.NewJiraConnection(
		"proj-1", "test conn", "https://jira.example.com",
		integrations.AuthTypeAPIToken, "PROJ", "user@example.com", "tok",
	)
	require.NoError(t, err)
	conn.Activate()
	return conn
}

// minimalValidEntries returns a minimal set of entries that pass validateEntries
// (required fields mapped, no duplicates).
func minimalValidEntries() []integrations.FieldMappingEntry {
	return []integrations.FieldMappingEntry{
		{FernField: integrations.FernFieldRequirementID, JiraFieldID: "issuekey"},
		{FernField: integrations.FernFieldRequirementTitle, JiraFieldID: "summary"},
	}
}

// ---------------------------------------------------------------------------
// TestJiraFieldMappingService_Get
// ---------------------------------------------------------------------------

func TestJiraFieldMappingService_Get(t *testing.T) {
	t.Run("returns default snapshot when repo returns nil", func(t *testing.T) {
		svc := integrations.NewJiraFieldMappingService(
			&fakeFieldMappingRepo{mapping: nil},
			&fakeConnectionRepo{},
		)

		snap, err := svc.Get(context.Background(), "proj-1")
		require.NoError(t, err)
		require.NotNil(t, snap)

		// Default mappings must be present for all required fields.
		byFernField := indexByFernField(snap.Entries)

		assert.Equal(t, "issuekey", byFernField[integrations.FernFieldRequirementID].JiraFieldID)
		assert.Equal(t, "summary", byFernField[integrations.FernFieldRequirementTitle].JiraFieldID)
		assert.Equal(t, "description", byFernField[integrations.FernFieldDescription].JiraFieldID)
		assert.Equal(t, "issuetype", byFernField[integrations.FernFieldRequirementType].JiraFieldID)
		assert.Equal(t, "status", byFernField[integrations.FernFieldRequirementStatus].JiraFieldID)
		assert.Equal(t, "labels", byFernField[integrations.FernFieldTags].JiraFieldID)
		// FernFieldReleaseVersion should be unmapped (empty JiraFieldID).
		assert.Equal(t, "", byFernField[integrations.FernFieldReleaseVersion].JiraFieldID)
		// FernFieldParentRequirement should have a non-empty default.
		assert.NotEmpty(t, byFernField[integrations.FernFieldParentRequirement].JiraFieldID)
	})

	t.Run("returns saved snapshot when repo has a mapping", func(t *testing.T) {
		saved := integrations.ReconstructJiraFieldMapping(
			"proj-1",
			[]integrations.FieldMappingEntry{
				{FernField: integrations.FernFieldRequirementID, JiraFieldID: "issuekey"},
				{FernField: integrations.FernFieldRequirementTitle, JiraFieldID: "summary"},
				{FernField: integrations.FernFieldDescription, JiraFieldID: "customfield_001"},
			},
			"alice",
			time.Now().Add(-time.Hour),
			time.Now(),
		)

		svc := integrations.NewJiraFieldMappingService(
			&fakeFieldMappingRepo{mapping: saved},
			&fakeConnectionRepo{},
		)

		snap, err := svc.Get(context.Background(), "proj-1")
		require.NoError(t, err)
		require.NotNil(t, snap)

		assert.Equal(t, "proj-1", snap.ProjectID)
		assert.Equal(t, "alice", snap.UpdatedBy)

		byFernField := indexByFernField(snap.Entries)
		assert.Equal(t, "customfield_001", byFernField[integrations.FernFieldDescription].JiraFieldID)
	})

	t.Run("propagates repo error", func(t *testing.T) {
		repoErr := errors.New("database unavailable")
		svc := integrations.NewJiraFieldMappingService(
			&fakeFieldMappingRepo{err: repoErr},
			&fakeConnectionRepo{},
		)

		_, err := svc.Get(context.Background(), "proj-1")
		require.Error(t, err)
		assert.ErrorIs(t, err, repoErr)
	})
}

// ---------------------------------------------------------------------------
// TestJiraFieldMappingService_Save
// ---------------------------------------------------------------------------

func TestJiraFieldMappingService_Save(t *testing.T) {
	t.Run("returns ErrNoJiraConnection when no connection exists for project", func(t *testing.T) {
		svc := integrations.NewJiraFieldMappingService(
			&fakeFieldMappingRepo{},
			&fakeConnectionRepo{connection: nil},
		)

		_, err := svc.Save(context.Background(), "proj-1", minimalValidEntries(), "bob")
		require.Error(t, err)
		assert.ErrorIs(t, err, integrations.ErrNoJiraConnection)
	})

	t.Run("saves mapping even when connection exists but has not been tested yet", func(t *testing.T) {
		conn, err := integrations.NewJiraConnection(
			"proj-1", "test conn", "https://jira.example.com",
			integrations.AuthTypeAPIToken, "PROJ", "user@example.com", "tok",
		)
		require.NoError(t, err)
		// conn.IsActive() == false by default — mapping save should still succeed.

		repo := &fakeFieldMappingRepo{}
		svc := integrations.NewJiraFieldMappingService(
			repo,
			&fakeConnectionRepo{connection: conn},
		)

		snap, err := svc.Save(context.Background(), "proj-1", minimalValidEntries(), "bob")
		require.NoError(t, err)
		assert.NotNil(t, snap)
	})

	t.Run("rejects invalid entries and returns typed error", func(t *testing.T) {
		// An entry where a required field (RequirementID) has an empty JiraFieldID
		// triggers ErrRequiredFieldUnmapped from the aggregate.
		invalidEntries := []integrations.FieldMappingEntry{
			{FernField: integrations.FernFieldRequirementID, JiraFieldID: ""},
			{FernField: integrations.FernFieldRequirementTitle, JiraFieldID: "summary"},
		}

		svc := integrations.NewJiraFieldMappingService(
			&fakeFieldMappingRepo{},
			&fakeConnectionRepo{connection: activeConnection(t)},
		)

		_, err := svc.Save(context.Background(), "proj-1", invalidEntries, "bob")
		require.Error(t, err)
		assert.ErrorIs(t, err, integrations.ErrRequiredFieldUnmapped)
	})

	t.Run("saves valid mapping and returns snapshot", func(t *testing.T) {
		repo := &fakeFieldMappingRepo{}
		svc := integrations.NewJiraFieldMappingService(
			repo,
			&fakeConnectionRepo{connection: activeConnection(t)},
		)

		snap, err := svc.Save(context.Background(), "proj-1", minimalValidEntries(), "bob")
		require.NoError(t, err)
		require.NotNil(t, snap)

		assert.Equal(t, "proj-1", snap.ProjectID)
		assert.NotNil(t, repo.upserted, "Upsert should have been called")
	})

	t.Run("propagates repo Upsert error", func(t *testing.T) {
		upsertErr := errors.New("database unavailable")
		svc := integrations.NewJiraFieldMappingService(
			&fakeFieldMappingRepo{err: upsertErr},
			&fakeConnectionRepo{connection: activeConnection(t)},
		)

		_, err := svc.Save(context.Background(), "proj-1", minimalValidEntries(), "bob")
		require.Error(t, err)
		assert.ErrorIs(t, err, upsertErr)
	})

	t.Run("saved snapshot has the correct UpdatedBy value", func(t *testing.T) {
		svc := integrations.NewJiraFieldMappingService(
			&fakeFieldMappingRepo{},
			&fakeConnectionRepo{connection: activeConnection(t)},
		)

		snap, err := svc.Save(context.Background(), "proj-1", minimalValidEntries(), "carol")
		require.NoError(t, err)
		require.NotNil(t, snap)

		assert.Equal(t, "carol", snap.UpdatedBy)
	})
}

// ---------------------------------------------------------------------------
// TestJiraFieldMappingService_Reset
// ---------------------------------------------------------------------------

func TestJiraFieldMappingService_Reset(t *testing.T) {
	t.Run("deletes saved mapping and returns default snapshot", func(t *testing.T) {
		repo := &fakeFieldMappingRepo{}
		svc := integrations.NewJiraFieldMappingService(repo, &fakeConnectionRepo{})

		snap, err := svc.Reset(context.Background(), "proj-1")
		require.NoError(t, err)
		require.NotNil(t, snap)

		assert.Equal(t, "proj-1", snap.ProjectID)
		assert.Equal(t, "proj-1", repo.deleted, "Delete should have been called for the project")

		byFernField := indexByFernField(snap.Entries)
		assert.Equal(t, "issuekey", byFernField[integrations.FernFieldRequirementID].JiraFieldID)
		assert.Equal(t, "summary", byFernField[integrations.FernFieldRequirementTitle].JiraFieldID)
	})

	t.Run("propagates repo Delete error", func(t *testing.T) {
		deleteErr := errors.New("database unavailable")
		svc := integrations.NewJiraFieldMappingService(
			&fakeFieldMappingRepo{err: deleteErr},
			&fakeConnectionRepo{},
		)

		_, err := svc.Reset(context.Background(), "proj-1")
		require.Error(t, err)
		assert.ErrorIs(t, err, deleteErr)
	})
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// indexByFernField converts a slice of FieldMappingEntry into a map keyed by FernField.
func indexByFernField(entries []integrations.FieldMappingEntry) map[integrations.FernField]integrations.FieldMappingEntry {
	m := make(map[integrations.FernField]integrations.FieldMappingEntry, len(entries))
	for _, e := range entries {
		m[e.FernField] = e
	}
	return m
}
