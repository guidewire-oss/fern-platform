package infrastructure_test

import (
	"context"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/guidewire-oss/fern-platform/internal/domains/projects/domain"
	"github.com/guidewire-oss/fern-platform/internal/domains/projects/infrastructure"
)

func TestProjectsInfrastructure(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Projects Infrastructure Suite")
}

var _ = Describe("GormProjectPermissionRepository", Label("unit", "infrastructure", "projects"), func() {
	var (
		db   *gorm.DB
		repo *infrastructure.GormProjectPermissionRepository
		ctx  context.Context
	)

	BeforeEach(func() {
		var err error
		db, err = gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		})
		Expect(err).NotTo(HaveOccurred())

		err = db.AutoMigrate(&infrastructure.ProjectPermissionDB{})
		Expect(err).NotTo(HaveOccurred())

		repo = infrastructure.NewGormProjectPermissionRepository(db)
		ctx = context.Background()
	})

	AfterEach(func() {
		sqlDB, err := db.DB()
		if err == nil {
			sqlDB.Close()
		}
	})

	Describe("NewGormProjectPermissionRepository", func() {
		It("returns a non-nil repository", func() {
			Expect(infrastructure.NewGormProjectPermissionRepository(db)).NotTo(BeNil())
		})
	})

	Describe("Save", func() {
		It("persists a new permission", func() {
			perm, err := domain.NewProjectPermission("proj-1", "user-1", domain.PermissionRead, "admin")
			Expect(err).NotTo(HaveOccurred())

			Expect(repo.Save(ctx, perm)).NotTo(HaveOccurred())

			var count int64
			db.Model(&infrastructure.ProjectPermissionDB{}).
				Where("project_id = ? AND user_id = ?", "proj-1", "user-1").
				Count(&count)
			Expect(count).To(Equal(int64(1)))
		})

		It("returns an error when saving a duplicate permission", func() {
			perm, err := domain.NewProjectPermission("proj-dup", "user-dup", domain.PermissionRead, "admin")
			Expect(err).NotTo(HaveOccurred())

			Expect(repo.Save(ctx, perm)).NotTo(HaveOccurred())
			Expect(repo.Save(ctx, perm)).To(HaveOccurred())
		})
	})

	Describe("FindByProjectAndUser", func() {
		BeforeEach(func() {
			for _, pt := range []domain.PermissionType{domain.PermissionRead, domain.PermissionWrite} {
				perm, err := domain.NewProjectPermission("proj-find", "user-find", pt, "admin")
				Expect(err).NotTo(HaveOccurred())
				Expect(repo.Save(ctx, perm)).NotTo(HaveOccurred())
			}
		})

		It("returns permissions for the matching project and user", func() {
			perms, err := repo.FindByProjectAndUser(ctx, "proj-find", "user-find")
			Expect(err).NotTo(HaveOccurred())
			Expect(perms).To(HaveLen(2))
		})

		It("returns an empty slice when no permissions exist", func() {
			perms, err := repo.FindByProjectAndUser(ctx, "proj-none", "user-none")
			Expect(err).NotTo(HaveOccurred())
			Expect(perms).To(BeEmpty())
		})

		It("does not return permissions for a different user", func() {
			perms, err := repo.FindByProjectAndUser(ctx, "proj-find", "user-other")
			Expect(err).NotTo(HaveOccurred())
			Expect(perms).To(BeEmpty())
		})
	})

	Describe("FindByUser", func() {
		BeforeEach(func() {
			for _, projID := range []string{"proj-a", "proj-b"} {
				perm, err := domain.NewProjectPermission(domain.ProjectID(projID), "user-all", domain.PermissionRead, "admin")
				Expect(err).NotTo(HaveOccurred())
				Expect(repo.Save(ctx, perm)).NotTo(HaveOccurred())
			}
		})

		It("returns all permissions for the user across projects", func() {
			perms, err := repo.FindByUser(ctx, "user-all")
			Expect(err).NotTo(HaveOccurred())
			Expect(perms).To(HaveLen(2))
		})

		It("returns an empty slice when the user has no permissions", func() {
			perms, err := repo.FindByUser(ctx, "user-nobody")
			Expect(err).NotTo(HaveOccurred())
			Expect(perms).To(BeEmpty())
		})
	})

	Describe("FindByProject", func() {
		BeforeEach(func() {
			for _, userID := range []string{"user-x", "user-y"} {
				perm, err := domain.NewProjectPermission("proj-multi", userID, domain.PermissionRead, "admin")
				Expect(err).NotTo(HaveOccurred())
				Expect(repo.Save(ctx, perm)).NotTo(HaveOccurred())
			}
		})

		It("returns all permissions for the project", func() {
			perms, err := repo.FindByProject(ctx, "proj-multi")
			Expect(err).NotTo(HaveOccurred())
			Expect(perms).To(HaveLen(2))
		})

		It("returns an empty slice when the project has no permissions", func() {
			perms, err := repo.FindByProject(ctx, "proj-empty")
			Expect(err).NotTo(HaveOccurred())
			Expect(perms).To(BeEmpty())
		})
	})

	Describe("Delete", func() {
		BeforeEach(func() {
			perm, err := domain.NewProjectPermission("proj-del", "user-del", domain.PermissionWrite, "admin")
			Expect(err).NotTo(HaveOccurred())
			Expect(repo.Save(ctx, perm)).NotTo(HaveOccurred())
		})

		It("removes the specified permission", func() {
			Expect(repo.Delete(ctx, "proj-del", "user-del", domain.PermissionWrite)).NotTo(HaveOccurred())

			perms, err := repo.FindByProjectAndUser(ctx, "proj-del", "user-del")
			Expect(err).NotTo(HaveOccurred())
			Expect(perms).To(BeEmpty())
		})

		It("does not error when deleting a non-existent permission", func() {
			Expect(repo.Delete(ctx, "proj-ghost", "user-ghost", domain.PermissionAdmin)).NotTo(HaveOccurred())
		})
	})

	Describe("DeleteExpired", func() {
		BeforeEach(func() {
			past := time.Now().Add(-time.Hour)
			future := time.Now().Add(time.Hour)

			// Permission with an already-expired expiry date (inserted directly)
			db.Create(&infrastructure.ProjectPermissionDB{
				ProjectID:  "proj-exp",
				UserID:     "user-exp",
				Permission: string(domain.PermissionRead),
				GrantedBy:  "admin",
				GrantedAt:  time.Now(),
				ExpiresAt:  &past,
			})

			// Permission that is still valid
			db.Create(&infrastructure.ProjectPermissionDB{
				ProjectID:  "proj-valid",
				UserID:     "user-valid",
				Permission: string(domain.PermissionRead),
				GrantedBy:  "admin",
				GrantedAt:  time.Now(),
				ExpiresAt:  &future,
			})

			// Permission with no expiry
			perm, _ := domain.NewProjectPermission("proj-noexp", "user-noexp", domain.PermissionRead, "admin")
			Expect(repo.Save(ctx, perm)).NotTo(HaveOccurred())
		})

		It("removes only expired permissions", func() {
			Expect(repo.DeleteExpired(ctx)).NotTo(HaveOccurred())

			var expiredCount, validCount, noExpCount int64
			db.Model(&infrastructure.ProjectPermissionDB{}).
				Where("project_id = ?", "proj-exp").Count(&expiredCount)
			db.Model(&infrastructure.ProjectPermissionDB{}).
				Where("project_id = ?", "proj-valid").Count(&validCount)
			db.Model(&infrastructure.ProjectPermissionDB{}).
				Where("project_id = ?", "proj-noexp").Count(&noExpCount)

			Expect(expiredCount).To(Equal(int64(0)))
			Expect(validCount).To(Equal(int64(1)))
			Expect(noExpCount).To(Equal(int64(1)))
		})
	})
})
