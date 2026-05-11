package performance_test

import (
	"flag"
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/playwright-community/playwright-go"
	
	"github.com/guidewire-oss/fern-platform/acceptance/helpers"
)

func TestPerformance(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Performance Test Suite")
}

var (
	// Configuration
	baseURL     string
	username    string
	password    string
	authEnabled bool
	headless    bool
	
	// Playwright objects
	pw      *playwright.Playwright
	browser playwright.Browser
	
	// Authentication cookies (only used if auth is enabled)
	savedCookies []playwright.OptionalCookie
)

func init() {
	flag.StringVar(&baseURL, "base-url", getEnvOrDefault("FERN_BASE_URL", "http://fern-platform.local:8080"), "Base URL for Fern Platform")
	flag.StringVar(&username, "username", getEnvOrDefault("FERN_USERNAME", "fern-user@fern.com"), "Username for authentication")
	flag.StringVar(&password, "password", getEnvOrDefault("FERN_PASSWORD", "test123"), "Password for authentication")
	flag.BoolVar(&authEnabled, "auth-enabled", getEnvOrDefault("AUTH_ENABLED", "false") == "true", "Whether auth is enabled")
	flag.BoolVar(&headless, "headless", getEnvOrDefault("FERN_HEADLESS", "true") == "true", "Run browser in headless mode")
}

var _ = BeforeSuite(func() {
	var err error

	// Initialize Playwright
	pw, err = playwright.Run()
	Expect(err).NotTo(HaveOccurred())

	// Launch browser with configurable headless mode
	browser, err = pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(headless),
	})
	Expect(err).NotTo(HaveOccurred())

	// If auth is enabled, login once and save cookies
	if authEnabled {
		GinkgoWriter.Println("Auth enabled - logging in...")
		ctx, err := browser.NewContext()
		Expect(err).NotTo(HaveOccurred())
		
		page, err := ctx.NewPage()
		Expect(err).NotTo(HaveOccurred())
		
		authHelper := helpers.NewLoginHelper(page, baseURL, username, password)
		authHelper.Login()
		
		// Save cookies for reuse in tests
		cookies, err := ctx.Cookies()
		Expect(err).NotTo(HaveOccurred())
		
		savedCookies = make([]playwright.OptionalCookie, len(cookies))
		for i, cookie := range cookies {
			domain := cookie.Domain
			path := cookie.Path
			expires := cookie.Expires
			httpOnly := cookie.HttpOnly
			secure := cookie.Secure
			
			savedCookies[i] = playwright.OptionalCookie{
				Name:     cookie.Name,
				Value:    cookie.Value,
				Domain:   &domain,
				Path:     &path,
				Expires:  &expires,
				HttpOnly: &httpOnly,
				Secure:   &secure,
				SameSite: cookie.SameSite,
			}
		}
		
		ctx.Close()
		GinkgoWriter.Println("Login successful - cookies saved")
	} else {
		GinkgoWriter.Println("Auth disabled - skipping login")
	}
})

var _ = AfterSuite(func() {
	if browser != nil {
		browser.Close()
	}
	if pw != nil {
		pw.Stop()
	}
})

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
