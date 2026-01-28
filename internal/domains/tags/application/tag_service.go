package application

import (
	"context"
	"fmt"
	"strings"

	"github.com/guidewire-oss/fern-platform/internal/domains/tags/domain"
)

// TagService handles tag business logic
type TagService struct {
	tagRepo domain.TagRepository
}

//go:generate mockgen -destination=../mocks/mock_tag_service.go -package=mocks github.com/guidewire-oss/fern-platform/internal/domains/tags/application TagServiceInterface

type TagServiceInterface interface {
	CreateTag(ctx context.Context, name string) (*domain.Tag, error)
	GetTag(ctx context.Context, id domain.TagID) (*domain.Tag, error)
	GetTagByName(ctx context.Context, name string) (*domain.Tag, error)
	ListTags(ctx context.Context) ([]*domain.Tag, error)
	DeleteTag(ctx context.Context, id domain.TagID) error
	AssignTagsToTestRun(ctx context.Context, testRunID string, tagIDs []domain.TagID) error
	CreateMultipleTags(ctx context.Context, tagNames []string) ([]*domain.Tag, error)
	GetOrCreateTag(ctx context.Context, name string) (*domain.Tag, error)
}

// NewTagService creates a new tag service
func NewTagService(tagRepo domain.TagRepository) *TagService {
	return &TagService{
		tagRepo: tagRepo,
	}
}

// CreateTag creates a new tag
func (s *TagService) CreateTag(ctx context.Context, name string) (*domain.Tag, error) {
	// Check if tag already exists
	existing, err := s.tagRepo.FindByName(ctx, name)
	if err == nil && existing != nil {
		return existing, nil // Return existing tag
	}

	// Create new tag
	tag, err := domain.NewTag(name)
	if err != nil {
		return nil, fmt.Errorf("failed to create tag: %w", err)
	}

	// Save the tag
	if err := s.tagRepo.Save(ctx, tag); err != nil {
		return nil, fmt.Errorf("failed to save tag: %w", err)
	}

	// Retrieve the saved tag to get the ID
	return s.tagRepo.FindByName(ctx, name)
}

// GetTag retrieves a tag by ID
func (s *TagService) GetTag(ctx context.Context, id domain.TagID) (*domain.Tag, error) {
	tag, err := s.tagRepo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get tag: %w", err)
	}
	return tag, nil
}

// GetTagByName retrieves a tag by name
func (s *TagService) GetTagByName(ctx context.Context, name string) (*domain.Tag, error) {
	tag, err := s.tagRepo.FindByName(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("failed to get tag by name: %w", err)
	}
	return tag, nil
}

// ListTags retrieves all tags
func (s *TagService) ListTags(ctx context.Context) ([]*domain.Tag, error) {
	tags, err := s.tagRepo.FindAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list tags: %w", err)
	}
	return tags, nil
}

// DeleteTag deletes a tag
func (s *TagService) DeleteTag(ctx context.Context, id domain.TagID) error {
	// Check if tag exists
	_, err := s.tagRepo.FindByID(ctx, id)
	if err != nil {
		return fmt.Errorf("tag not found: %w", err)
	}

	// Delete the tag
	if err := s.tagRepo.Delete(ctx, id); err != nil {
		return fmt.Errorf("failed to delete tag: %w", err)
	}

	return nil
}

// AssignTagsToTestRun assigns multiple tags to a test run
func (s *TagService) AssignTagsToTestRun(ctx context.Context, testRunID string, tagIDs []domain.TagID) error {
	// Verify all tags exist
	for _, tagID := range tagIDs {
		if _, err := s.tagRepo.FindByID(ctx, tagID); err != nil {
			return fmt.Errorf("tag %s not found: %w", tagID, err)
		}
	}

	// Assign tags to test run
	if err := s.tagRepo.AssignToTestRun(ctx, testRunID, tagIDs); err != nil {
		return fmt.Errorf("failed to assign tags to test run: %w", err)
	}

	return nil
}

// CreateMultipleTags creates multiple tags from a list of names
func (s *TagService) CreateMultipleTags(ctx context.Context, tagNames []string) ([]*domain.Tag, error) {
	tags := make([]*domain.Tag, 0, len(tagNames))

	for _, name := range tagNames {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}

		tag, err := s.CreateTag(ctx, name)
		if err != nil {
			return nil, fmt.Errorf("failed to create tag '%s': %w", name, err)
		}
		tags = append(tags, tag)
	}

	return tags, nil
}

// GetOrCreateTag gets an existing tag or creates a new one
func (s *TagService) GetOrCreateTag(ctx context.Context, name string) (*domain.Tag, error) {
	// Try to get existing tag
	tag, err := s.tagRepo.FindByName(ctx, name)
	if err == nil {
		return tag, nil
	}

	// Create new tag if not found
	return s.CreateTag(ctx, name)
}
