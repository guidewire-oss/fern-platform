package pmconnectors_test

import (
	"fmt"
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/playwright-community/playwright-go"
)

var (
	pw          *playwright.Playwright
	browserType playwright.BrowserType
	baseURL     string
	username    string
	password    string
	recordVideo bool
	headless    bool
)

func TestPMConnectors(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "PM Connectors Acceptance Test Suite")
}

var _ = BeforeSuite(func() {
	var err error

	// Get environment variables
	baseURL = os.Getenv("FERN_BASE_URL")
	if baseURL == "" {
		baseURL = "http://fern-platform.local:8080"
	}

	username = os.Getenv("FERN_USERNAME")
	if username == "" {
		username = "admin@fern.com"
	}

	password = os.Getenv("FERN_PASSWORD")
	if password == "" {
		password = "test123"
	}

	recordVideo = os.Getenv("FERN_RECORD_VIDEO") == "true"
	headless = os.Getenv("FERN_HEADLESS") != "false"

	fmt.Printf("Running PM Connectors tests against: %s\n", baseURL)
	fmt.Printf("Using username: %s\n", username)
	fmt.Printf("Record video: %v\n", recordVideo)
	fmt.Printf("Headless: %v\n", headless)

	// Install playwright
	err = playwright.Install()
	Expect(err).NotTo(HaveOccurred())

	// Start playwright
	pw, err = playwright.Run()
	Expect(err).NotTo(HaveOccurred())

	// Get browser type
	browserName := os.Getenv("BROWSER")
	switch browserName {
	case "firefox":
		browserType = pw.Firefox
	case "webkit":
		browserType = pw.WebKit
	default:
		browserType = pw.Chromium
	}

	// Create video directory if needed
	if recordVideo {
		os.MkdirAll("../videos/pmconnectors", 0755)
	}
})

var _ = AfterSuite(func() {
	if pw != nil {
		err := pw.Stop()
		Expect(err).NotTo(HaveOccurred())
	}
})

// CreateBrowser creates a new browser instance
func CreateBrowser() playwright.Browser {
	browser, err := browserType.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(headless),
		SlowMo:   playwright.Float(100),
		Args: []string{
			"--disable-blink-features=AutomationControlled",
			"--no-sandbox",
			"--disable-setuid-sandbox",
			"--disable-dev-shm-usage",
			"--disable-gpu",
			"--disable-web-security",
			"--disable-features=IsolateOrigins,site-per-process",
		},
	})
	Expect(err).NotTo(HaveOccurred())

	// Default timeout will be set at context level

	return browser
}
