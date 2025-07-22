package testruns_test

import (
	"flag"
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/playwright-community/playwright-go"

	"github.com/guidewire-oss/fern-platform/acceptance/helpers"
)

var (
	// Configuration flags
	baseURL     string
	headless    bool
	slowMo      float64
	teamName    string
	username    string
	password    string
	recordVideo bool

	// Playwright objects
	pw      *playwright.Playwright
	browser playwright.Browser

	// Shared helpers
	authHelper *helpers.LoginHelper
)

func init() {
	flag.StringVar(&baseURL, "base-url", getEnvOrDefault("FERN_BASE_URL", "http://fern-platform.local:8080"), "Base URL for Fern Platform")
	flag.BoolVar(&headless, "headless", getEnvOrDefault("FERN_HEADLESS", "true") == "true", "Run browser in headless mode")
	flag.Float64Var(&slowMo, "slow-mo", 0, "Slow motion delay in milliseconds")
	flag.StringVar(&teamName, "team-name", getEnvOrDefault("FERN_TEAM_NAME", "fern"), "Team name for test user")
	flag.StringVar(&username, "username", getEnvOrDefault("FERN_USERNAME", "fern-user@fern.com"), "Username for authentication")
	flag.StringVar(&password, "password", getEnvOrDefault("FERN_PASSWORD", "test123"), "Password for authentication")
	flag.BoolVar(&recordVideo, "record-video", getEnvOrDefault("FERN_RECORD_VIDEO", "false") == "true", "Record videos of test runs")
}

func TestTestRuns(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Test Runs Test Suite")
}

var _ = BeforeSuite(func() {
	var err error

	// Install playwright browsers if needed
	err = playwright.Install()
	Expect(err).NotTo(HaveOccurred())

	// Initialize playwright
	pw, err = playwright.Run()
	Expect(err).NotTo(HaveOccurred())

	// Launch browser
	browser, err = pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(headless),
		SlowMo:   playwright.Float(slowMo),
	})
	Expect(err).NotTo(HaveOccurred())

	// Login once for the entire suite
	ctx, page := createContextAndPage()
	authHelper = helpers.NewLoginHelper(page, baseURL, username, password)
	authHelper.Login()

	// Save cookies for reuse
	cookies, err := ctx.Cookies()
	Expect(err).NotTo(HaveOccurred())
	// Convert cookies to OptionalCookie format
	optCookies := make([]playwright.OptionalCookie, len(cookies))
	for i, cookie := range cookies {
		// Create local copies to avoid pointer aliasing issues
		domain := cookie.Domain
		path := cookie.Path
		expires := cookie.Expires
		httpOnly := cookie.HttpOnly
		secure := cookie.Secure
		
		optCookies[i] = playwright.OptionalCookie{
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
	saveCookies(optCookies)

	ctx.Close()
})

var _ = AfterSuite(func() {
	if browser != nil {
		err := browser.Close()
		Expect(err).NotTo(HaveOccurred())
	}

	if pw != nil {
		err := pw.Stop()
		Expect(err).NotTo(HaveOccurred())
	}
})

// Helper function to get environment variable with default
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// Helper function to create a new browser context and page
func createContextAndPage() (playwright.BrowserContext, playwright.Page) {
	options := playwright.BrowserNewContextOptions{
		BaseURL: playwright.String(baseURL),
	}

	if recordVideo {
		options.RecordVideo = &playwright.RecordVideo{
			Dir:  "./videos",
			Size: &playwright.Size{Width: 1280, Height: 720},
		}
	}

	ctx, err := browser.NewContext(options)
	Expect(err).NotTo(HaveOccurred())

	p, err := ctx.NewPage()
	Expect(err).NotTo(HaveOccurred())

	return ctx, p
}

// Helper function to create authenticated context
func createAuthenticatedContext() (playwright.BrowserContext, playwright.Page) {
	ctx, page := createContextAndPage()

	// Add saved cookies
	if cookies := getSavedCookies(); cookies != nil {
		err := ctx.AddCookies(cookies)
		Expect(err).NotTo(HaveOccurred())
	}

	return ctx, page
}

var savedCookies []playwright.OptionalCookie

func saveCookies(cookies []playwright.OptionalCookie) {
	savedCookies = cookies
}

func getSavedCookies() []playwright.OptionalCookie {
	return savedCookies
}
