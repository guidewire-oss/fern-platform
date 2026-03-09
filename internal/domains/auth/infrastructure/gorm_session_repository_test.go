package infrastructure_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/guidewire-oss/fern-platform/internal/domains/auth/domain"
	"github.com/guidewire-oss/fern-platform/internal/domains/auth/infrastructure"
	"github.com/guidewire-oss/fern-platform/pkg/database"
)

var _ = Describe("GormSessionRepository", Label("unit", "infrastructure", "auth"), func() {
	var (
		db   *gorm.DB
		repo *infrastructure.GormSessionRepository
		ctx  context.Context
	)

	BeforeEach(func() {
		var err error
		db, err = gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		})
		Expect(err).NotTo(HaveOccurred())

		err = db.AutoMigrate(&database.UserSession{})
		Expect(err).NotTo(HaveOccurred())

		repo = infrastructure.NewGormSessionRepository(db)
		ctx = context.Background()
	})

	AfterEach(func() {
		sqlDB, err := db.DB()
		if err == nil {
			sqlDB.Close()
		}
	})

	Describe("NewGormSessionRepository", func() {
		It("returns a non-nil repository", func() {
			Expect(infrastructure.NewGormSessionRepository(db)).NotTo(BeNil())
		})
	})

	Describe("Create", func() {
		It("persists a new session", func() {
			session := &domain.Session{
				SessionID:    "sess-create-1",
				UserID:       "user-1",
				AccessToken:  "access-token",
				RefreshToken: "refresh-token",
				ExpiresAt:    time.Now().Add(time.Hour),
				IsActive:     true,
				IPAddress:    "127.0.0.1",
				UserAgent:    "Go-test-client",
				LastActivity: time.Now(),
			}

			Expect(repo.Create(ctx, session)).NotTo(HaveOccurred())

			var count int64
			db.Model(&database.UserSession{}).Where("session_id = ?", "sess-create-1").Count(&count)
			Expect(count).To(Equal(int64(1)))
		})

		It("returns an error for a duplicate session ID", func() {
			session := &domain.Session{
				SessionID: "sess-dup",
				UserID:    "user-dup",
				ExpiresAt: time.Now().Add(time.Hour),
				IsActive:  true,
			}
			Expect(repo.Create(ctx, session)).NotTo(HaveOccurred())
			Expect(repo.Create(ctx, session)).To(HaveOccurred())
		})
	})

	Describe("FindByID", func() {
		BeforeEach(func() {
			Expect(repo.Create(ctx, &domain.Session{
				SessionID: "sess-find",
				UserID:    "user-find",
				ExpiresAt: time.Now().Add(time.Hour),
				IsActive:  true,
			})).NotTo(HaveOccurred())
		})

		It("returns the session when found", func() {
			sess, err := repo.FindByID(ctx, "sess-find")
			Expect(err).NotTo(HaveOccurred())
			Expect(sess.SessionID).To(Equal("sess-find"))
			Expect(sess.UserID).To(Equal("user-find"))
			Expect(sess.IsActive).To(BeTrue())
		})

		It("returns an error when the session does not exist", func() {
			_, err := repo.FindByID(ctx, "sess-missing")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("session not found"))
		})
	})

	Describe("FindActiveByID", func() {
		BeforeEach(func() {
			// Active, non-expired session
			Expect(repo.Create(ctx, &domain.Session{
				SessionID: "sess-active",
				UserID:    "user-active",
				ExpiresAt: time.Now().Add(time.Hour),
				IsActive:  true,
			})).NotTo(HaveOccurred())

			// Active but already expired (inserted directly to bypass time checks)
			Expect(db.Create(&database.UserSession{
				SessionID: "sess-expired",
				UserID:    "user-active",
				ExpiresAt: time.Now().Add(-time.Hour),
				IsActive:  true,
			}).Error).NotTo(HaveOccurred())

			// Create then invalidate to reliably set is_active = false
			Expect(repo.Create(ctx, &domain.Session{
				SessionID: "sess-inactive",
				UserID:    "user-active",
				ExpiresAt: time.Now().Add(time.Hour),
				IsActive:  true,
			})).NotTo(HaveOccurred())
			Expect(repo.Invalidate(ctx, "sess-inactive")).NotTo(HaveOccurred())
		})

		It("returns an active, non-expired session", func() {
			sess, err := repo.FindActiveByID(ctx, "sess-active")
			Expect(err).NotTo(HaveOccurred())
			Expect(sess.SessionID).To(Equal("sess-active"))
		})

		It("returns an error for an expired session", func() {
			_, err := repo.FindActiveByID(ctx, "sess-expired")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("active session not found"))
		})

		It("returns an error for an inactive session", func() {
			_, err := repo.FindActiveByID(ctx, "sess-inactive")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("active session not found"))
		})

		It("returns an error when the session does not exist", func() {
			_, err := repo.FindActiveByID(ctx, "sess-ghost")
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("UpdateActivity", func() {
		BeforeEach(func() {
			Expect(repo.Create(ctx, &domain.Session{
				SessionID:    "sess-activity",
				UserID:       "user-1",
				ExpiresAt:    time.Now().Add(time.Hour),
				IsActive:     true,
				LastActivity: time.Now().Add(-time.Minute),
			})).NotTo(HaveOccurred())
		})

		It("updates the last activity without error", func() {
			Expect(repo.UpdateActivity(ctx, "sess-activity")).NotTo(HaveOccurred())
		})

		It("returns an error when the session does not exist", func() {
			err := repo.UpdateActivity(ctx, "sess-nonexistent")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("session not found"))
		})
	})

	Describe("Invalidate", func() {
		BeforeEach(func() {
			Expect(repo.Create(ctx, &domain.Session{
				SessionID: "sess-invalidate",
				UserID:    "user-1",
				ExpiresAt: time.Now().Add(time.Hour),
				IsActive:  true,
			})).NotTo(HaveOccurred())
		})

		It("marks the session as inactive", func() {
			Expect(repo.Invalidate(ctx, "sess-invalidate")).NotTo(HaveOccurred())

			var dbSession database.UserSession
			db.Where("session_id = ?", "sess-invalidate").First(&dbSession)
			Expect(dbSession.IsActive).To(BeFalse())
		})

		It("returns an error when the session does not exist", func() {
			err := repo.Invalidate(ctx, "sess-ghost")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("session not found"))
		})
	})

	Describe("InvalidateAllForUser", func() {
		BeforeEach(func() {
			for _, id := range []string{"sess-u1-a", "sess-u1-b"} {
				Expect(repo.Create(ctx, &domain.Session{
					SessionID: id,
					UserID:    "user-multi",
					ExpiresAt: time.Now().Add(time.Hour),
					IsActive:  true,
				})).NotTo(HaveOccurred())
			}
		})

		It("invalidates all sessions for the given user", func() {
			Expect(repo.InvalidateAllForUser(ctx, "user-multi")).NotTo(HaveOccurred())

			var count int64
			db.Model(&database.UserSession{}).
				Where("user_id = ? AND is_active = ?", "user-multi", true).
				Count(&count)
			Expect(count).To(Equal(int64(0)))
		})

		It("does not affect sessions for other users", func() {
			Expect(repo.Create(ctx, &domain.Session{
				SessionID: "sess-other-user",
				UserID:    "user-other",
				ExpiresAt: time.Now().Add(time.Hour),
				IsActive:  true,
			})).NotTo(HaveOccurred())

			Expect(repo.InvalidateAllForUser(ctx, "user-multi")).NotTo(HaveOccurred())

			var count int64
			db.Model(&database.UserSession{}).
				Where("user_id = ? AND is_active = ?", "user-other", true).
				Count(&count)
			Expect(count).To(Equal(int64(1)))
		})
	})

	Describe("CleanupExpired", func() {
		BeforeEach(func() {
			// Expired session
			Expect(db.Create(&database.UserSession{
				SessionID: "sess-expired-cleanup",
				UserID:    "user-cleanup",
				ExpiresAt: time.Now().Add(-time.Hour),
				IsActive:  false,
			}).Error).NotTo(HaveOccurred())

			// Valid session
			Expect(repo.Create(ctx, &domain.Session{
				SessionID: "sess-valid-cleanup",
				UserID:    "user-cleanup",
				ExpiresAt: time.Now().Add(time.Hour),
				IsActive:  true,
			})).NotTo(HaveOccurred())
		})

		It("removes only expired sessions", func() {
			Expect(repo.CleanupExpired(ctx)).NotTo(HaveOccurred())

			var expiredCount, validCount int64
			db.Model(&database.UserSession{}).Where("session_id = ?", "sess-expired-cleanup").Count(&expiredCount)
			db.Model(&database.UserSession{}).Where("session_id = ?", "sess-valid-cleanup").Count(&validCount)

			Expect(expiredCount).To(Equal(int64(0)))
			Expect(validCount).To(Equal(int64(1)))
		})
	})
})
