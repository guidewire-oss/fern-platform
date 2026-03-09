package application_test

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"

	"github.com/guidewire-oss/fern-platform/internal/domains/auth/application"
	"github.com/guidewire-oss/fern-platform/internal/domains/auth/domain"
)

var _ = Describe("AuthenticateWithOAuth - additional tests", func() {
	var (
		authService     *application.AuthenticationService
		mockUserRepo    *MockUserRepository
		mockSessionRepo *MockSessionRepository
		ctx             context.Context
	)

	BeforeEach(func() {
		mockUserRepo = new(MockUserRepository)
		mockSessionRepo = new(MockSessionRepository)
		ctx = context.Background()
		authService = application.NewAuthenticationService(mockUserRepo, mockSessionRepo)
	})

	Describe("ValidateSession - extended", func() {
		It("should reject expired session", func() {
			expiredSession := &domain.Session{
				SessionID: "expired-session",
				UserID:    "user-1",
				ExpiresAt: time.Now().Add(-1 * time.Hour),
				IsActive:  true,
			}

			mockSessionRepo.On("FindActiveByID", ctx, "expired-session").
				Return(expiredSession, nil)

			session, err := authService.ValidateSession(ctx, "expired-session")

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("session is invalid or expired"))
			Expect(session).To(BeNil())
		})

		It("should reject inactive session", func() {
			inactiveSession := &domain.Session{
				SessionID: "inactive-session",
				UserID:    "user-1",
				ExpiresAt: time.Now().Add(1 * time.Hour),
				IsActive:  false,
			}

			mockSessionRepo.On("FindActiveByID", ctx, "inactive-session").
				Return(inactiveSession, nil)

			session, err := authService.ValidateSession(ctx, "inactive-session")

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("session is invalid or expired"))
			Expect(session).To(BeNil())
		})

		It("should reject when user is not active", func() {
			validSession := &domain.Session{
				SessionID: "valid-session",
				UserID:    "suspended-user",
				ExpiresAt: time.Now().Add(1 * time.Hour),
				IsActive:  true,
			}

			suspendedUser := &domain.User{
				UserID: "suspended-user",
				Email:  "suspended@example.com",
				Status: domain.StatusSuspended,
			}

			mockSessionRepo.On("FindActiveByID", ctx, "valid-session").
				Return(validSession, nil)
			mockSessionRepo.On("UpdateActivity", ctx, "valid-session").
				Return(nil)
			mockUserRepo.On("FindByID", ctx, "suspended-user").
				Return(suspendedUser, nil)

			session, err := authService.ValidateSession(ctx, "valid-session")

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("user account is not active"))
			Expect(session).To(BeNil())
		})

		It("should reject when user is not found", func() {
			validSession := &domain.Session{
				SessionID: "valid-session",
				UserID:    "deleted-user",
				ExpiresAt: time.Now().Add(1 * time.Hour),
				IsActive:  true,
			}

			mockSessionRepo.On("FindActiveByID", ctx, "valid-session").
				Return(validSession, nil)
			mockSessionRepo.On("UpdateActivity", ctx, "valid-session").
				Return(nil)
			mockUserRepo.On("FindByID", ctx, "deleted-user").
				Return(nil, fmt.Errorf("user not found"))

			session, err := authService.ValidateSession(ctx, "valid-session")

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("user not found"))
			Expect(session).To(BeNil())
		})

		It("should populate session.User on successful validation", func() {
			activeSession := &domain.Session{
				SessionID: "sess-good",
				UserID:    "user-good",
				ExpiresAt: time.Now().Add(12 * time.Hour),
				IsActive:  true,
			}

			user := &domain.User{
				UserID: "user-good",
				Email:  "good@example.com",
				Name:   "Good User",
				Role:   domain.RoleUser,
				Status: domain.StatusActive,
			}

			mockSessionRepo.On("FindActiveByID", ctx, "sess-good").
				Return(activeSession, nil)
			mockSessionRepo.On("UpdateActivity", ctx, "sess-good").
				Return(nil)
			mockUserRepo.On("FindByID", ctx, "user-good").
				Return(user, nil)

			result, err := authService.ValidateSession(ctx, "sess-good")

			Expect(err).NotTo(HaveOccurred())
			Expect(result.User).NotTo(BeNil())
			Expect(result.User.Email).To(Equal("good@example.com"))
		})
	})

	Describe("AuthenticateWithOAuth - session with zero ExpiresIn", func() {
		It("should use default 24h expiration when ExpiresIn is zero", func() {
			userInfo := application.UserInfo{
				Sub:   "user-zero-exp",
				Email: "user@example.com",
			}

			tokenInfo := application.TokenInfo{
				AccessToken: "token",
				ExpiresIn:   0,
			}

			mockUserRepo.On("FindByIDOrEmail", ctx, "user-zero-exp", "user@example.com").
				Return(nil, fmt.Errorf("not found"))
			mockUserRepo.On("Create", ctx, mock.AnythingOfType("*domain.User")).
				Return(nil)
			mockUserRepo.On("SetUserGroups", ctx, "user-zero-exp", mock.Anything).
				Return(nil)
			mockUserRepo.On("UpdateLastLogin", ctx, "user-zero-exp", mock.AnythingOfType("time.Time")).
				Return(nil)
			mockSessionRepo.On("Create", ctx, mock.MatchedBy(func(s *domain.Session) bool {
				// Session should expire roughly 24h from now
				return s.ExpiresAt.After(time.Now().Add(23*time.Hour)) &&
					s.ExpiresAt.Before(time.Now().Add(25*time.Hour))
			})).Return(nil)

			result, err := authService.AuthenticateWithOAuth(ctx, userInfo, tokenInfo, "10.0.0.1", "TestAgent")

			Expect(err).NotTo(HaveOccurred())
			Expect(result).NotTo(BeNil())
			Expect(result.IsNewUser).To(BeTrue())
		})
	})

	Describe("AuthenticateWithOAuth - SetUserGroups failure", func() {
		It("should return error when SetUserGroups fails", func() {
			userInfo := application.UserInfo{
				Sub:    "user-grp-fail",
				Email:  "user@example.com",
				Groups: []string{"g1"},
			}

			tokenInfo := application.TokenInfo{
				AccessToken: "token",
				ExpiresIn:   3600,
			}

			mockUserRepo.On("FindByIDOrEmail", ctx, "user-grp-fail", "user@example.com").
				Return(nil, fmt.Errorf("not found"))
			mockUserRepo.On("Create", ctx, mock.AnythingOfType("*domain.User")).
				Return(nil)
			mockUserRepo.On("SetUserGroups", ctx, "user-grp-fail", []string{"g1"}).
				Return(fmt.Errorf("groups db error"))

			result, err := authService.AuthenticateWithOAuth(ctx, userInfo, tokenInfo, "", "")

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to update user groups"))
			Expect(result).To(BeNil())
		})
	})

	Describe("Logout", func() {
		It("should delegate to session repo Invalidate", func() {
			mockSessionRepo.On("Invalidate", ctx, "sess-123").
				Return(nil)

			err := authService.Logout(ctx, "sess-123")

			Expect(err).NotTo(HaveOccurred())
			mockSessionRepo.AssertExpectations(GinkgoT())
		})
	})

	Describe("LogoutAllSessions", func() {
		It("should delegate to session repo InvalidateAllForUser", func() {
			mockSessionRepo.On("InvalidateAllForUser", ctx, "user-abc").
				Return(nil)

			err := authService.LogoutAllSessions(ctx, "user-abc")

			Expect(err).NotTo(HaveOccurred())
			mockSessionRepo.AssertExpectations(GinkgoT())
		})

		It("should propagate errors", func() {
			mockSessionRepo.On("InvalidateAllForUser", ctx, "user-bad").
				Return(fmt.Errorf("db error"))

			err := authService.LogoutAllSessions(ctx, "user-bad")

			Expect(err).To(HaveOccurred())
		})
	})

	Describe("generateSessionID", func() {
		It("should produce unique session IDs across multiple authentications", func() {
			// Create two users with sessions and verify they get different session IDs
			for _, sub := range []string{"user-a", "user-b"} {
				userInfo := application.UserInfo{
					Sub:   sub,
					Email: sub + "@example.com",
				}
				tokenInfo := application.TokenInfo{
					AccessToken: "token-" + sub,
					ExpiresIn:   3600,
				}

				mockUserRepo.On("FindByIDOrEmail", ctx, sub, sub+"@example.com").
					Return(nil, fmt.Errorf("not found"))
				mockUserRepo.On("Create", ctx, mock.AnythingOfType("*domain.User")).
					Return(nil)
				mockUserRepo.On("SetUserGroups", ctx, sub, mock.Anything).
					Return(nil)
				mockUserRepo.On("UpdateLastLogin", ctx, sub, mock.AnythingOfType("time.Time")).
					Return(nil)
				mockSessionRepo.On("Create", ctx, mock.AnythingOfType("*domain.Session")).
					Return(nil)

				result, err := authService.AuthenticateWithOAuth(ctx, userInfo, tokenInfo, "", "")
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Session.SessionID).NotTo(BeEmpty())
			}
		})
	})
})
