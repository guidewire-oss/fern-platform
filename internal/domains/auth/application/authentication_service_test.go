package application_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"

	"github.com/guidewire-oss/fern-platform/internal/domains/auth/application"
	"github.com/guidewire-oss/fern-platform/internal/domains/auth/domain"
)

func TestAuthApplication(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Auth Application Suite")
}

// Mocks for repositories
type MockUserRepository struct {
	mock.Mock
}

func (m *MockUserRepository) Create(ctx context.Context, user *domain.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockUserRepository) Update(ctx context.Context, user *domain.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockUserRepository) FindByID(ctx context.Context, userID string) (*domain.User, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *MockUserRepository) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *MockUserRepository) FindByIDOrEmail(ctx context.Context, userID, email string) (*domain.User, error) {
	args := m.Called(ctx, userID, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *MockUserRepository) UpdateLastLogin(ctx context.Context, userID string, loginTime time.Time) error {
	args := m.Called(ctx, userID, loginTime)
	return args.Error(0)
}

func (m *MockUserRepository) SetUserGroups(ctx context.Context, userID string, groups []string) error {
	args := m.Called(ctx, userID, groups)
	return args.Error(0)
}

func (m *MockUserRepository) GetUserGroups(ctx context.Context, userID string) ([]domain.UserGroup, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.UserGroup), args.Error(1)
}

func (m *MockUserRepository) GrantScope(ctx context.Context, scope domain.UserScope) error {
	args := m.Called(ctx, scope)
	return args.Error(0)
}

func (m *MockUserRepository) RevokeScope(ctx context.Context, userID, scope string) error {
	args := m.Called(ctx, userID, scope)
	return args.Error(0)
}

func (m *MockUserRepository) GetUserScopes(ctx context.Context, userID string) ([]domain.UserScope, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.UserScope), args.Error(1)
}

type MockSessionRepository struct {
	mock.Mock
}

func (m *MockSessionRepository) Create(ctx context.Context, session *domain.Session) error {
	args := m.Called(ctx, session)
	return args.Error(0)
}

func (m *MockSessionRepository) FindByID(ctx context.Context, sessionID string) (*domain.Session, error) {
	args := m.Called(ctx, sessionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Session), args.Error(1)
}

func (m *MockSessionRepository) FindActiveByID(ctx context.Context, sessionID string) (*domain.Session, error) {
	args := m.Called(ctx, sessionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Session), args.Error(1)
}

func (m *MockSessionRepository) UpdateActivity(ctx context.Context, sessionID string) error {
	args := m.Called(ctx, sessionID)
	return args.Error(0)
}

func (m *MockSessionRepository) Invalidate(ctx context.Context, sessionID string) error {
	args := m.Called(ctx, sessionID)
	return args.Error(0)
}

func (m *MockSessionRepository) InvalidateAllForUser(ctx context.Context, userID string) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *MockSessionRepository) CleanupExpired(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

var _ = Describe("AuthenticationService", func() {
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

		authService = application.NewAuthenticationService(
			mockUserRepo,
			mockSessionRepo,
		)
	})

	Describe("AuthenticateWithOAuth", func() {
		Context("successful authentication", func() {
			It("should create new user and session for first-time login", func() {
				userInfo := application.UserInfo{
					Sub:           "oauth-user-123",
					Email:         "newuser@example.com",
					Name:          "New User",
					Groups:        []string{"group1", "group2"},
					EmailVerified: true,
				}

				tokenInfo := application.TokenInfo{
					AccessToken:  "access-token-123",
					RefreshToken: "refresh-token-123",
					IDToken:      "id-token-123",
					ExpiresIn:    3600,
				}

				// Setup mocks - user doesn't exist
				mockUserRepo.On("FindByIDOrEmail", ctx, "oauth-user-123", "newuser@example.com").
					Return(nil, fmt.Errorf("user not found"))
				mockUserRepo.On("Create", ctx, mock.AnythingOfType("*domain.User")).
					Return(nil)
				mockUserRepo.On("SetUserGroups", ctx, "oauth-user-123", []string{"group1", "group2"}).
					Return(nil)
				mockSessionRepo.On("Create", ctx, mock.AnythingOfType("*domain.Session")).
					Return(nil)

				// Execute
				result, err := authService.AuthenticateWithOAuth(ctx, userInfo, tokenInfo, "192.168.1.1", "Mozilla/5.0")

				// Verify
				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.User).NotTo(BeNil())
				Expect(result.Session).NotTo(BeNil())
				Expect(result.User.Email).To(Equal("newuser@example.com"))
				Expect(result.IsNewUser).To(BeTrue())

				mockUserRepo.AssertExpectations(GinkgoT())
				mockSessionRepo.AssertExpectations(GinkgoT())
			})

			It("should update existing user and create new session", func() {
				existingUser := &domain.User{
					UserID:    "oauth-user-123",
					Email:     "user@example.com",
					Name:      "Old Name",
					Role:      domain.RoleUser,
					Status:    domain.StatusActive,
					CreatedAt: time.Now().Add(-30 * 24 * time.Hour),
				}

				userInfo := application.UserInfo{
					Sub:           "oauth-user-123",
					Email:         "updated@example.com",
					Name:          "Updated User",
					Groups:        []string{"new-group1", "new-group2"},
					EmailVerified: true,
				}

				tokenInfo := application.TokenInfo{
					AccessToken: "access-token-456",
					ExpiresIn:   3600,
				}

				// Setup mocks
				mockUserRepo.On("FindByIDOrEmail", ctx, "oauth-user-123", "updated@example.com").
					Return(existingUser, nil)
				mockUserRepo.On("Update", ctx, mock.MatchedBy(func(u *domain.User) bool {
					return u.Email == "updated@example.com" && u.Name == "Updated User"
				})).Return(nil)
				mockUserRepo.On("UpdateLastLogin", ctx, "oauth-user-123", mock.AnythingOfType("time.Time")).
					Return(nil)
				mockUserRepo.On("SetUserGroups", ctx, "oauth-user-123", []string{"new-group1", "new-group2"}).
					Return(nil)
				mockSessionRepo.On("Create", ctx, mock.AnythingOfType("*domain.Session")).
					Return(nil)

				// Execute
				result, err := authService.AuthenticateWithOAuth(ctx, userInfo, tokenInfo, "192.168.1.1", "Mozilla/5.0")

				// Verify
				Expect(err).NotTo(HaveOccurred())
				Expect(result.IsNewUser).To(BeFalse())
				Expect(result.User.Email).To(Equal("updated@example.com"))

				mockUserRepo.AssertExpectations(GinkgoT())
				mockSessionRepo.AssertExpectations(GinkgoT())
			})
		})

		Context("error handling", func() {
			It("should handle user creation failure", func() {
				userInfo := application.UserInfo{
					Sub:   "user-123",
					Email: "user@example.com",
				}

				tokenInfo := application.TokenInfo{
					AccessToken: "token",
					ExpiresIn:   3600,
				}

				mockUserRepo.On("FindByIDOrEmail", ctx, "user-123", "user@example.com").
					Return(nil, fmt.Errorf("not found"))
				mockUserRepo.On("Create", ctx, mock.AnythingOfType("*domain.User")).
					Return(fmt.Errorf("database error"))

				result, err := authService.AuthenticateWithOAuth(ctx, userInfo, tokenInfo, "", "")

				Expect(err).To(HaveOccurred())
				Expect(result).To(BeNil())
			})

			It("should handle session creation failure", func() {
				userInfo := application.UserInfo{
					Sub:   "user-123",
					Email: "user@example.com",
				}

				tokenInfo := application.TokenInfo{
					AccessToken: "token",
					ExpiresIn:   3600,
				}

				mockUserRepo.On("FindByIDOrEmail", ctx, "user-123", "user@example.com").
					Return(nil, fmt.Errorf("not found"))
				mockUserRepo.On("Create", ctx, mock.AnythingOfType("*domain.User")).
					Return(nil)
				mockUserRepo.On("SetUserGroups", ctx, "user-123", mock.Anything).
					Return(nil)
				mockSessionRepo.On("Create", ctx, mock.AnythingOfType("*domain.Session")).
					Return(fmt.Errorf("session creation failed"))

				_, err := authService.AuthenticateWithOAuth(ctx, userInfo, tokenInfo, "", "")

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("session creation failed"))
			})
		})
	})

	Describe("ValidateSession", func() {
		It("should validate active session and update activity", func() {
			activeSession := &domain.Session{
				SessionID: "valid-session-123",
				UserID:    "user-123",
				ExpiresAt: time.Now().Add(12 * time.Hour),
				IsActive:  true,
			}

			mockSessionRepo.On("FindActiveByID", ctx, "valid-session-123").
				Return(activeSession, nil)
			mockSessionRepo.On("UpdateActivity", ctx, "valid-session-123").
				Return(nil)

			validatedSession, err := authService.ValidateSession(ctx, "valid-session-123")

			Expect(err).NotTo(HaveOccurred())
			Expect(validatedSession).To(Equal(activeSession))

			mockSessionRepo.AssertExpectations(GinkgoT())
		})

		It("should handle non-existent session", func() {
			mockSessionRepo.On("FindActiveByID", ctx, "non-existent").
				Return(nil, fmt.Errorf("session not found"))

			session, err := authService.ValidateSession(ctx, "non-existent")

			Expect(err).To(HaveOccurred())
			Expect(session).To(BeNil())
		})
	})

	Describe("Logout", func() {
		It("should invalidate session successfully", func() {
			mockSessionRepo.On("Invalidate", ctx, "session-to-logout").
				Return(nil)

			err := authService.Logout(ctx, "session-to-logout")

			Expect(err).NotTo(HaveOccurred())
			mockSessionRepo.AssertExpectations(GinkgoT())
		})

		It("should handle invalidation error", func() {
			mockSessionRepo.On("Invalidate", ctx, "session-123").
				Return(fmt.Errorf("database error"))

			err := authService.Logout(ctx, "session-123")

			Expect(err).To(HaveOccurred())
		})
	})

	Describe("LogoutAllSessions", func() {
		It("should invalidate all user sessions", func() {
			mockSessionRepo.On("InvalidateAllForUser", ctx, "user-123").
				Return(nil)

			err := authService.LogoutAllSessions(ctx, "user-123")

			Expect(err).NotTo(HaveOccurred())
			mockSessionRepo.AssertExpectations(GinkgoT())
		})
	})

	Describe("Role Determination", func() {
		It("should assign admin role based on admin flag", func() {
			adminInfo := application.UserInfo{
				Sub:    "admin-user",
				Email:  "admin@example.com",
				Roles:  []string{"admin"},
			}

			tokenInfo := application.TokenInfo{ExpiresIn: 3600}

			mockUserRepo.On("FindByIDOrEmail", ctx, "admin-user", "admin@example.com").
				Return(nil, fmt.Errorf("not found"))
			mockUserRepo.On("Create", ctx, mock.MatchedBy(func(u *domain.User) bool {
				return u.Role == domain.RoleAdmin
			})).Return(nil)
			mockUserRepo.On("SetUserGroups", ctx, "admin-user", mock.Anything).
				Return(nil)
			mockSessionRepo.On("Create", ctx, mock.Anything).Return(nil)

			result, err := authService.AuthenticateWithOAuth(ctx, adminInfo, tokenInfo, "", "")

			Expect(err).NotTo(HaveOccurred())
			Expect(result.User.Role).To(Equal(domain.RoleAdmin))
		})
	})
})