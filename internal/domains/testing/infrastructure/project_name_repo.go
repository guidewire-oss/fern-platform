package infrastructure

import (
	"context"

	"gorm.io/gorm"

	"github.com/guidewire-oss/fern-platform/pkg/database"
)

// ProjectNameRepo resolves project display names for the v2 read path.
//
// Test runs carry only a project_id; the human-readable name lives in
// project_details. Rather than joining it into every list query (and
// paying for it on the filter/facet/count paths too), the read side
// looks names up in one batched query after the page is fetched.
type ProjectNameRepo struct {
	db *gorm.DB
}

// NewProjectNameRepo builds a resolver on an existing GORM handle.
func NewProjectNameRepo(db *gorm.DB) *ProjectNameRepo {
	return &ProjectNameRepo{db: db}
}

// NamesByProjectID returns project_id → display name for the given IDs.
//
// The result only contains IDs that resolved to a non-empty name;
// unknown or unnamed projects are simply absent, letting callers fall
// back to displaying the raw ID. IDs are de-duplicated so the cost is
// one indexed IN (...) query per call, not one per run on the page.
func (r *ProjectNameRepo) NamesByProjectID(ctx context.Context, ids []string) (map[string]string, error) {
	unique := make([]string, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if id == "" {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
	}
	out := make(map[string]string, len(unique))
	if len(unique) == 0 {
		return out, nil
	}

	type row struct {
		ProjectID string
		Name      string
	}
	var rows []row
	// Scan, not Find: the explicit two-column SELECT does not match the
	// full ProjectDetails model, and Scan respects it exactly.
	err := r.db.WithContext(ctx).
		Model(&database.ProjectDetails{}).
		Select("project_id, name").
		Where("project_id IN ?", unique).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, x := range rows {
		if x.Name == "" {
			continue
		}
		out[x.ProjectID] = x.Name
	}
	return out, nil
}
