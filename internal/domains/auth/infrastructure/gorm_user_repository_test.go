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

var _ = Describe("GormUserRepository", Label("unit", "infrastructure", "auth"), func() {
	var (
		db   *gorm.DB
		repo *infrastructure.GormUserRepository
		ctx  context.Context
	)

	BeforeEach(func() {
		var err error
		db, err = gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		})
		Expect(err).NotTo(HaveOccurred())

		err = db.AutoMigrate(
			&database.User{},
			&database.UserGroup{},
			&database.UserScope{},
		)
		Expect(err).NotTo(HaveOccurred())

		repo = infrastructure.NewGormUserRepository(db)
		ctx = context.Background()
	})

	AfterEach(func() {
		sqlDB, err := db.DB()
		if err == nil {
			sqlDB.Close()
		}
	})

	Describe("NewGormUserRepository", func() {
		It("returns a non-nil repository", func() {
			Expect(infrastructure.NewGormUserRepository(db)).NotTo(BeNil())
		})
	})

	Describe("Create", func() {
		It("persists a new user", func() {
			user := &domain.User{
				UserID:        "user-create-1",
				Email:         "alice@example.com",
				Name:          "Alice Smith",
				FirstName:     "Alice",
				LastName:      "Smith",
				Role:          domain.RoleUser,
				Status:        domain.StatusActive,
				EmailVerified: true,
			}

			Expect(repo.Create(ctx, user)).NotTo(HaveOccurred())

			var count int64
			db.Model(&database.User{}).Where("user_id = ?", "user-create-1").Count(&count)
			Expect(count).To(Equal(int64(1)))
		})

		It("returns an error for duplicate user_id", func() {
			user := &domain.User{
				UserID: "user-dup",
				Email:  "dup@example.com",
				Name:   "Dup User",
				Role:   domain.RoleUser,
				Status: domain.StatusActive,
			}
			Expect(repo.Create(ctx, user)).NotTo(HaveOccurred())
			Expect(repo.Create(ctx, user)).To(HaveOccurred())
		})
	})

	Describe("Update", func() {
		BeforeEach(func() {
			Expect(db.Create(&database.User{
				UserID: "user-upd",
				Email:  "upd@example.com",
				Name:   "Original Name",
				Role:   string(domain.RoleUser),
				Status: string(domain.StatusActive),
			}).Error).NotTo(HaveOccurred())
		})

		It("updates an existing user's fields", func() {
			err := repo.Update(ctx, &domain.User{
				UserID: "user-upd",
				Email:  "new@example.com",
				Name:   "New Name",
				Role:   domain.RoleAdmin,
				Status: domain.StatusActive,
			})
			Expect(err).NotTo(HaveOccurred())

			var dbUser database.User
			db.Where("user_id = ?", "user-upd").First(&dbUser)
			Expect(dbUser.Email).To(Equal("new@example.com"))
			Expect(dbUser.Name).To(Equal("New Name"))
			Expect(dbUser.Role).To(Equal(string(domain.RoleAdmin)))
		})

		It("returns an error for a non-existent user", func() {
			err := repo.Update(ctx, &domain.User{
				UserID: "user-ghost",
				Email:  "ghost@example.com",
				Name:   "Ghost",
				Role:   domain.RoleUser,
				Status: domain.StatusActive,
			})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("user not found"))
		})
	})

	Describe("FindByID", func() {
		BeforeEach(func() {
			Expect(db.Create(&database.User{
				UserID: "user-find",
				Email:  "find@example.com",
				Name:   "Find User",
				Role:   string(domain.RoleUser),
				Status: string(domain.StatusActive),
			}).Error).NotTo(HaveOccurred())
		})

		It("returns the user when found", func() {
			user, err := repo.FindByID(ctx, "user-find")
			Expect(err).NotTo(HaveOccurred())
			Expect(user.UserID).To(Equal("user-find"))
			Expect(user.Email).To(Equal("find@example.com"))
		})

		It("returns an error when the user does not exist", func() {
			_, err := repo.FindByID(ctx, "user-missing")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("user not found"))
		})
	})

	Describe("FindByEmail", func() {
		BeforeEach(func() {
			Expect(db.Create(&database.User{
				UserID: "user-email",
				Email:  "email@example.com",
				Name:   "Email User",
				Role:   string(domain.RoleUser),
				Status: string(domain.StatusActive),
			}).Error).NotTo(HaveOccurred())
		})

		It("returns the user when found by email", func() {
			user, err := repo.FindByEmail(ctx, "email@example.com")
			Expect(err).NotTo(HaveOccurred())
			Expect(user.Email).To(Equal("email@example.com"))
			Expect(user.UserID).To(Equal("user-email"))
		})

		It("returns an error when email does not exist", func() {
			_, err := repo.FindByEmail(ctx, "nobody@example.com")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("user not found"))
		})
	})

	Describe("FindByIDOrEmail", func() {
		BeforeEach(func() {
			Expect(db.Create(&database.User{
				UserID: "user-idor",
				Email:  "idor@example.com",
				Name:   "IDOrEmail User",
				Role:   string(domain.RoleUser),
				Status: string(domain.StatusActive),
			}).Error).NotTo(HaveOccurred())
		})

		It("finds the user by ID", func() {
			user, err := repo.FindByIDOrEmail(ctx, "user-idor", "")
			Expect(err).NotTo(HaveOccurred())
			Expect(user.UserID).To(Equal("user-idor"))
		})

		It("finds the user by email when ID does not match", func() {
			user, err := repo.FindByIDOrEmail(ctx, "x-no-match", "idor@example.com")
			Expect(err).NotTo(HaveOccurred())
			Expect(user.Email).To(Equal("idor@example.com"))
		})

		It("returns an error when neither ID nor email matches", func() {
			_, err := repo.FindByIDOrEmail(ctx, "no-id", "no@email.com")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("user not found"))
		})
	})

	Describe("UpdateLastLogin", func() {
		BeforeEach(func() {
			Expect(db.Create(&database.User{
				UserID: "user-login",
				Email:  "login@example.com",
				Name:   "Login User",
				Role:   string(domain.RoleUser),
				Status: string(domain.StatusActive),
			}).Error).NotTo(HaveOccurred())
		})

		It("updates the last login timestamp without error", func() {
			loginTime := time.Now().UTC().Truncate(time.Second)
			Expect(repo.UpdateLastLogin(ctx, "user-login", loginTime)).NotTo(HaveOccurred())

			var dbUser database.User
			db.Where("user_id = ?", "user-login").First(&dbUser)
			Expect(dbUser.LastLoginAt).NotTo(BeNil())
		})
	})

	Describe("SetUserGroups", func() {
		BeforeEach(func() {
			Expect(db.Create(&database.User{
				UserID: "user-groups",
				Email:  "groups@example.com",
				Name:   "Groups User",
				Role:   string(domain.RoleUser),
				Status: string(domain.StatusActive),
			}).Error).NotTo(HaveOccurred())
		})

		It("creates the specified group memberships", func() {
			Expect(repo.SetUserGroups(ctx, "user-groups", []string{"engineering", "platform"})).NotTo(HaveOccurred())

			var count int64
			db.Model(&database.UserGroup{}).Where("user_id = ?", "user-groups").Count(&count)
			Expect(count).To(Equal(int64(2)))
		})

		It("replaces existing groups on subsequent calls", func() {
			Expect(repo.SetUserGroups(ctx, "user-groups", []string{"engineering", "platform"})).NotTo(HaveOccurred())
			Expect(repo.SetUserGroups(ctx, "user-groups", []string{"security"})).NotTo(HaveOccurred())

			var count int64
			db.Model(&database.UserGroup{}).Where("user_id = ?", "user-groups").Count(&count)
			Expect(count).To(Equal(int64(1)))
		})

		It("clears all groups when given an empty slice", func() {
			Expect(repo.SetUserGroups(ctx, "user-groups", []string{"engineering"})).NotTo(HaveOccurred())
			Expect(repo.SetUserGroups(ctx, "user-groups", []string{})).NotTo(HaveOccurred())

			var count int64
			db.Model(&database.UserGroup{}).Where("user_id = ?", "user-groups").Count(&count)
			Expect(count).To(Equal(int64(0)))
		})
	})

	Describe("GetUserGroups", func() {
		BeforeEach(func() {
			Expect(db.Create(&database.User{
				UserID: "user-getgroups",
				Email:  "getgroups@example.com",
				Name:   "GetGroups User",
				Role:   string(domain.RoleUser),
				Status: string(domain.StatusActive),
			}).Error).NotTo(HaveOccurred())
			Expect(repo.SetUserGroups(ctx, "user-getgroups", []string{"alpha", "beta"})).NotTo(HaveOccurred())
		})

		It("returns the correct group memberships", func() {
			groups, err := repo.GetUserGroups(ctx, "user-getgroups")
			Expect(err).NotTo(HaveOccurred())
			Expect(groups).To(HaveLen(2))

			names := make([]string, len(groups))
			for i, g := range groups {
				names[i] = g.GroupName
			}
			Expect(names).To(ConsistOf("alpha", "beta"))
		})

		It("returns an empty slice when the user has no groups", func() {
			groups, err := repo.GetUserGroups(ctx, "user-nogroups")
			Expect(err).NotTo(HaveOccurred())
			Expect(groups).To(BeEmpty())
		})
	})

	Describe("GrantScope", func() {
		BeforeEach(func() {
			Expect(db.Create(&database.User{
				UserID: "user-scope",
				Email:  "scope@example.com",
				Name:   "Scope User",
				Role:   string(domain.RoleUser),
				Status: string(domain.StatusActive),
			}).Error).NotTo(HaveOccurred())
		})

		It("grants a scope to the user", func() {
			scope := domain.UserScope{
				UserID:    "user-scope",
				Scope:     "read:projects",
				GrantedBy: "admin",
			}
			Expect(repo.GrantScope(ctx, scope)).NotTo(HaveOccurred())

			var count int64
			db.Model(&database.UserScope{}).Where("user_id = ? AND scope = ?", "user-scope", "read:projects").Count(&count)
			Expect(count).To(Equal(int64(1)))
		})
	})

	Describe("RevokeScope", func() {
		BeforeEach(func() {
			Expect(db.Create(&database.User{
				UserID: "user-revoke",
				Email:  "revoke@example.com",
				Name:   "Revoke User",
				Role:   string(domain.RoleUser),
				Status: string(domain.StatusActive),
			}).Error).NotTo(HaveOccurred())
			Expect(repo.GrantScope(ctx, domain.UserScope{
				UserID:    "user-revoke",
				Scope:     "write:projects",
				GrantedBy: "admin",
			})).NotTo(HaveOccurred())
		})

		It("removes the granted scope", func() {
			Expect(repo.RevokeScope(ctx, "user-revoke", "write:projects")).NotTo(HaveOccurred())

			var count int64
			db.Model(&database.UserScope{}).Where("user_id = ? AND scope = ?", "user-revoke", "write:projects").Count(&count)
			Expect(count).To(Equal(int64(0)))
		})
	})

	Describe("GetUserScopes", func() {
		BeforeEach(func() {
			Expect(db.Create(&database.User{
				UserID: "user-scopes",
				Email:  "scopes@example.com",
				Name:   "Scopes User",
				Role:   string(domain.RoleUser),
				Status: string(domain.StatusActive),
			}).Error).NotTo(HaveOccurred())
			for _, s := range []string{"read:tests", "write:tests"} {
				Expect(repo.GrantScope(ctx, domain.UserScope{
					UserID:    "user-scopes",
					Scope:     s,
					GrantedBy: "admin",
				})).NotTo(HaveOccurred())
			}
		})

		It("returns all scopes for the user", func() {
			scopes, err := repo.GetUserScopes(ctx, "user-scopes")
			Expect(err).NotTo(HaveOccurred())
			Expect(scopes).To(HaveLen(2))

			names := make([]string, len(scopes))
			for i, s := range scopes {
				names[i] = s.Scope
			}
			Expect(names).To(ConsistOf("read:tests", "write:tests"))
		})

		It("returns an empty slice when the user has no scopes", func() {
			scopes, err := repo.GetUserScopes(ctx, "user-noscopes")
			Expect(err).NotTo(HaveOccurred())
			Expect(scopes).To(BeEmpty())
		})
	})
})
