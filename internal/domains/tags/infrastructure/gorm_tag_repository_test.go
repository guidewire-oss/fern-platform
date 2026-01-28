package infrastructure_test

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/guidewire-oss/fern-platform/internal/domains/tags/domain"
	"github.com/guidewire-oss/fern-platform/internal/domains/tags/infrastructure"
	"github.com/guidewire-oss/fern-platform/pkg/database"
)

func TestGormTagRepository(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Tags Infrastructure Suite")
}

var _ = Describe("GormTagRepository", Label("integration", "infrastructure", "tags"), func() {
	var (
		db   *gorm.DB
		repo *infrastructure.GormTagRepository
		ctx  context.Context
	)

	BeforeEach(func() {
		var err error
		// Create in-memory SQLite database for testing
		db, err = gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
		Expect(err).NotTo(HaveOccurred())

		// Auto-migrate the schema
		err = db.AutoMigrate(&database.Tag{}, &database.TestRunTag{})
		Expect(err).NotTo(HaveOccurred())

		repo = infrastructure.NewGormTagRepository(db)
		ctx = context.Background()
	})

	AfterEach(func() {
		sqlDB, err := db.DB()
		if err == nil {
			sqlDB.Close()
		}
	})

	Describe("NewGormTagRepository", func() {
		It("should create a new repository", func() {
			newRepo := infrastructure.NewGormTagRepository(db)
			Expect(newRepo).NotTo(BeNil())
		})
	})

	Describe("Save", func() {
		It("should save a tag successfully", func() {
			tag, err := domain.NewTag("priority:high")
			Expect(err).NotTo(HaveOccurred())

			err = repo.Save(ctx, tag)
			Expect(err).NotTo(HaveOccurred())

			// Verify it was saved
			var dbTag database.Tag
			err = db.Where("name = ?", "priority:high").First(&dbTag).Error
			Expect(err).NotTo(HaveOccurred())
			Expect(dbTag.Name).To(Equal("priority:high"))
			Expect(dbTag.Category).To(Equal("priority"))
			Expect(dbTag.Value).To(Equal("high"))
		})

		It("should save a tag without category", func() {
			tag, err := domain.NewTag("important")
			Expect(err).NotTo(HaveOccurred())

			err = repo.Save(ctx, tag)
			Expect(err).NotTo(HaveOccurred())

			// Verify it was saved
			var dbTag database.Tag
			err = db.Where("name = ?", "important").First(&dbTag).Error
			Expect(err).NotTo(HaveOccurred())
			Expect(dbTag.Name).To(Equal("important"))
			Expect(dbTag.Category).To(Equal(""))
			Expect(dbTag.Value).To(Equal("important"))
		})

		It("should return error when database operation fails", func() {
			tag, err := domain.NewTag("test:tag")
			Expect(err).NotTo(HaveOccurred())

			// Close the database connection to simulate error
			sqlDB, _ := db.DB()
			sqlDB.Close()

			err = repo.Save(ctx, tag)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to save tag"))
		})
	})

	Describe("FindByID", func() {
		It("should find a tag by ID", func() {
			// Create a tag first
			dbTag := &database.Tag{
				Name:     "status:active",
				Category: "status",
				Value:    "active",
			}
			err := db.Create(dbTag).Error
			Expect(err).NotTo(HaveOccurred())

			// Find by ID
			tag, err := repo.FindByID(ctx, domain.TagID("1"))
			Expect(err).NotTo(HaveOccurred())
			Expect(tag).NotTo(BeNil())
			Expect(tag.Name()).To(Equal("status:active"))
			Expect(tag.Category()).To(Equal("status"))
			Expect(tag.Value()).To(Equal("active"))
		})

		It("should return error for non-existent ID", func() {
			tag, err := repo.FindByID(ctx, domain.TagID("999"))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("tag not found"))
			Expect(tag).To(BeNil())
		})

		It("should return error for invalid ID format", func() {
			tag, err := repo.FindByID(ctx, domain.TagID("invalid"))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid tag ID format"))
			Expect(tag).To(BeNil())
		})

		It("should return error when database query fails", func() {
			// Close database to simulate error
			sqlDB, _ := db.DB()
			sqlDB.Close()

			tag, err := repo.FindByID(ctx, domain.TagID("1"))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to find tag"))
			Expect(tag).To(BeNil())
		})
	})

	Describe("FindByName", func() {
		It("should find a tag by name", func() {
			// Create a tag first
			dbTag := &database.Tag{
				Name:     "environment:production",
				Category: "environment",
				Value:    "production",
			}
			err := db.Create(dbTag).Error
			Expect(err).NotTo(HaveOccurred())

			// Find by name
			tag, err := repo.FindByName(ctx, "environment:production")
			Expect(err).NotTo(HaveOccurred())
			Expect(tag).NotTo(BeNil())
			Expect(tag.Name()).To(Equal("environment:production"))
			Expect(tag.Category()).To(Equal("environment"))
			Expect(tag.Value()).To(Equal("production"))
		})

		It("should find tag with case-insensitive search", func() {
			// Create a tag first
			dbTag := &database.Tag{
				Name:     "priority:high",
				Category: "priority",
				Value:    "high",
			}
			err := db.Create(dbTag).Error
			Expect(err).NotTo(HaveOccurred())

			// Find with different case
			tag, err := repo.FindByName(ctx, "PRIORITY:HIGH")
			Expect(err).NotTo(HaveOccurred())
			Expect(tag).NotTo(BeNil())
			Expect(tag.Name()).To(Equal("priority:high"))
		})

		It("should trim whitespace in search", func() {
			// Create a tag first
			dbTag := &database.Tag{
				Name:     "test",
				Category: "",
				Value:    "test",
			}
			err := db.Create(dbTag).Error
			Expect(err).NotTo(HaveOccurred())

			// Find with whitespace
			tag, err := repo.FindByName(ctx, "  test  ")
			Expect(err).NotTo(HaveOccurred())
			Expect(tag).NotTo(BeNil())
			Expect(tag.Name()).To(Equal("test"))
		})

		It("should return error for non-existent tag", func() {
			tag, err := repo.FindByName(ctx, "nonexistent")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("tag not found"))
			Expect(tag).To(BeNil())
		})

		It("should return error when database query fails", func() {
			// Close database to simulate error
			sqlDB, _ := db.DB()
			sqlDB.Close()

			tag, err := repo.FindByName(ctx, "test")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to find tag"))
			Expect(tag).To(BeNil())
		})
	})

	Describe("FindAll", func() {
		It("should return all tags ordered by name", func() {
			// Create multiple tags
			tags := []database.Tag{
				{Name: "zebra", Category: "", Value: "zebra"},
				{Name: "alpha", Category: "", Value: "alpha"},
				{Name: "beta", Category: "", Value: "beta"},
			}
			for _, tag := range tags {
				err := db.Create(&tag).Error
				Expect(err).NotTo(HaveOccurred())
			}

			// Find all
			result, err := repo.FindAll(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(HaveLen(3))
			// Verify ordering
			Expect(result[0].Name()).To(Equal("alpha"))
			Expect(result[1].Name()).To(Equal("beta"))
			Expect(result[2].Name()).To(Equal("zebra"))
		})

		It("should return empty slice when no tags exist", func() {
			result, err := repo.FindAll(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(BeEmpty())
		})

		It("should return error when database query fails", func() {
			// Close database to simulate error
			sqlDB, _ := db.DB()
			sqlDB.Close()

			result, err := repo.FindAll(ctx)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to find tags"))
			Expect(result).To(BeNil())
		})
	})

	Describe("Delete", func() {
		It("should delete a tag successfully", func() {
			// Create a tag
			dbTag := &database.Tag{
				Name:     "temp:tag",
				Category: "temp",
				Value:    "tag",
			}
			err := db.Create(dbTag).Error
			Expect(err).NotTo(HaveOccurred())

			// Delete it
			err = repo.Delete(ctx, domain.TagID("1"))
			Expect(err).NotTo(HaveOccurred())

			// Verify it's gone
			var count int64
			db.Model(&database.Tag{}).Where("id = ?", 1).Count(&count)
			Expect(count).To(Equal(int64(0)))
		})

		It("should delete tag associations when deleting tag", func() {
			// Create a tag
			dbTag := &database.Tag{
				Name:     "test:tag",
				Category: "test",
				Value:    "tag",
			}
			err := db.Create(dbTag).Error
			Expect(err).NotTo(HaveOccurred())

			// Create a tag association
			association := &database.TestRunTag{
				TestRunID: 1,
				TagID:     1,
			}
			err = db.Create(association).Error
			Expect(err).NotTo(HaveOccurred())

			// Delete the tag
			err = repo.Delete(ctx, domain.TagID("1"))
			Expect(err).NotTo(HaveOccurred())

			// Verify association is also deleted
			var count int64
			db.Model(&database.TestRunTag{}).Where("tag_id = ?", 1).Count(&count)
			Expect(count).To(Equal(int64(0)))
		})

		It("should return error for invalid ID format", func() {
			err := repo.Delete(ctx, domain.TagID("invalid"))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid tag ID format"))
		})

		It("should handle deletion with no associations gracefully", func() {
			// Create a tag with no associations
			dbTag := &database.Tag{
				Name:     "solo:tag",
				Category: "solo",
				Value:    "tag",
			}
			err := db.Create(dbTag).Error
			Expect(err).NotTo(HaveOccurred())

			// Delete should work even with no associations
			err = repo.Delete(ctx, domain.TagID("1"))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return error when transaction fails", func() {
			// Create a tag
			dbTag := &database.Tag{
				Name:     "fail:tag",
				Category: "fail",
				Value:    "tag",
			}
			err := db.Create(dbTag).Error
			Expect(err).NotTo(HaveOccurred())

			// Close database to simulate error
			sqlDB, _ := db.DB()
			sqlDB.Close()

			err = repo.Delete(ctx, domain.TagID("1"))
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("AssignToTestRun", func() {
		It("should assign tags to a test run", func() {
			// Create tags
			tag1 := &database.Tag{Name: "tag1", Category: "", Value: "tag1"}
			tag2 := &database.Tag{Name: "tag2", Category: "", Value: "tag2"}
			db.Create(tag1)
			db.Create(tag2)

			// Assign to test run
			tagIDs := []domain.TagID{domain.TagID("1"), domain.TagID("2")}
			err := repo.AssignToTestRun(ctx, "100", tagIDs)
			Expect(err).NotTo(HaveOccurred())

			// Verify associations
			var associations []database.TestRunTag
			db.Where("test_run_id = ?", 100).Find(&associations)
			Expect(associations).To(HaveLen(2))
		})

		It("should replace existing tag associations", func() {
			// Create tags
			tag1 := &database.Tag{Name: "tag1", Category: "", Value: "tag1"}
			tag2 := &database.Tag{Name: "tag2", Category: "", Value: "tag2"}
			tag3 := &database.Tag{Name: "tag3", Category: "", Value: "tag3"}
			db.Create(tag1)
			db.Create(tag2)
			db.Create(tag3)

			// First assignment
			tagIDs1 := []domain.TagID{domain.TagID("1"), domain.TagID("2")}
			err := repo.AssignToTestRun(ctx, "100", tagIDs1)
			Expect(err).NotTo(HaveOccurred())

			// Second assignment (should replace)
			tagIDs2 := []domain.TagID{domain.TagID("3")}
			err = repo.AssignToTestRun(ctx, "100", tagIDs2)
			Expect(err).NotTo(HaveOccurred())

			// Verify only tag3 is associated
			var associations []database.TestRunTag
			db.Where("test_run_id = ?", 100).Find(&associations)
			Expect(associations).To(HaveLen(1))
			Expect(associations[0].TagID).To(Equal(uint(3)))
		})

		It("should return error for invalid test run ID format", func() {
			err := repo.AssignToTestRun(ctx, "invalid", []domain.TagID{})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid test run ID format"))
		})

		It("should return error for invalid tag ID format", func() {
			tagIDs := []domain.TagID{domain.TagID("invalid")}
			err := repo.AssignToTestRun(ctx, "100", tagIDs)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid tag ID format"))
		})

		It("should handle empty tag list", func() {
			// Assign empty list should work (removes all associations)
			err := repo.AssignToTestRun(ctx, "100", []domain.TagID{})
			Expect(err).NotTo(HaveOccurred())

			// Verify no associations
			var associations []database.TestRunTag
			db.Where("test_run_id = ?", 100).Find(&associations)
			Expect(associations).To(BeEmpty())
		})

		It("should return error when association creation fails", func() {
			// Create a tag
			tag := &database.Tag{Name: "tag1", Category: "", Value: "tag1"}
			db.Create(tag)

			// Close database to simulate error during association creation
			sqlDB, _ := db.DB()
			sqlDB.Close()

			tagIDs := []domain.TagID{domain.TagID("1")}
			err := repo.AssignToTestRun(ctx, "100", tagIDs)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("GetOrCreateTag", func() {
		It("should return existing tag if found", func() {
			// Create a tag first
			dbTag := &database.Tag{
				Name:     "existing:tag",
				Category: "existing",
				Value:    "tag",
			}
			err := db.Create(dbTag).Error
			Expect(err).NotTo(HaveOccurred())

			// Get or create should return existing
			tag, err := repo.GetOrCreateTag(ctx, "existing:tag")
			Expect(err).NotTo(HaveOccurred())
			Expect(tag).NotTo(BeNil())
			Expect(tag.Name()).To(Equal("existing:tag"))

			// Verify no duplicate was created
			var count int64
			db.Model(&database.Tag{}).Where("name = ?", "existing:tag").Count(&count)
			Expect(count).To(Equal(int64(1)))
		})

		It("should create new tag if not found", func() {
			// Get or create non-existent tag
			tag, err := repo.GetOrCreateTag(ctx, "new:tag")
			Expect(err).NotTo(HaveOccurred())
			Expect(tag).NotTo(BeNil())
			Expect(tag.Name()).To(Equal("new:tag"))
			Expect(tag.Category()).To(Equal("new"))
			Expect(tag.Value()).To(Equal("tag"))

			// Verify it was created
			var dbTag database.Tag
			err = db.Where("name = ?", "new:tag").First(&dbTag).Error
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return error for invalid tag name", func() {
			tag, err := repo.GetOrCreateTag(ctx, "")
			Expect(err).To(HaveOccurred())
			Expect(tag).To(BeNil())
		})

		It("should handle case-insensitive matching", func() {
			// Create a tag
			dbTag := &database.Tag{
				Name:     "test:value",
				Category: "test",
				Value:    "value",
			}
			err := db.Create(dbTag).Error
			Expect(err).NotTo(HaveOccurred())

			// Get with different case
			tag, err := repo.GetOrCreateTag(ctx, "TEST:VALUE")
			Expect(err).NotTo(HaveOccurred())
			Expect(tag).NotTo(BeNil())
			Expect(tag.Name()).To(Equal("test:value"))

			// Verify no duplicate
			var count int64
			db.Model(&database.Tag{}).Count(&count)
			Expect(count).To(Equal(int64(1)))
		})

		It("should handle save error when creating new tag", func() {
			// Close database to simulate error
			sqlDB, _ := db.DB()
			sqlDB.Close()

			tag, err := repo.GetOrCreateTag(ctx, "fail:tag")
			Expect(err).To(HaveOccurred())
			Expect(tag).To(BeNil())
		})

		It("should handle whitespace in tag names", func() {
			// Create tag with whitespace (should be normalized)
			tag, err := repo.GetOrCreateTag(ctx, "  spaced:tag  ")
			Expect(err).NotTo(HaveOccurred())
			Expect(tag).NotTo(BeNil())
			Expect(tag.Name()).To(Equal("spaced:tag"))

			// Try to get with different whitespace - should find same tag
			tag2, err := repo.GetOrCreateTag(ctx, "spaced:tag")
			Expect(err).NotTo(HaveOccurred())
			Expect(tag2).NotTo(BeNil())
			Expect(tag2.Name()).To(Equal("spaced:tag"))

			// Verify only one was created
			var count int64
			db.Model(&database.Tag{}).Where("name = ?", "spaced:tag").Count(&count)
			Expect(count).To(Equal(int64(1)))
		})
	})

	Describe("toDomainModel", func() {
		It("should convert database model to domain model correctly", func() {
			// Create and save a tag to test the full cycle
			domainTag, err := domain.NewTag("convert:test")
			Expect(err).NotTo(HaveOccurred())

			err = repo.Save(ctx, domainTag)
			Expect(err).NotTo(HaveOccurred())

			// Find it back
			retrievedTag, err := repo.FindByName(ctx, "convert:test")
			Expect(err).NotTo(HaveOccurred())
			Expect(retrievedTag).NotTo(BeNil())
			Expect(retrievedTag.ID()).NotTo(Equal(domain.TagID("")))
			Expect(retrievedTag.Name()).To(Equal("convert:test"))
			Expect(retrievedTag.Category()).To(Equal("convert"))
			Expect(retrievedTag.Value()).To(Equal("test"))
		})
	})
})
