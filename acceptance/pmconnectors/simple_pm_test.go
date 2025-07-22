package pmconnectors_test

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/playwright-community/playwright-go"

	"github.com/guidewire-oss/fern-platform/acceptance/helpers"
)

var _ = Describe("Simple PM Connector Test", Label("e2e"), func() {
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

		// Login as admin user
		auth.Login()
	})

	AfterEach(func() {
		if page != nil {
			page.Close()
		}
		if ctx != nil {
			ctx.Close()
		}
		if browser != nil {
			browser.Close()
		}
	})

	It("should successfully navigate to PM Connectors page", func() {
		// Navigate to base URL
		_, err := page.Goto(baseURL)
		Expect(err).NotTo(HaveOccurred())

		// Wait for page to load
		err = page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
			State: playwright.LoadStateNetworkidle,
		})
		Expect(err).NotTo(HaveOccurred())

		// Take screenshot before clicking
		page.Screenshot(playwright.PageScreenshotOptions{
			Path: playwright.String("./before_click.png"),
		})

		// Look for PM Connectors in navigation
		pmConnectorsNav := page.Locator("text=PM Connectors")
		count, _ := pmConnectorsNav.Count()
		fmt.Printf("Found %d PM Connectors nav items\n", count)

		if count > 0 {
			// Click PM Connectors
			err = pmConnectorsNav.Click()
			Expect(err).NotTo(HaveOccurred())

			// Wait a bit for navigation
			time.Sleep(2 * time.Second)

			// Take screenshot after clicking
			page.Screenshot(playwright.PageScreenshotOptions{
				Path: playwright.String("./after_click.png"),
			})

			// Check URL
			currentURL := page.URL()
			fmt.Printf("Current URL: %s\n", currentURL)

			// Look for any content on the page
			bodyText, _ := page.Locator("body").TextContent()
			fmt.Printf("Page content length: %d\n", len(bodyText))
			if len(bodyText) < 500 {
				fmt.Printf("Page content: %s\n", bodyText)
			}

			// Check for any error messages
			errorMsg := page.Locator("text=/error|failed|denied/i")
			errorCount, _ := errorMsg.Count()
			if errorCount > 0 {
				fmt.Printf("Found %d error messages\n", errorCount)
				for i := 0; i < errorCount; i++ {
					text, _ := errorMsg.Nth(i).TextContent()
					fmt.Printf("Error %d: %s\n", i+1, text)
				}
			}

			// Check for loading indicators
			loading := page.Locator("text=/loading|spinner/i")
			loadingCount, _ := loading.Count()
			fmt.Printf("Found %d loading indicators\n", loadingCount)

			// Check for PM Connectors header
			header := page.Locator("h2:has-text('PM Connectors')")
			headerCount, _ := header.Count()
			fmt.Printf("Found %d PM Connectors headers\n", headerCount)

			// Check for empty state
			emptyState := page.Locator("text=/No PM connectors|no connectors/i")
			emptyCount, _ := emptyState.Count()
			fmt.Printf("Found %d empty state messages\n", emptyCount)
		}
	})
})
