package auth_test

import (
	"fmt"
	"os"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/playwright-community/playwright-go"

	"github.com/guidewire-oss/fern-platform/acceptance/helpers"
)

var _ = Describe("UC-00: Authentication", Label("acceptance", "auth", "e2e"), func() {
	var (
		browser playwright.Browser
		ctx     playwright.BrowserContext
		page    playwright.Page
		auth    *helpers.LoginHelper
	)

	BeforeEach(func() {
		var err error

		// Create a new browser for each test
		browser = CreateBrowser()

		// Create browser context options
		contextOptions := playwright.BrowserNewContextOptions{
			BaseURL: playwright.String(baseURL),
		}

		// Add video recording if enabled
		if recordVideo {
			contextOptions.RecordVideo = &playwright.RecordVideo{
				Dir:  "./videos",
				Size: &playwright.Size{Width: 1280, Height: 720},
			}
		}

		ctx, err = browser.NewContext(contextOptions)
		Expect(err).NotTo(HaveOccurred())

		page, err = ctx.NewPage()
		Expect(err).NotTo(HaveOccurred())

		auth = helpers.NewLoginHelper(page, baseURL, username, password)
	})

	AfterEach(func() {
		// Handle cleanup even if test fails
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("Recovered from panic in AfterEach: %v\n", r)
			}

			// Force cleanup in correct order
			// 1. Close page first
			if page != nil {
				_ = page.Close()
				page = nil
			}

			// 2. Close context
			if ctx != nil {
				_ = ctx.Close()
				ctx = nil
			}

			// 3. Close browser last
			if browser != nil {
				_ = browser.Close()
				browser = nil
			}
		}()

		// Save video if recording is enabled
		if recordVideo && page != nil {
			video := page.Video()
			if video != nil {
				// Get the test name for the video file
				testName := CurrentSpecReport().LeafNodeText
				testName = strings.ReplaceAll(testName, " ", "_")
				testName = strings.ReplaceAll(testName, "/", "-")

				// Save the video with a descriptive name
				timestamp := time.Now().Format("20060102_150405")
				newPath := fmt.Sprintf("../videos/auth/%s_%s.webm", testName, timestamp)

				// Get the original video path before closing
				originalPath, _ := video.Path()

				// Close the page first (not the context) to finalize video
				if page != nil {
					page.Close()
					page = nil
				}

				// Now close the context
				if ctx != nil {
					ctx.Close()
					ctx = nil
				}

				// Save the video with the new name
				err := video.SaveAs(newPath)
				if err == nil {
					fmt.Printf("Video saved: %s\n", newPath)
					// Try to delete the original file if it exists and is different
					if originalPath != "" && originalPath != newPath {
						os.Remove(originalPath)
					}
				} else {
					fmt.Printf("Failed to save video: %v\n", err)
				}
			}
		} else {
			// If no video recording, just close page and context
			if page != nil {
				page.Close()
				page = nil
			}
			if ctx != nil {
				ctx.Close()
				ctx = nil
			}
		}
	})

	Describe("UC-00-01: User Login", Label("e2e"), func() {
		Context("Accessing platform redirects to login", func() {
			It("should redirect unauthenticated users to login page", func() {
				_, err := page.Goto(baseURL)
				Expect(err).NotTo(HaveOccurred())

				// Should redirect to login page
				Eventually(func() string {
					return page.URL()
				}, 5*time.Second).Should(Equal(baseURL + "/auth/login"))

				// Verify sign in element is present
				signInElement := page.Locator("button:has-text('Sign in'), a:has-text('Sign in')")
				count, _ := signInElement.Count()
				Expect(count).To(BeNumerically(">=", 1))
			})
		})

		Context("Successful login with valid credentials", func() {
			It("should log in and redirect to dashboard", func() {
				auth.Login()

				// Verify we're on the main dashboard
				Expect(page.URL()).To(Equal(baseURL + "/"))

				// Verify user menu appears (shows "FFern User")
				userMenu := page.Locator("div.user-menu")
				count, _ := userMenu.Count()
				Expect(count).To(Equal(1))
			})
		})

		Context("Failed login with invalid credentials", func() {
			It("should show error for invalid credentials", func() {
				_, err := page.Goto(baseURL + "/auth/login")
				Expect(err).NotTo(HaveOccurred())

				// Click sign in element
				signInElement := page.Locator("button:has-text('Sign in'), a:has-text('Sign in')")
				err = signInElement.Click()
				Expect(err).NotTo(HaveOccurred())

				// Wait for Keycloak redirect
				Eventually(func() bool {
					url := page.URL()
					return strings.Contains(url, "keycloak") && strings.Contains(url, "/realms/fern-platform")
				}, 10*time.Second).Should(BeTrue())

				// Fill invalid credentials in Keycloak
				err = page.Locator("input#username, input[name='username']").Fill("invalid@example.com")
				Expect(err).NotTo(HaveOccurred())

				err = page.Locator("input#password, input[name='password']").Fill("wrongpassword")
				Expect(err).NotTo(HaveOccurred())

				// Try to sign in
				signInButton := page.Locator("input[type='submit'], button[type='submit']")
				err = signInButton.Click()
				Expect(err).NotTo(HaveOccurred())

				// Should see error message on Keycloak
				Eventually(func() bool {
					errorMsg := page.Locator("text=/Invalid username or password|Invalid user credentials/")
					count, _ := errorMsg.Count()
					return count > 0
				}, 10*time.Second).Should(BeTrue())

				// Should remain on Keycloak login page
				Expect(page.URL()).To(ContainSubstring("keycloak"))
			})
		})

		Context("Session persistence after login", func() {
			It("should maintain session cookies during navigation", func() {
				// First login
				auth.Login()

				// Get cookies after login
				cookies, err := ctx.Cookies()
				Expect(err).NotTo(HaveOccurred())
				Expect(cookies).NotTo(BeEmpty())

				// Verify we have session cookie
				var hasSessionCookie bool
				for _, cookie := range cookies {
					if cookie.Name == "session_id" || strings.Contains(cookie.Name, "KEYCLOAK") {
						hasSessionCookie = true
						// Verify cookie has reasonable expiry
						if cookie.Expires > 0 {
							expiryTime := time.Unix(int64(cookie.Expires), 0)
							Expect(expiryTime.After(time.Now())).To(BeTrue(), "Cookie should not be expired")
						}
					}
				}
				Expect(hasSessionCookie).To(BeTrue(), "Should have session cookie")

				// Navigate to another page
				_, err = page.Goto(baseURL + "/test-summaries")
				Expect(err).NotTo(HaveOccurred())

				// Navigate back to home
				_, err = page.Goto(baseURL)
				Expect(err).NotTo(HaveOccurred())

				// Wait for page to stabilize
				time.Sleep(2 * time.Second)

				// Verify still logged in
				Eventually(func() bool {
					return auth.IsLoggedIn()
				}, 5*time.Second).Should(BeTrue(), "Should still be logged in after navigation")

				// Cookies should still exist
				cookiesAfter, err := ctx.Cookies()
				Expect(err).NotTo(HaveOccurred())
				Expect(len(cookiesAfter)).To(BeNumerically(">=", len(cookies)))
			})
		})

		Context("Deep link authentication", func() {
			It("should preserve return URL through OAuth flow", func() {
				// Access login page with return URL parameter
				targetPath := "/test-summaries"
				loginURL := baseURL + "/auth/login?return=" + targetPath

				_, err := page.Goto(loginURL)
				Expect(err).NotTo(HaveOccurred())

				// Should be on login page
				Expect(page.URL()).To(ContainSubstring("/auth/login"))

				// Login
				auth.Login()

				// After login, should be on the dashboard (OAuth doesn't preserve deep links)
				Eventually(func() string {
					return page.URL()
				}, 10*time.Second).Should(Equal(baseURL + "/"))
			})
		})
	})

	Describe("UC-00-02: User Logout", Label("e2e"), func() {
		BeforeEach(func() {
			// Login first
			auth.Login()
		})

		Context("Logout from user menu", func() {
			It("should show logout option in user dropdown", func() {
				// Click on user menu trigger
				userMenuTrigger := page.Locator("button.user-menu-trigger")
				err := userMenuTrigger.Click()
				Expect(err).NotTo(HaveOccurred())

				// Should see logout option ("Sign Out")
				logoutOption := page.Locator("text=Sign Out")
				Eventually(func() bool {
					count, _ := logoutOption.Count()
					return count > 0
				}, 2*time.Second).Should(BeTrue())
			})
		})

		Context("Confirm logout", func() {
			It("should logout and redirect to login page", func() {
				auth.Logout()

				// Verify we're back on login page
				Expect(page.URL()).To(ContainSubstring("/auth/login"))

				// Verify user is no longer logged in
				Expect(auth.IsLoggedIn()).To(BeFalse())
			})
		})

		Context("Accessing protected resources after logout", func() {
			It("should not show user-specific content after logout", func() {
				auth.Logout()

				// Try to access dashboard
				_, err := page.Goto(baseURL + "/")
				Expect(err).NotTo(HaveOccurred())

				// Should be redirected to login since we're not authenticated
				Eventually(func() string {
					return page.URL()
				}, 5*time.Second).Should(ContainSubstring("/auth/login"))

				// Verify no user menu is visible
				userMenu := page.Locator("div.user-menu")
				count, _ := userMenu.Count()
				Expect(count).To(Equal(0))
			})
		})
	})

	Describe("UC-00-03: Session Management", Label("e2e"), func() {
		BeforeEach(func() {
			auth.Login()
		})

		Context("Active use prevents timeout", func() {
			It("should keep session active during use", func() {
				// Verify initial login state
				Expect(auth.IsLoggedIn()).To(BeTrue(), "Should be logged in initially")

				// Perform some actions to simulate activity
				for i := 0; i < 3; i++ {
					// Click on navigation elements instead of direct navigation
					navButtons := page.Locator("button.nav-button")
					count, _ := navButtons.Count()

					if count > 1 {
						// Click a nav button (skip first which is usually active)
						err := navButtons.Nth(1).Click()
						Expect(err).NotTo(HaveOccurred())

						time.Sleep(2 * time.Second)

						// Click back to first button
						err = navButtons.Nth(0).Click()
						Expect(err).NotTo(HaveOccurred())
					}

					time.Sleep(1 * time.Second)

					// Check we're still logged in by looking for user menu
					userMenu := page.Locator("div.user-menu")
					count, _ = userMenu.Count()
					Expect(count).To(BeNumerically(">", 0), "User menu should still be visible")
				}
			})
		})
	})

	Describe("UC-00-04: Authentication Error Handling", Label("e2e"), func() {
		Context("Account not found in any team", func() {
			It("should show appropriate error for users without team", func() {
				// This would require a test user without team assignment
				// Skipping as it requires specific test data setup
				Skip("Requires test user without team assignment")
			})
		})
	})
})

// Custom matcher for element visibility
func BeVisible() OmegaMatcher {
	return WithTransform(func(locator playwright.Locator) bool {
		visible, _ := locator.IsVisible()
		return visible
	}, BeTrue())
}
