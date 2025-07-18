package projects_test

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/playwright-community/playwright-go"
)

var (
	pw              *playwright.Playwright
	browserType     playwright.BrowserType
	browser         playwright.Browser
	baseURL         string
	username        string
	password        string
	teamName        string
	headless        bool
	recordVideo     bool
	contextOptions  playwright.BrowserNewContextOptions
	launchOptions   playwright.BrowserTypeLaunchOptions
	defaultTimeout  = 30 * time.Second
	slowMo          float64
	videoDir        = "videos"
)

func init() {
	// Parse command line flags
	flag.StringVar(&baseURL, "base-url", getEnvOrDefault("FERN_BASE_URL", "http://localhost:8080"), "Base URL for the application")
	flag.StringVar(&username, "username", getEnvOrDefault("FERN_USERNAME", "admin@fern.com"), "Username for authentication")
	flag.StringVar(&password, "password", getEnvOrDefault("FERN_PASSWORD", "test123"), "Password for authentication")
	flag.StringVar(&teamName, "team", getEnvOrDefault("FERN_TEAM_NAME", "fern"), "Team name")
	flag.BoolVar(&headless, "headless", getEnvOrDefault("FERN_HEADLESS", "true") == "true", "Run browser in headless mode")
	flag.BoolVar(&recordVideo, "record", getEnvOrDefault("FERN_RECORD_VIDEO", "false") == "true", "Record videos of test runs")
	flag.Float64Var(&slowMo, "slowmo", 0, "Slow down browser operations by specified milliseconds")
}

func TestProjects(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Projects Suite")
}

var _ = BeforeSuite(func() {
	var err error
	
	// Initialize Playwright
	pw, err = playwright.Run()
	Expect(err).NotTo(HaveOccurred())
	
	// Get browser type
	browserType = pw.Chromium
	
	// Set launch options
	launchOptions = playwright.BrowserTypeLaunchOptions{
		Headless: &headless,
		SlowMo:   &slowMo,
	}
	
	// Set context options
	contextOptions = playwright.BrowserNewContextOptions{
		BaseURL: playwright.String(baseURL),
	}
	
	log.Printf("Test configuration:")
	log.Printf("  Base URL: %s", baseURL)
	log.Printf("  Username: %s", username)
	log.Printf("  Team: %s", teamName)
	log.Printf("  Headless: %v", headless)
	log.Printf("  Record Video: %v", recordVideo)
	log.Printf("  Slow Motion: %v ms", slowMo)
})

var _ = AfterSuite(func() {
	if pw != nil {
		err := pw.Stop()
		Expect(err).NotTo(HaveOccurred())
	}
})

// Helper functions

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func CreateBrowser() playwright.Browser {
	browser, err := browserType.Launch(launchOptions)
	Expect(err).NotTo(HaveOccurred())
	return browser
}

func SaveVideoOnFailure(ctx playwright.BrowserContext, report SpecReport) {
	if recordVideo && report.Failed() {
		// Video will be saved automatically by Playwright
		// Log the video location for debugging
		testName := report.FullText()
		log.Printf("Test failed: %s", testName)
		log.Printf("Video saved in: %s/", videoDir)
		
		// Optionally, rename the video file to match the test name
		pages := ctx.Pages()
		if len(pages) > 0 {
			video := pages[0].Video()
			if video != nil {
				videoPath, err := video.Path()
				if err == nil && videoPath != "" {
					// Create a descriptive filename
					timestamp := time.Now().Format("20060102_150405")
					newPath := fmt.Sprintf("%s/%s_%s.webm", videoDir, timestamp, sanitizeTestName(testName))
					os.Rename(videoPath, newPath)
					log.Printf("Video renamed to: %s", newPath)
				}
			}
		}
	}
}

func sanitizeTestName(name string) string {
	// Replace spaces and special characters with underscores
	replacer := strings.NewReplacer(
		" ", "_",
		":", "",
		"/", "_",
		"\\", "_",
		"<", "",
		">", "",
		"|", "_",
		"?", "",
		"*", "",
	)
	return replacer.Replace(name)
}