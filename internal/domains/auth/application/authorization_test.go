package application_test

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/guidewire-oss/fern-platform/internal/domains/auth/application"
	"github.com/guidewire-oss/fern-platform/internal/domains/auth/domain"
)

var _ = Describe("AuthorizationService", func() {
	var (
		authzService *application.AuthorizationService
		mockUserRepo *MockUserRepository
		ctx          context.Context
	)

	BeforeEach(func() {
		mockUserRepo = new(MockUserRepository)
		ctx = context.Background()
		authzService = application.NewAuthorizationService(mockUserRepo)
	})

	Describe("NewAuthorizationService", func() {
		It("should create service with user repo", func() {
			Expect(authzService).NotTo(BeNil())
		})
	})

	Describe("CanAccessProject", func() {
		Context("admin user", func() {
			It("should allow any action for admin user", func() {
				adminUser := &domain.User{
					UserID: "admin-1",
					Role:   domain.RoleAdmin,
					Status: domain.StatusActive,
				}

				allowed, err := authzService.CanAccessProject(ctx, adminUser, "any-project", "write")
				Expect(err).NotTo(HaveOccurred())
				Expect(allowed).To(BeTrue())
			})

			It("should not call repo for admin", func() {
				adminUser := &domain.User{
					UserID: "admin-1",
					Role:   domain.RoleAdmin,
				}

				allowed, err := authzService.CanAccessProject(ctx, adminUser, "proj", "read")
				Expect(err).NotTo(HaveOccurred())
				Expect(allowed).To(BeTrue())
				// mockUserRepo should not have been called
				mockUserRepo.AssertNotCalled(GinkgoT(), "GetUserScopes")
			})
		})

		Context("regular user with scopes", func() {
			It("should allow access with exact scope match", func() {
				user := &domain.User{
					UserID: "user-1",
					Role:   domain.RoleUser,
				}

				scopes := []domain.UserScope{
					{UserID: "user-1", Scope: "project:read:proj-abc"},
				}

				mockUserRepo.On("GetUserScopes", ctx, "user-1").Return(scopes, nil)

				allowed, err := authzService.CanAccessProject(ctx, user, "proj-abc", "read")
				Expect(err).NotTo(HaveOccurred())
				Expect(allowed).To(BeTrue())
			})

			It("should deny access with wrong action", func() {
				user := &domain.User{
					UserID: "user-1",
					Role:   domain.RoleUser,
				}

				scopes := []domain.UserScope{
					{UserID: "user-1", Scope: "project:read:proj-abc"},
				}

				mockUserRepo.On("GetUserScopes", ctx, "user-1").Return(scopes, nil)

				allowed, err := authzService.CanAccessProject(ctx, user, "proj-abc", "write")
				Expect(err).NotTo(HaveOccurred())
				Expect(allowed).To(BeFalse())
			})

			It("should deny access with wrong project", func() {
				user := &domain.User{
					UserID: "user-1",
					Role:   domain.RoleUser,
				}

				scopes := []domain.UserScope{
					{UserID: "user-1", Scope: "project:read:proj-abc"},
				}

				mockUserRepo.On("GetUserScopes", ctx, "user-1").Return(scopes, nil)

				allowed, err := authzService.CanAccessProject(ctx, user, "proj-xyz", "read")
				Expect(err).NotTo(HaveOccurred())
				Expect(allowed).To(BeFalse())
			})

			It("should allow access with wildcard action", func() {
				user := &domain.User{
					UserID: "user-2",
					Role:   domain.RoleUser,
				}

				scopes := []domain.UserScope{
					{UserID: "user-2", Scope: "project:*:proj-abc"},
				}

				mockUserRepo.On("GetUserScopes", ctx, "user-2").Return(scopes, nil)

				allowed, err := authzService.CanAccessProject(ctx, user, "proj-abc", "write")
				Expect(err).NotTo(HaveOccurred())
				Expect(allowed).To(BeTrue())
			})

			It("should allow access with wildcard project", func() {
				user := &domain.User{
					UserID: "user-3",
					Role:   domain.RoleUser,
				}

				scopes := []domain.UserScope{
					{UserID: "user-3", Scope: "project:read:*"},
				}

				mockUserRepo.On("GetUserScopes", ctx, "user-3").Return(scopes, nil)

				allowed, err := authzService.CanAccessProject(ctx, user, "any-project", "read")
				Expect(err).NotTo(HaveOccurred())
				Expect(allowed).To(BeTrue())
			})

			It("should allow access with full wildcard scope", func() {
				user := &domain.User{
					UserID: "user-4",
					Role:   domain.RoleUser,
				}

				scopes := []domain.UserScope{
					{UserID: "user-4", Scope: "project:*:*"},
				}

				mockUserRepo.On("GetUserScopes", ctx, "user-4").Return(scopes, nil)

				allowed, err := authzService.CanAccessProject(ctx, user, "anything", "delete")
				Expect(err).NotTo(HaveOccurred())
				Expect(allowed).To(BeTrue())
			})

			It("should skip expired scopes", func() {
				user := &domain.User{
					UserID: "user-5",
					Role:   domain.RoleUser,
				}

				expired := time.Now().Add(-1 * time.Hour)
				scopes := []domain.UserScope{
					{UserID: "user-5", Scope: "project:read:proj-abc", ExpiresAt: &expired},
				}

				mockUserRepo.On("GetUserScopes", ctx, "user-5").Return(scopes, nil)

				allowed, err := authzService.CanAccessProject(ctx, user, "proj-abc", "read")
				Expect(err).NotTo(HaveOccurred())
				Expect(allowed).To(BeFalse())
			})

			It("should use non-expired scope", func() {
				user := &domain.User{
					UserID: "user-6",
					Role:   domain.RoleUser,
				}

				future := time.Now().Add(1 * time.Hour)
				scopes := []domain.UserScope{
					{UserID: "user-6", Scope: "project:read:proj-abc", ExpiresAt: &future},
				}

				mockUserRepo.On("GetUserScopes", ctx, "user-6").Return(scopes, nil)

				allowed, err := authzService.CanAccessProject(ctx, user, "proj-abc", "read")
				Expect(err).NotTo(HaveOccurred())
				Expect(allowed).To(BeTrue())
			})

			It("should accept scope with nil ExpiresAt as non-expiring", func() {
				user := &domain.User{
					UserID: "user-7",
					Role:   domain.RoleUser,
				}

				scopes := []domain.UserScope{
					{UserID: "user-7", Scope: "project:write:proj-abc", ExpiresAt: nil},
				}

				mockUserRepo.On("GetUserScopes", ctx, "user-7").Return(scopes, nil)

				allowed, err := authzService.CanAccessProject(ctx, user, "proj-abc", "write")
				Expect(err).NotTo(HaveOccurred())
				Expect(allowed).To(BeTrue())
			})

			It("should ignore non-project scopes", func() {
				user := &domain.User{
					UserID: "user-8",
					Role:   domain.RoleUser,
				}

				scopes := []domain.UserScope{
					{UserID: "user-8", Scope: "admin:read:all"},
					{UserID: "user-8", Scope: "invalid-scope"},
				}

				mockUserRepo.On("GetUserScopes", ctx, "user-8").Return(scopes, nil)

				allowed, err := authzService.CanAccessProject(ctx, user, "proj-abc", "read")
				Expect(err).NotTo(HaveOccurred())
				Expect(allowed).To(BeFalse())
			})

			It("should deny when user has no scopes", func() {
				user := &domain.User{
					UserID: "user-9",
					Role:   domain.RoleUser,
				}

				mockUserRepo.On("GetUserScopes", ctx, "user-9").Return([]domain.UserScope{}, nil)

				allowed, err := authzService.CanAccessProject(ctx, user, "proj-abc", "read")
				Expect(err).NotTo(HaveOccurred())
				Expect(allowed).To(BeFalse())
			})
		})

		Context("error handling", func() {
			It("should return error when GetUserScopes fails", func() {
				user := &domain.User{
					UserID: "user-err",
					Role:   domain.RoleUser,
				}

				mockUserRepo.On("GetUserScopes", ctx, "user-err").
					Return(nil, fmt.Errorf("database error"))

				allowed, err := authzService.CanAccessProject(ctx, user, "proj", "read")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to get user scopes"))
				Expect(allowed).To(BeFalse())
			})
		})

		Context("manager user", func() {
			It("should check scopes for manager user (not auto-allowed)", func() {
				managerUser := &domain.User{
					UserID: "mgr-1",
					Role:   domain.RoleManager,
				}

				scopes := []domain.UserScope{
					{UserID: "mgr-1", Scope: "project:read:proj-abc"},
				}

				mockUserRepo.On("GetUserScopes", ctx, "mgr-1").Return(scopes, nil)

				allowed, err := authzService.CanAccessProject(ctx, managerUser, "proj-abc", "read")
				Expect(err).NotTo(HaveOccurred())
				Expect(allowed).To(BeTrue())
			})
		})
	})

	Describe("CanManageTeam", func() {
		It("should allow admin to manage any team", func() {
			admin := &domain.User{
				UserID: "admin-1",
				Role:   domain.RoleAdmin,
			}

			Expect(authzService.CanManageTeam(ctx, admin, "any-team")).To(BeTrue())
		})

		It("should allow manager to manage any team", func() {
			manager := &domain.User{
				UserID: "mgr-1",
				Role:   domain.RoleManager,
			}

			Expect(authzService.CanManageTeam(ctx, manager, "some-team")).To(BeTrue())
		})

		It("should allow user with team-specific manager group", func() {
			user := &domain.User{
				UserID: "user-1",
				Role:   domain.RoleUser,
				Groups: []domain.UserGroup{
					{UserID: "user-1", GroupName: "devops-managers"},
				},
			}

			Expect(authzService.CanManageTeam(ctx, user, "devops")).To(BeTrue())
		})

		It("should deny user without matching team manager group", func() {
			user := &domain.User{
				UserID: "user-1",
				Role:   domain.RoleUser,
				Groups: []domain.UserGroup{
					{UserID: "user-1", GroupName: "devops-users"},
				},
			}

			Expect(authzService.CanManageTeam(ctx, user, "devops")).To(BeFalse())
		})

		It("should deny user with no groups", func() {
			user := &domain.User{
				UserID: "user-1",
				Role:   domain.RoleUser,
			}

			Expect(authzService.CanManageTeam(ctx, user, "team-x")).To(BeFalse())
		})
	})

	Describe("GrantScope", func() {
		It("should delegate to user repo", func() {
			scope := domain.UserScope{
				UserID: "user-1",
				Scope:  "project:read:proj-abc",
			}

			mockUserRepo.On("GrantScope", ctx, scope).Return(nil)

			err := authzService.GrantScope(ctx, scope)
			Expect(err).NotTo(HaveOccurred())
			mockUserRepo.AssertExpectations(GinkgoT())
		})

		It("should propagate error from repo", func() {
			scope := domain.UserScope{
				UserID: "user-1",
				Scope:  "project:read:proj-abc",
			}

			mockUserRepo.On("GrantScope", ctx, scope).Return(fmt.Errorf("grant failed"))

			err := authzService.GrantScope(ctx, scope)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("grant failed"))
		})
	})

	Describe("RevokeScope", func() {
		It("should delegate to user repo", func() {
			mockUserRepo.On("RevokeScope", ctx, "user-1", "project:read:proj-abc").Return(nil)

			err := authzService.RevokeScope(ctx, "user-1", "project:read:proj-abc")
			Expect(err).NotTo(HaveOccurred())
			mockUserRepo.AssertExpectations(GinkgoT())
		})

		It("should propagate error from repo", func() {
			mockUserRepo.On("RevokeScope", ctx, "user-1", "scope").
				Return(fmt.Errorf("revoke failed"))

			err := authzService.RevokeScope(ctx, "user-1", "scope")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("revoke failed"))
		})
	})
})
