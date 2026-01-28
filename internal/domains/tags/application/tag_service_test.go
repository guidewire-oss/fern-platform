package application_test

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	app "github.com/guidewire-oss/fern-platform/internal/domains/tags/application"
	domain "github.com/guidewire-oss/fern-platform/internal/domains/tags/domain"
)

func TestTagService(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "TagService Suite")
}

// --- In-memory repository mock ---
type inMemoryTagRepo struct {
	mu      sync.Mutex
	nextID  int
	byID    map[domain.TagID]*domain.Tag
	byName  map[string]*domain.Tag
	assigns map[string][]domain.TagID
	fail    bool
}

func newInMemoryTagRepo() *inMemoryTagRepo {
	return &inMemoryTagRepo{
		nextID:  1,
		byID:    make(map[domain.TagID]*domain.Tag),
		byName:  make(map[string]*domain.Tag),
		assigns: make(map[string][]domain.TagID),
	}
}

func (r *inMemoryTagRepo) FindByName(ctx context.Context, name string) (*domain.Tag, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.fail {
		return nil, fmt.Errorf("repo error")
	}
	if t, ok := r.byName[strings.ToLower(name)]; ok {
		return t, nil
	}
	return nil, fmt.Errorf("not found")
}

func (r *inMemoryTagRepo) FindByID(ctx context.Context, id domain.TagID) (*domain.Tag, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.fail {
		return nil, fmt.Errorf("repo error")
	}
	t, ok := r.byID[id]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	return t, nil
}

func (r *inMemoryTagRepo) Save(ctx context.Context, t *domain.Tag) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	id := t.ID()
	if id == "" {
		id = domain.TagID(strconv.Itoa(r.nextID))
		r.nextID++
		t = domain.ReconstructTag(id, t.Name(), t.Category(), t.Value(), t.CreatedAt())
	}
	r.byID[id] = t
	r.byName[strings.ToLower(t.Name())] = t
	return nil
}

func (r *inMemoryTagRepo) FindAll(ctx context.Context) ([]*domain.Tag, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.fail {
		return nil, fmt.Errorf("repo error")
	}
	var out []*domain.Tag
	for _, t := range r.byID {
		out = append(out, t)
	}
	return out, nil
}

func (r *inMemoryTagRepo) Delete(ctx context.Context, id domain.TagID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.byID[id]
	if !ok {
		return fmt.Errorf("not found")
	}
	delete(r.byID, id)
	delete(r.byName, strings.ToLower(t.Name()))
	return nil
}

func (r *inMemoryTagRepo) AssignToTestRun(ctx context.Context, testRunID string, tagIDs []domain.TagID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.fail {
		return fmt.Errorf("repo error")
	}
	for _, id := range tagIDs {
		if _, ok := r.byID[id]; !ok {
			return fmt.Errorf("tag %s not found", id)
		}
	}
	r.assigns[testRunID] = append(r.assigns[testRunID], tagIDs...)
	return nil
}

// erroringRepo is a repository that can fail on specific operations
type erroringRepo struct {
	shouldFailOn string
	callCount    int
}

func (r *erroringRepo) FindByName(ctx context.Context, name string) (*domain.Tag, error) {
	if r.shouldFailOn == "FindByNameAfterSave" {
		r.callCount++
		if r.callCount > 1 {
			return nil, fmt.Errorf("error after save")
		}
		return nil, fmt.Errorf("not found initially")
	}
	if r.shouldFailOn == "FindByName" {
		return nil, fmt.Errorf("find by name error")
	}
	return nil, fmt.Errorf("not found")
}

func (r *erroringRepo) FindByID(ctx context.Context, id domain.TagID) (*domain.Tag, error) {
	if r.shouldFailOn == "FindByID" {
		return nil, fmt.Errorf("find by id error")
	}
	return nil, fmt.Errorf("not found")
}

func (r *erroringRepo) Save(ctx context.Context, t *domain.Tag) error {
	if r.shouldFailOn == "Save" {
		return fmt.Errorf("save error")
	}
	return nil
}

func (r *erroringRepo) FindAll(ctx context.Context) ([]*domain.Tag, error) {
	if r.shouldFailOn == "FindAll" {
		return nil, fmt.Errorf("find all error")
	}
	return []*domain.Tag{}, nil
}

func (r *erroringRepo) Delete(ctx context.Context, id domain.TagID) error {
	if r.shouldFailOn == "Delete" {
		return fmt.Errorf("delete error")
	}
	return nil
}

func (r *erroringRepo) AssignToTestRun(ctx context.Context, testRunID string, tagIDs []domain.TagID) error {
	if r.shouldFailOn == "AssignToTestRun" {
		return fmt.Errorf("assign error")
	}
	return nil
}

// mockRepoForDelete allows FindByID to succeed but Delete to fail
type mockRepoForDelete struct {
	tag *domain.Tag
}

func (r *mockRepoForDelete) FindByID(ctx context.Context, id domain.TagID) (*domain.Tag, error) {
	return r.tag, nil
}

func (r *mockRepoForDelete) Delete(ctx context.Context, id domain.TagID) error {
	return fmt.Errorf("database error during delete")
}

func (r *mockRepoForDelete) FindByName(ctx context.Context, name string) (*domain.Tag, error) {
	return nil, fmt.Errorf("not implemented")
}

func (r *mockRepoForDelete) Save(ctx context.Context, t *domain.Tag) error {
	return fmt.Errorf("not implemented")
}

func (r *mockRepoForDelete) FindAll(ctx context.Context) ([]*domain.Tag, error) {
	return nil, fmt.Errorf("not implemented")
}

func (r *mockRepoForDelete) AssignToTestRun(ctx context.Context, testRunID string, tagIDs []domain.TagID) error {
	return fmt.Errorf("not implemented")
}

// mockRepoForAssign allows FindByID to succeed but AssignToTestRun to fail
type mockRepoForAssign struct {
	tag *domain.Tag
}

func (r *mockRepoForAssign) FindByID(ctx context.Context, id domain.TagID) (*domain.Tag, error) {
	return r.tag, nil
}

func (r *mockRepoForAssign) AssignToTestRun(ctx context.Context, testRunID string, tagIDs []domain.TagID) error {
	return fmt.Errorf("database error during assign")
}

func (r *mockRepoForAssign) FindByName(ctx context.Context, name string) (*domain.Tag, error) {
	return nil, fmt.Errorf("not implemented")
}

func (r *mockRepoForAssign) Save(ctx context.Context, t *domain.Tag) error {
	return fmt.Errorf("not implemented")
}

func (r *mockRepoForAssign) FindAll(ctx context.Context) ([]*domain.Tag, error) {
	return nil, fmt.Errorf("not implemented")
}

func (r *mockRepoForAssign) Delete(ctx context.Context, id domain.TagID) error {
	return fmt.Errorf("not implemented")
}

// --- Specs ---
var _ = Describe("TagService", func() {
	var (
		repo *inMemoryTagRepo
		svc  *app.TagService
		ctx  context.Context
	)

	BeforeEach(func() {
		ctx = context.Background()
		repo = newInMemoryTagRepo()
		svc = app.NewTagService(repo)
	})

	Describe("CreateTag", func() {
		It("creates a new tag if not exists", func() {
			tag, err := svc.CreateTag(ctx, "TestTag")
			Expect(err).To(BeNil())
			Expect(tag.Name()).To(Equal("testtag")) // normalized
			Expect(tag.ID()).NotTo(BeEmpty())
		})

		It("returns existing tag if already exists", func() {
			first, _ := svc.CreateTag(ctx, "dup")
			second, err := svc.CreateTag(ctx, "dup")
			Expect(err).To(BeNil())
			Expect(second.ID()).To(Equal(first.ID()))
		})

		It("returns error if repository fails", func() {
			repo.fail = true
			_, err := svc.CreateTag(ctx, "x")
			Expect(err).ToNot(BeNil())
		})

		It("returns error for invalid tag name", func() {
			_, err := svc.CreateTag(ctx, "")
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("failed to create tag"))
		})

		It("handles save errors", func() {
			repo2 := &erroringRepo{shouldFailOn: "Save"}
			svc2 := app.NewTagService(repo2)
			_, err := svc2.CreateTag(ctx, "test")
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("failed to save tag"))
		})

		It("handles FindByName error after save", func() {
			repo2 := &erroringRepo{shouldFailOn: "FindByNameAfterSave"}
			svc2 := app.NewTagService(repo2)
			_, err := svc2.CreateTag(ctx, "test")
			Expect(err).ToNot(BeNil())
		})
	})

	Describe("GetTag and GetTagByName", func() {
		It("retrieves tag by ID", func() {
			t, _ := svc.CreateTag(ctx, "byid")
			got, err := svc.GetTag(ctx, t.ID())
			Expect(err).To(BeNil())
			Expect(got.ID()).To(Equal(t.ID()))
		})

		It("retrieves tag by Name", func() {
			t, _ := svc.CreateTag(ctx, "byname")
			got, err := svc.GetTagByName(ctx, t.Name())
			Expect(err).To(BeNil())
			Expect(got.Name()).To(Equal(t.Name()))
		})

		It("returns error when not found", func() {
			_, err := svc.GetTag(ctx, "nonexistent")
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("failed to get tag"))
		})

		It("returns error when GetTagByName fails", func() {
			repo2 := &erroringRepo{shouldFailOn: "FindByName"}
			svc2 := app.NewTagService(repo2)
			_, err := svc2.GetTagByName(ctx, "test")
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("failed to get tag by name"))
		})

		It("returns error when GetTag fails", func() {
			repo2 := &erroringRepo{shouldFailOn: "FindByID"}
			svc2 := app.NewTagService(repo2)
			_, err := svc2.GetTag(ctx, "test-id")
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("failed to get tag"))
		})
	})

	Describe("ListTags", func() {
		It("lists all tags", func() {
			_, _ = svc.CreateTag(ctx, "a")
			_, _ = svc.CreateTag(ctx, "b")
			tags, err := svc.ListTags(ctx)
			Expect(err).To(BeNil())
			Expect(len(tags)).To(Equal(2))
		})

		It("returns error when repository fails", func() {
			repo2 := &erroringRepo{shouldFailOn: "FindAll"}
			svc2 := app.NewTagService(repo2)
			_, err := svc2.ListTags(ctx)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("failed to list tags"))
		})
	})

	Describe("DeleteTag", func() {
		It("deletes an existing tag", func() {
			t, _ := svc.CreateTag(ctx, "todel")
			err := svc.DeleteTag(ctx, t.ID())
			Expect(err).To(BeNil())
			_, err2 := svc.GetTag(ctx, t.ID())
			Expect(err2).ToNot(BeNil())
		})

		It("returns error if tag not found", func() {
			err := svc.DeleteTag(ctx, "missing")
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("tag not found"))
		})

		It("returns error when delete operation fails", func() {
			tag, _ := domain.NewTag("test")
			mockRepo := &mockRepoForDelete{tag: tag}
			svc2 := app.NewTagService(mockRepo)
			err := svc2.DeleteTag(ctx, "test-id")
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("failed to delete tag"))
		})
	})

	Describe("AssignTagsToTestRun", func() {
		It("assigns tags successfully", func() {
			t1, _ := svc.CreateTag(ctx, "one")
			t2, _ := svc.CreateTag(ctx, "two")
			err := svc.AssignTagsToTestRun(ctx, "run1", []domain.TagID{t1.ID(), t2.ID()})
			Expect(err).To(BeNil())
			Expect(repo.assigns["run1"]).To(ContainElements(t1.ID(), t2.ID()))
		})

		It("returns error if any tag not found", func() {
			err := svc.AssignTagsToTestRun(ctx, "run2", []domain.TagID{"missing"})
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("not found"))
		})

		It("returns error when assignment operation fails", func() {
			tag, _ := domain.NewTag("test")
			mockRepo := &mockRepoForAssign{tag: tag}
			svc2 := app.NewTagService(mockRepo)
			err := svc2.AssignTagsToTestRun(ctx, "run1", []domain.TagID{"test-id"})
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("failed to assign tags to test run"))
		})
	})

	Describe("CreateMultipleTags", func() {
		It("creates multiple tags", func() {
			names := []string{"x", "y", "z"}
			tags, err := svc.CreateMultipleTags(ctx, names)
			Expect(err).To(BeNil())
			Expect(len(tags)).To(Equal(3))
		})

		It("ignores empty names", func() {
			tags, err := svc.CreateMultipleTags(ctx, []string{"a", "", "b"})
			Expect(err).To(BeNil())
			Expect(len(tags)).To(Equal(2))
		})

		It("trims whitespace from names", func() {
			tags, err := svc.CreateMultipleTags(ctx, []string{"  spaced  ", "normal"})
			Expect(err).To(BeNil())
			Expect(len(tags)).To(Equal(2))
		})

		It("skips whitespace-only names", func() {
			tags, err := svc.CreateMultipleTags(ctx, []string{"a", "   ", "b"})
			Expect(err).To(BeNil())
			Expect(len(tags)).To(Equal(2))
		})

		It("returns error when any tag creation fails", func() {
			repo2 := &erroringRepo{shouldFailOn: "Save"}
			svc2 := app.NewTagService(repo2)
			_, err := svc2.CreateMultipleTags(ctx, []string{"test"})
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("failed to create tag"))
		})
	})

	Describe("GetOrCreateTag", func() {
		It("returns existing tag if found", func() {
			t, _ := svc.CreateTag(ctx, "exist")
			got, err := svc.GetOrCreateTag(ctx, "exist")
			Expect(err).To(BeNil())
			Expect(got.ID()).To(Equal(t.ID()))
		})

		It("creates tag if not found", func() {
			got, err := svc.GetOrCreateTag(ctx, "newtag")
			Expect(err).To(BeNil())
			Expect(got.Name()).To(Equal("newtag"))
		})
	})
})
