package application

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/guidewire-oss/fern-platform/internal/domains/auth/domain"
	"github.com/spf13/viper"
)

// AuthenticationService handles user authentication
type AuthenticationService struct {
	userRepo    domain.UserRepository
	sessionRepo domain.SessionRepository
}

// NewAuthenticationService creates a new authentication service
func NewAuthenticationService(userRepo domain.UserRepository, sessionRepo domain.SessionRepository) *AuthenticationService {
	return &AuthenticationService{
		userRepo:    userRepo,
		sessionRepo: sessionRepo,
	}
}

// UserInfo represents user information from OAuth provider
type UserInfo struct {
	Sub           string
	Email         string
	Name          string
	FirstName     string
	LastName      string
	Picture       string
	Groups        []string
	Roles         []string
	EmailVerified bool
}

// TokenInfo represents OAuth token information
type TokenInfo struct {
	AccessToken  string
	RefreshToken string
	IDToken      string
	ExpiresIn    int
}

// AuthenticateResult represents the result of authentication
type AuthenticateResult struct {
	User      *domain.User
	Session   *domain.Session
	IsNewUser bool
}

// AuthenticateWithOAuth authenticates a user with OAuth provider info
func (s *AuthenticationService) AuthenticateWithOAuth(ctx context.Context, userInfo UserInfo, tokenInfo TokenInfo, ipAddress, userAgent string) (*AuthenticateResult, error) {
	// Find or create user
	user, isNew, err := s.findOrCreateUser(ctx, userInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to find or create user: %w", err)
	}

	// Update user groups
	if err := s.userRepo.SetUserGroups(ctx, user.UserID, userInfo.Groups); err != nil {
		return nil, fmt.Errorf("failed to update user groups: %w", err)
	}

	// Create session
	session, err := s.createSession(ctx, user, tokenInfo, ipAddress, userAgent)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	// Update last login
	now := time.Now()
	if err := s.userRepo.UpdateLastLogin(ctx, user.UserID, now); err != nil {
		// Non-critical error, log but don't fail
		// In real implementation, we'd log this
	}

	return &AuthenticateResult{
		User:      user,
		Session:   session,
		IsNewUser: isNew,
	}, nil
}

// ValidateSession validates a session by ID
func (s *AuthenticationService) ValidateSession(ctx context.Context, sessionID string) (*domain.Session, error) {
	session, err := s.sessionRepo.FindActiveByID(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("session not found: %w", err)
	}

	if !session.IsValid() {
		return nil, fmt.Errorf("session is invalid or expired")
	}

	// Update activity
	if err := s.sessionRepo.UpdateActivity(ctx, sessionID); err != nil {
		// Non-critical error, continue
	}

	// Load user data
	user, err := s.userRepo.FindByID(ctx, session.UserID)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	if !user.IsActive() {
		return nil, fmt.Errorf("user account is not active")
	}

	session.User = user
	return session, nil
}

// Logout invalidates a user session
func (s *AuthenticationService) Logout(ctx context.Context, sessionID string) error {
	return s.sessionRepo.Invalidate(ctx, sessionID)
}

// LogoutAllSessions invalidates all sessions for a user
func (s *AuthenticationService) LogoutAllSessions(ctx context.Context, userID string) error {
	return s.sessionRepo.InvalidateAllForUser(ctx, userID)
}

// Helper methods

func (s *AuthenticationService) findOrCreateUser(ctx context.Context, userInfo UserInfo) (*domain.User, bool, error) {
	// Try to find existing user
	user, err := s.userRepo.FindByIDOrEmail(ctx, userInfo.Sub, userInfo.Email)
	if err == nil {
		// Update existing user
		user.Email = userInfo.Email
		user.Name = userInfo.Name
		user.FirstName = userInfo.FirstName
		user.LastName = userInfo.LastName
		user.ProfileURL = userInfo.Picture
		user.EmailVerified = userInfo.EmailVerified
		user.Role = s.determineUserRole(userInfo)

		if err := s.userRepo.Update(ctx, user); err != nil {
			return nil, false, err
		}
		return user, false, nil
	}

	// Create new user
	role := s.determineUserRole(userInfo)
	newUser := &domain.User{
		UserID:        userInfo.Sub,
		Email:         userInfo.Email,
		Name:          userInfo.Name,
		FirstName:     userInfo.FirstName,
		LastName:      userInfo.LastName,
		Role:          role,
		Status:        domain.StatusActive,
		ProfileURL:    userInfo.Picture,
		EmailVerified: userInfo.EmailVerified,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	if err := s.userRepo.Create(ctx, newUser); err != nil {
		return nil, false, err
	}

	return newUser, true, nil
}

func (s *AuthenticationService) createSession(ctx context.Context, user *domain.User, tokenInfo TokenInfo, ipAddress, userAgent string) (*domain.Session, error) {
	expiresAt := time.Now().Add(time.Duration(tokenInfo.ExpiresIn) * time.Second)
	if tokenInfo.ExpiresIn == 0 {
		expiresAt = time.Now().Add(24 * time.Hour) // Default 24 hours
	}

	sessionID, err := generateSessionID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate session ID: %w", err)
	}

	session := &domain.Session{
		SessionID:    sessionID,
		UserID:       user.UserID,
		User:         user,
		AccessToken:  tokenInfo.AccessToken,
		RefreshToken: tokenInfo.RefreshToken,
		IDToken:      tokenInfo.IDToken,
		ExpiresAt:    expiresAt,
		IsActive:     true,
		IPAddress:    ipAddress,
		UserAgent:    userAgent,
		LastActivity: time.Now(),
		CreatedAt:    time.Now(),
	}

	if err := s.sessionRepo.Create(ctx, session); err != nil {
		return nil, err
	}

	return session, nil
}

func (s *AuthenticationService) determineUserRole(userInfo UserInfo) domain.UserRole {
	// Check for admin groups
	for _, group := range userInfo.Groups {
		if group == "admin" || group == "/admin" || stringSliceContains(viper.GetStringSlice("auth.oauth.adminGroups"), group) {
			return domain.RoleAdmin
		}
	}

	// Default to regular user
	return domain.RoleUser
}

// generateSessionID generates a cryptographically secure session ID
func generateSessionID() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// stringSliceContains checks if a string is present in a slice
func stringSliceContains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
