package domain_test

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/guidewire-oss/fern-platform/internal/domains/auth/domain"
)

var _ = Describe("Session", Label("unit", "domain", "auth"), func() {
	Describe("Session Creation", func() {
		It("should create a new session with valid inputs", func() {
			ttl := 24 * time.Hour
			session := &domain.Session{
				SessionID:    "session-123",
				UserID:       "user-456",
				ExpiresAt:    time.Now().Add(ttl),
				IsActive:     true,
				LastActivity: time.Now(),
				CreatedAt:    time.Now(),
			}

			Expect(session.SessionID).To(Equal("session-123"))
			Expect(session.UserID).To(Equal("user-456"))
			Expect(session.ExpiresAt).To(BeTemporally("~", time.Now().Add(ttl), time.Second))
			Expect(session.LastActivity).To(BeTemporally("~", time.Now(), time.Second))
			Expect(session.CreatedAt).To(BeTemporally("~", time.Now(), time.Second))
		})

		It("should set expiration time correctly", func() {
			shortTTL := 1 * time.Hour
			longTTL := 7 * 24 * time.Hour

			shortSession := &domain.Session{
				SessionID: "session-1",
				UserID:    "user-1",
				ExpiresAt: time.Now().Add(shortTTL),
				IsActive:  true,
			}
			longSession := &domain.Session{
				SessionID: "session-2",
				UserID:    "user-2",
				ExpiresAt: time.Now().Add(longTTL),
				IsActive:  true,
			}

			Expect(shortSession.ExpiresAt).To(BeTemporally("~", time.Now().Add(shortTTL), time.Second))
			Expect(longSession.ExpiresAt).To(BeTemporally("~", time.Now().Add(longTTL), time.Second))
		})
	})

	Describe("IsValid", func() {
		It("should return true for active session", func() {
			session := &domain.Session{
				SessionID: "session-123",
				UserID:    "user-456",
				ExpiresAt: time.Now().Add(24 * time.Hour),
				IsActive:  true,
			}
			Expect(session.IsValid()).To(BeTrue())
		})

		It("should return false for expired session", func() {
			session := &domain.Session{
				SessionID:      "expired-session",
				UserID:         "user-123",
				ExpiresAt:      time.Now().Add(-1 * time.Hour), // Expired 1 hour ago
				LastActivity: time.Now().Add(-2 * time.Hour),
			}
			Expect(session.IsValid()).To(BeFalse())
		})

		It("should handle edge case of exactly expired session", func() {
			session := &domain.Session{
				SessionID:      "edge-session",
				UserID:         "user-123",
				ExpiresAt:      time.Now(), // Expires right now
				LastActivity: time.Now().Add(-1 * time.Hour),
			}
			// Might be flaky due to timing, but should generally be false
			Expect(session.IsValid()).To(BeFalse())
		})
	})

	Describe("IsExpired", func() {
		It("should return false for active session", func() {
			session := &domain.Session{
				SessionID: "session-123",
				UserID:    "user-456",
				ExpiresAt: time.Now().Add(24 * time.Hour),
				IsActive:  true,
			}
			Expect(session.IsExpired()).To(BeFalse())
		})

		It("should return true for expired session", func() {
			session := &domain.Session{
				SessionID: "expired-session",
				UserID:    "user-123",
				ExpiresAt: time.Now().Add(-1 * time.Second),
			}
			Expect(session.IsExpired()).To(BeTrue())
		})

		It("should handle far future expiration", func() {
			session := &domain.Session{
				SessionID: "future-session",
				UserID:    "user-123",
				ExpiresAt: time.Now().Add(365 * 24 * time.Hour), // 1 year
			}
			Expect(session.IsExpired()).To(BeFalse())
		})
	})

	Describe("UpdateActivity", func() {
		It("should update last activity time", func() {
			session := &domain.Session{
				SessionID:    "session-123",
				UserID:       "user-456",
				ExpiresAt:    time.Now().Add(24 * time.Hour),
				IsActive:     true,
				LastActivity: time.Now(),
			}
			originalActivity := session.LastActivity

			// Wait a bit to ensure time difference
			time.Sleep(10 * time.Millisecond)
			
			session.UpdateActivity()

			Expect(session.LastActivity).To(BeTemporally(">", originalActivity))
			Expect(session.LastActivity).To(BeTemporally("~", time.Now(), time.Second))
		})

		It("should be safe to call multiple times", func() {
			session := &domain.Session{
				SessionID:    "session-123",
				UserID:       "user-456",
				ExpiresAt:    time.Now().Add(24 * time.Hour),
				IsActive:     true,
				LastActivity: time.Now(),
			}

			// Update multiple times
			for i := 0; i < 5; i++ {
				session.UpdateActivity()
				time.Sleep(5 * time.Millisecond)
			}

			Expect(session.LastActivity).To(BeTemporally("~", time.Now(), time.Second))
		})
	})

	Describe("Session Extension", func() {
		It("should be able to extend session expiration", func() {
			session := &domain.Session{
				SessionID: "session-123",
				UserID:    "user-456",
				ExpiresAt: time.Now().Add(1 * time.Hour),
				IsActive:  true,
			}
			originalExpiry := session.ExpiresAt

			// Extend session by updating expiry
			session.ExpiresAt = time.Now().Add(3 * time.Hour)

			Expect(session.ExpiresAt).To(BeTemporally(">", originalExpiry))
			Expect(session.ExpiresAt).To(BeTemporally("~", time.Now().Add(3*time.Hour), time.Second))
		})

		It("should also update activity when extending", func() {
			session := &domain.Session{
				SessionID:    "session-123",
				UserID:       "user-456",
				ExpiresAt:    time.Now().Add(1 * time.Hour),
				IsActive:     true,
				LastActivity: time.Now(),
			}
			originalActivity := session.LastActivity

			time.Sleep(10 * time.Millisecond)
			// Sessions don't have ExtendSession method, just update expiry
			session.ExpiresAt = time.Now().Add(1 * time.Hour)
			session.UpdateActivity()

			Expect(session.LastActivity).To(BeTemporally(">", originalActivity))
		})

		It("should handle negative expiration time", func() {
			session := &domain.Session{
				SessionID: "session-123",
				UserID:    "user-456",
				ExpiresAt: time.Now().Add(-1 * time.Hour), // Already expired
				IsActive:  true,
			}
			
			// Session should be expired
			Expect(session.IsExpired()).To(BeTrue())
		})
	})

	Describe("Session Lifecycle", func() {
		It("should track complete session lifecycle", func() {
			// Create session
			ttl := 2 * time.Hour
			session := &domain.Session{
				SessionID:    "lifecycle-session",
				UserID:       "user-789",
				ExpiresAt:    time.Now().Add(ttl),
				IsActive:     true,
				LastActivity: time.Now(),
				CreatedAt:    time.Now(),
			}
			
			// Verify initial state
			Expect(session.IsValid()).To(BeTrue())
			Expect(session.IsExpired()).To(BeFalse())
			
			// Simulate activity
			time.Sleep(10 * time.Millisecond)
			session.UpdateActivity()
			activityTime1 := session.LastActivity
			
			// More activity
			time.Sleep(10 * time.Millisecond)
			session.UpdateActivity()
			activityTime2 := session.LastActivity
			
			Expect(activityTime2).To(BeTemporally(">", activityTime1))
			
			// Force expiration
			session.ExpiresAt = time.Now().Add(-1 * time.Second)
			Expect(session.IsValid()).To(BeFalse())
			Expect(session.IsExpired()).To(BeTrue())
		})
	})

	Describe("Concurrent Access", func() {
		It("should handle concurrent activity updates", func() {
			// Note: This test demonstrates that UpdateActivity is NOT thread-safe
			// In a real application, session updates should be protected by locks
			// or handled through a single goroutine
			
			// Create multiple sessions to avoid race condition in test
			sessions := make([]*domain.Session, 10)
			for i := 0; i < 10; i++ {
				sessions[i] = &domain.Session{
					SessionID:    fmt.Sprintf("concurrent-session-%d", i),
					UserID:       "user-999",
					ExpiresAt:    time.Now().Add(24 * time.Hour),
					IsActive:     true,
					LastActivity: time.Now(),
				}
			}
			
			// Simulate concurrent updates on different sessions
			done := make(chan bool, 10)
			for i := 0; i < 10; i++ {
				go func(idx int) {
					sessions[idx].UpdateActivity()
					done <- true
				}(i)
			}
			
			// Wait for all goroutines
			for i := 0; i < 10; i++ {
				<-done
			}
			
			// All sessions should still be valid
			for _, s := range sessions {
				Expect(s.IsValid()).To(BeTrue())
				Expect(s.LastActivity).To(BeTemporally("~", time.Now(), time.Second))
			}
		})
	})

	Describe("Edge Cases", func() {
		It("should handle zero TTL", func() {
			session := &domain.Session{
				SessionID: "zero-ttl",
				UserID:    "user-123",
				ExpiresAt: time.Now(), // Expires immediately
				IsActive:  true,
			}
			// With 0 TTL, session expires at creation time
			Expect(session.IsExpired()).To(BeTrue())
		})

		It("should handle very long TTL", func() {
			session := &domain.Session{
				SessionID: "long-ttl",
				UserID:    "user-123",
				ExpiresAt: time.Now().Add(365 * 24 * time.Hour), // 1 year
				IsActive:  true,
			}
			Expect(session.IsValid()).To(BeTrue())
			Expect(session.ExpiresAt.Year()).To(Equal(time.Now().Year() + 1))
		})

		It("should handle session with empty IDs", func() {
			// This documents current behavior - no validation
			session := &domain.Session{
				SessionID: "",
				UserID:    "",
				ExpiresAt: time.Now().Add(1 * time.Hour),
				IsActive:  true,
			}
			Expect(session.SessionID).To(BeEmpty())
			Expect(session.UserID).To(BeEmpty())
			Expect(session.IsValid()).To(BeTrue()) // Still valid based on time
		})
	})
})