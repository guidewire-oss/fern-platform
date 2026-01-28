package domain_test

import (
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/guidewire-oss/fern-platform/internal/domains/tags/domain"
)

func TestTagDomain(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Tags Domain Suite")
}

var _ = Describe("Tag", Label("unit", "domain", "tags"), func() {
	Describe("NewTag", func() {
		Context("with valid tag names", func() {
			It("should create a tag with category and value", func() {
				tag, err := domain.NewTag("priority:high")
				Expect(err).NotTo(HaveOccurred())
				Expect(tag).NotTo(BeNil())
				Expect(tag.Name()).To(Equal("priority:high"))
				Expect(tag.Category()).To(Equal("priority"))
				Expect(tag.Value()).To(Equal("high"))
				Expect(tag.ID()).To(Equal(domain.TagID("")))
				Expect(tag.CreatedAt()).To(BeTemporally("~", time.Now(), time.Second))
			})

			It("should create a tag with only value (no category)", func() {
				tag, err := domain.NewTag("important")
				Expect(err).NotTo(HaveOccurred())
				Expect(tag).NotTo(BeNil())
				Expect(tag.Name()).To(Equal("important"))
				Expect(tag.Category()).To(Equal(""))
				Expect(tag.Value()).To(Equal("important"))
			})

			It("should normalize tag name to lowercase", func() {
				tag, err := domain.NewTag("Priority:HIGH")
				Expect(err).NotTo(HaveOccurred())
				Expect(tag.Name()).To(Equal("priority:high"))
				Expect(tag.Category()).To(Equal("priority"))
				Expect(tag.Value()).To(Equal("high"))
			})

			It("should trim whitespace from tag name", func() {
				tag, err := domain.NewTag("  priority:high  ")
				Expect(err).NotTo(HaveOccurred())
				Expect(tag.Name()).To(Equal("priority:high"))
				Expect(tag.Category()).To(Equal("priority"))
				Expect(tag.Value()).To(Equal("high"))
			})

			It("should trim whitespace around colon", func() {
				tag, err := domain.NewTag("priority : high")
				Expect(err).NotTo(HaveOccurred())
				Expect(tag.Name()).To(Equal("priority : high"))
				Expect(tag.Category()).To(Equal("priority"))
				Expect(tag.Value()).To(Equal("high"))
			})

			It("should handle tag with multiple colons (uses first colon)", func() {
				tag, err := domain.NewTag("type:bug:critical")
				Expect(err).NotTo(HaveOccurred())
				Expect(tag.Name()).To(Equal("type:bug:critical"))
				Expect(tag.Category()).To(Equal("type"))
				Expect(tag.Value()).To(Equal("bug:critical"))
			})

			It("should handle tag with colon at the end", func() {
				tag, err := domain.NewTag("category:")
				Expect(err).NotTo(HaveOccurred())
				Expect(tag.Name()).To(Equal("category:"))
				Expect(tag.Category()).To(Equal("category"))
				Expect(tag.Value()).To(Equal(""))
			})

			It("should handle tag with colon at the beginning", func() {
				tag, err := domain.NewTag(":value")
				Expect(err).NotTo(HaveOccurred())
				Expect(tag.Name()).To(Equal(":value"))
				Expect(tag.Category()).To(Equal(""))
				Expect(tag.Value()).To(Equal("value"))
			})
		})

		Context("with invalid tag names", func() {
			It("should return error when name is empty", func() {
				tag, err := domain.NewTag("")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("tag name cannot be empty"))
				Expect(tag).To(BeNil())
			})

			It("should return error when name is only whitespace", func() {
				tag, err := domain.NewTag("   ")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("tag name cannot be empty after normalization"))
				Expect(tag).To(BeNil())
			})

			It("should return error when name is tabs and spaces", func() {
				tag, err := domain.NewTag("\t\n  \t")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("tag name cannot be empty after normalization"))
				Expect(tag).To(BeNil())
			})
		})
	})

	Describe("Tag Getters", func() {
		var tag *domain.Tag

		BeforeEach(func() {
			var err error
			tag, err = domain.NewTag("environment:production")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return the tag ID", func() {
			id := tag.ID()
			Expect(id).To(Equal(domain.TagID("")))
		})

		It("should return the tag name", func() {
			name := tag.Name()
			Expect(name).To(Equal("environment:production"))
		})

		It("should return the tag category", func() {
			category := tag.Category()
			Expect(category).To(Equal("environment"))
		})

		It("should return the tag value", func() {
			value := tag.Value()
			Expect(value).To(Equal("production"))
		})

		It("should return the creation time", func() {
			createdAt := tag.CreatedAt()
			Expect(createdAt).To(BeTemporally("~", time.Now(), time.Second))
		})
	})

	Describe("ToSnapshot", func() {
		It("should create a snapshot with all tag fields", func() {
			tag, err := domain.NewTag("severity:critical")
			Expect(err).NotTo(HaveOccurred())

			snapshot := tag.ToSnapshot()
			Expect(snapshot.ID).To(Equal(domain.TagID("")))
			Expect(snapshot.Name).To(Equal("severity:critical"))
			Expect(snapshot.Category).To(Equal("severity"))
			Expect(snapshot.Value).To(Equal("critical"))
			Expect(snapshot.CreatedAt).To(BeTemporally("~", time.Now(), time.Second))
		})

		It("should create a snapshot for tag without category", func() {
			tag, err := domain.NewTag("urgent")
			Expect(err).NotTo(HaveOccurred())

			snapshot := tag.ToSnapshot()
			Expect(snapshot.Name).To(Equal("urgent"))
			Expect(snapshot.Category).To(Equal(""))
			Expect(snapshot.Value).To(Equal("urgent"))
		})
	})

	Describe("ReconstructTag", func() {
		It("should reconstruct a tag from persistence data", func() {
			id := domain.TagID("tag-123")
			name := "status:active"
			category := "status"
			value := "active"
			createdAt := time.Now().Add(-24 * time.Hour)

			tag := domain.ReconstructTag(id, name, category, value, createdAt)

			Expect(tag).NotTo(BeNil())
			Expect(tag.ID()).To(Equal(id))
			Expect(tag.Name()).To(Equal(name))
			Expect(tag.Category()).To(Equal(category))
			Expect(tag.Value()).To(Equal(value))
			Expect(tag.CreatedAt()).To(Equal(createdAt))
		})

		It("should reconstruct a tag without category", func() {
			id := domain.TagID("tag-456")
			name := "important"
			category := ""
			value := "important"
			createdAt := time.Now().Add(-48 * time.Hour)

			tag := domain.ReconstructTag(id, name, category, value, createdAt)

			Expect(tag).NotTo(BeNil())
			Expect(tag.ID()).To(Equal(id))
			Expect(tag.Name()).To(Equal(name))
			Expect(tag.Category()).To(Equal(""))
			Expect(tag.Value()).To(Equal(value))
			Expect(tag.CreatedAt()).To(Equal(createdAt))
		})

		It("should reconstruct a tag with empty ID", func() {
			id := domain.TagID("")
			name := "test"
			category := ""
			value := "test"
			createdAt := time.Now()

			tag := domain.ReconstructTag(id, name, category, value, createdAt)

			Expect(tag).NotTo(BeNil())
			Expect(tag.ID()).To(Equal(domain.TagID("")))
		})
	})

	Describe("Edge Cases", func() {
		It("should handle special characters in tag name", func() {
			tag, err := domain.NewTag("type:bug-fix")
			Expect(err).NotTo(HaveOccurred())
			Expect(tag.Name()).To(Equal("type:bug-fix"))
			Expect(tag.Category()).To(Equal("type"))
			Expect(tag.Value()).To(Equal("bug-fix"))
		})

		It("should handle numbers in tag name", func() {
			tag, err := domain.NewTag("version:1.2.3")
			Expect(err).NotTo(HaveOccurred())
			Expect(tag.Name()).To(Equal("version:1.2.3"))
			Expect(tag.Category()).To(Equal("version"))
			Expect(tag.Value()).To(Equal("1.2.3"))
		})

		It("should handle underscores in tag name", func() {
			tag, err := domain.NewTag("test_type:unit_test")
			Expect(err).NotTo(HaveOccurred())
			Expect(tag.Name()).To(Equal("test_type:unit_test"))
			Expect(tag.Category()).To(Equal("test_type"))
			Expect(tag.Value()).To(Equal("unit_test"))
		})

		It("should handle Unicode characters in tag name", func() {
			tag, err := domain.NewTag("language:日本語")
			Expect(err).NotTo(HaveOccurred())
			Expect(tag.Name()).To(Equal("language:日本語"))
			Expect(tag.Category()).To(Equal("language"))
			Expect(tag.Value()).To(Equal("日本語"))
		})
	})
})
