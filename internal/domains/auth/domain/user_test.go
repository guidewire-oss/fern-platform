package domain_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/guidewire-oss/fern-platform/internal/domains/auth/domain"
)

func TestAuthDomain(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Auth Domain Suite")
}

var _ = Describe("User", Label("unit", "domain", "auth"), func() {
	Describe("User Creation", func() {
		It("should create a user with all fields", func() {
			user := &domain.User{
				UserID:        "user-123",
				Email:         "test@example.com",
				Name:          "Test User",
				FirstName:     "Test",
				LastName:      "User",
				Role:          domain.RoleUser,
				Status:        domain.StatusActive,
				EmailVerified: true,
			}

			Expect(user.UserID).To(Equal("user-123"))
			Expect(user.Email).To(Equal("test@example.com"))
			Expect(user.Name).To(Equal("Test User"))
			Expect(user.Status).To(Equal(domain.StatusActive))
			Expect(user.Role).To(Equal(domain.RoleUser))
		})
	})

	Describe("IsAdmin", func() {
		It("should return true for admin users", func() {
			adminUser := &domain.User{
				UserID: "admin-123",
				Role:   domain.RoleAdmin,
			}
			Expect(adminUser.IsAdmin()).To(BeTrue())
		})

		It("should return false for non-admin users", func() {
			regularUser := &domain.User{
				UserID: "user-123",
				Role:   domain.RoleUser,
			}
			Expect(regularUser.IsAdmin()).To(BeFalse())
		})
	})

	Describe("IsActive", func() {
		It("should return true for active users", func() {
			activeUser := &domain.User{
				UserID: "user-123",
				Status: domain.StatusActive,
			}
			Expect(activeUser.IsActive()).To(BeTrue())
		})

		It("should return false for inactive users", func() {
			inactiveUser := &domain.User{
				UserID: "user-123",
				Status: domain.StatusInactive,
			}
			Expect(inactiveUser.IsActive()).To(BeFalse())
		})

		It("should return false for suspended users", func() {
			suspendedUser := &domain.User{
				UserID: "user-123",
				Status: domain.StatusSuspended,
			}
			Expect(suspendedUser.IsActive()).To(BeFalse())
		})
	})

	Describe("HasGroup", func() {
		It("should check group membership correctly", func() {
			user := &domain.User{
				UserID: "user-123",
				Groups: []domain.UserGroup{
					{UserID: "user-123", GroupName: "group1"},
					{UserID: "user-123", GroupName: "group2"},
					{UserID: "user-123", GroupName: "group3"},
				},
			}

			Expect(user.HasGroup("group1")).To(BeTrue())
			Expect(user.HasGroup("group2")).To(BeTrue())
			Expect(user.HasGroup("group3")).To(BeTrue())
			Expect(user.HasGroup("group4")).To(BeFalse())
		})

		It("should handle empty groups", func() {
			user := &domain.User{
				UserID: "user-123",
				Groups: []domain.UserGroup{},
			}

			Expect(user.HasGroup("any-group")).To(BeFalse())
		})

		It("should handle case-sensitive group names", func() {
			user := &domain.User{
				UserID: "user-123",
				Groups: []domain.UserGroup{
					{GroupName: "Fern-Manager"},
					{GroupName: "Atmos-User"},
				},
			}

			Expect(user.HasGroup("Fern-Manager")).To(BeTrue())
			Expect(user.HasGroup("fern-manager")).To(BeFalse())
		})
	})

	Describe("GetTeams", func() {
		It("should extract teams from group names", func() {
			user := &domain.User{
				UserID: "user-123",
				Groups: []domain.UserGroup{
					{GroupName: "fern-managers"},
					{GroupName: "atmos-users"},
					{GroupName: "Other-Group"},
				},
			}

			teams := user.GetTeams()
			Expect(teams).To(ConsistOf("fern", "atmos"))
		})

		It("should handle groups without team pattern", func() {
			user := &domain.User{
				UserID: "user-123",
				Groups: []domain.UserGroup{
					{GroupName: "RandomGroup"},
					{GroupName: "AnotherGroup"},
				},
			}

			teams := user.GetTeams()
			Expect(teams).To(BeEmpty())
		})

		It("should handle empty groups", func() {
			user := &domain.User{
				UserID: "user-123",
				Groups: []domain.UserGroup{},
			}

			teams := user.GetTeams()
			Expect(teams).To(BeEmpty())
		})

		It("should extract teams from multiple groups", func() {
			user := &domain.User{
				UserID: "user-123",
				Groups: []domain.UserGroup{
					{GroupName: "fern-managers"},
					{GroupName: "fern-users"},
				},
			}

			teams := user.GetTeams()
			// Note: GetTeams doesn't deduplicate, so we get "fern" twice
			Expect(teams).To(Equal([]string{"fern", "fern"}))
		})
	})

	Describe("IsTeamManager", func() {
		It("should return true for admin users", func() {
			adminUser := &domain.User{
				UserID: "admin-123",
				Role:   domain.RoleAdmin,
			}
			Expect(adminUser.IsTeamManager()).To(BeTrue())
		})

		It("should return true for users in manager groups", func() {
			user := &domain.User{
				UserID: "user-123",
				Role:   domain.RoleUser,
				Groups: []domain.UserGroup{
					{GroupName: "fern-managers"},
				},
			}
			Expect(user.IsTeamManager()).To(BeTrue())
		})

		It("should return false for regular users without manager groups", func() {
			user := &domain.User{
				UserID: "user-123",
				Role:   domain.RoleUser,
				Groups: []domain.UserGroup{
					{GroupName: "fern-users"},
				},
			}
			Expect(user.IsTeamManager()).To(BeFalse())
		})
	})

	Describe("IsManagerForTeam", func() {
		It("should return true for admin regardless of team", func() {
			adminUser := &domain.User{
				UserID: "admin-123",
				Role:   domain.RoleAdmin,
			}
			Expect(adminUser.IsManagerForTeam("any-team")).To(BeTrue())
		})

		It("should check specific team manager group", func() {
			user := &domain.User{
				UserID: "user-123",
				Role:   domain.RoleUser,
				Groups: []domain.UserGroup{
					{GroupName: "fern-managers"},
					{GroupName: "atmos-users"},
				},
			}

			Expect(user.IsManagerForTeam("fern")).To(BeTrue())
			Expect(user.IsManagerForTeam("atmos")).To(BeFalse())
			Expect(user.IsManagerForTeam("terra")).To(BeFalse())
		})
	})

	Describe("Edge Cases", func() {
		It("should handle very long group lists", func() {
			groups := make([]domain.UserGroup, 100)
			for i := range groups {
				groups[i] = domain.UserGroup{GroupName: "Group-" + string(rune(i))}
			}

			user := &domain.User{
				UserID: "user-123",
				Groups: groups,
			}

			// Should not panic or error
			teams := user.GetTeams()
			Expect(teams).To(BeEmpty()) // No valid team names
		})

		It("should handle special characters in group names", func() {
			user := &domain.User{
				UserID: "user-123",
				Groups: []domain.UserGroup{
					{GroupName: "Group-With-Dash"},
					{GroupName: "Group_With_Underscore"},
					{GroupName: "Group.With.Dot"},
				},
			}

			Expect(user.HasGroup("Group-With-Dash")).To(BeTrue())
			Expect(user.HasGroup("Group_With_Underscore")).To(BeTrue())
			Expect(user.HasGroup("Group.With.Dot")).To(BeTrue())
		})

		It("should handle groups with leading slash", func() {
			user := &domain.User{
				UserID: "user-123",
				Groups: []domain.UserGroup{
					{GroupName: "/fern-managers"},
					{GroupName: "/atmos-users"},
				},
			}

			teams := user.GetTeams()
			Expect(teams).To(ConsistOf("fern", "atmos"))
		})
	})

	Describe("UserRole Constants", func() {
		It("should have correct values", func() {
			Expect(string(domain.RoleAdmin)).To(Equal("admin"))
			Expect(string(domain.RoleUser)).To(Equal("user"))
		})
	})

	Describe("UserStatus Constants", func() {
		It("should have correct values", func() {
			Expect(string(domain.StatusActive)).To(Equal("active"))
			Expect(string(domain.StatusInactive)).To(Equal("inactive"))
			Expect(string(domain.StatusSuspended)).To(Equal("suspended"))
		})
	})
})