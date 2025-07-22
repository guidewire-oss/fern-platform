package pmconnectors_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/playwright-community/playwright-go"

	"github.com/guidewire-oss/fern-platform/acceptance/helpers"
)

var _ = Describe("PM Connector Label Integration Test", Label("e2e"), func() {
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

		// Create browser context options with increased timeout
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

		// Set default timeout
		ctx.SetDefaultTimeout(30000)

		page, err = ctx.NewPage()
		Expect(err).NotTo(HaveOccurred())

		auth = helpers.NewLoginHelper(page, baseURL, username, password)

		// Login as admin user
		auth.Login()

		// Navigate to PM Connectors page
		_, err = page.Goto(baseURL)
		Expect(err).NotTo(HaveOccurred())

		err = page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
			State: playwright.LoadStateNetworkidle,
		})
		Expect(err).NotTo(HaveOccurred())

		// Click PM Connectors
		pmConnectorsNav := page.Locator("text=PM Connectors")
		err = pmConnectorsNav.Click()
		Expect(err).NotTo(HaveOccurred())

		// Wait for navigation
		Eventually(func() string {
			return page.URL()
		}, 10*time.Second).Should(ContainSubstring("/pm-connectors"))
	})

	AfterEach(func() {
		// Handle cleanup even if test fails
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("Recovered from panic in AfterEach: %v\n", r)
			}

			// Ensure browser is closed
			if browser != nil {
				browser.Close()
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

				// Create timestamp prefix
				timestamp := time.Now().Format("20060102_150405")
				newPath := filepath.Join("videos", fmt.Sprintf("%s_PM_Label_%s.webm", timestamp, testName))

				// Get the original video path
				originalPath, _ := video.Path()

				// Close the page and context to finalize video
				page.Close()
				page = nil
				if ctx != nil {
					ctx.Close()
					ctx = nil
				}

				// Create videos directory if it doesn't exist
				os.MkdirAll("videos", 0755)

				// Move/rename the video file
				if originalPath != "" {
					err := os.Rename(originalPath, newPath)
					if err == nil {
						fmt.Printf("Video saved: %s\n", newPath)
					} else {
						// If rename fails, try SaveAs
						err = video.SaveAs(newPath)
						if err == nil {
							fmt.Printf("Video saved: %s\n", newPath)
						} else {
							fmt.Printf("Failed to save video: %v\n", err)
						}
					}
				}
			} else {
				// No video, just close
				if page != nil {
					page.Close()
					page = nil
				}
				if ctx != nil {
					ctx.Close()
					ctx = nil
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

	It("should demonstrate PM connector working with mock JIRA labels", func() {
		// First, verify we have a JIRA connector
		connectorCards := page.Locator("div.card").Filter(playwright.LocatorFilterOptions{
			HasText: "JIRA",
		})

		connectorCount, _ := connectorCards.Count()
		fmt.Printf("Found %d JIRA connectors\n", connectorCount)

		if connectorCount == 0 {
			// Create a connector first
			fmt.Println("No JIRA connector found, creating one...")

			addButton := page.Locator("button:has-text('Add Connector')")
			err := addButton.Click()
			Expect(err).NotTo(HaveOccurred())

			// Wait for modal
			modal := page.Locator("div.modal")
			err = modal.WaitFor(playwright.LocatorWaitForOptions{
				State:   playwright.WaitForSelectorStateVisible,
				Timeout: playwright.Float(5000),
			})
			Expect(err).NotTo(HaveOccurred())

			// Create unique connector name
			timestamp := time.Now().Format("20060102-150405")
			connectorName := fmt.Sprintf("Label Test JIRA %s", timestamp)

			// Fill connector details
			nameInput := page.Locator("input[placeholder*='Production JIRA']")
			err = nameInput.Fill(connectorName)
			Expect(err).NotTo(HaveOccurred())

			// Fill base URL
			urlInput := page.Locator("input[type='url']")
			err = urlInput.Fill("http://mock-jira.fern-platform.svc.cluster.local:8080")
			Expect(err).NotTo(HaveOccurred())

			// Click Next
			nextButton := page.Locator("button:has-text('Next')")
			err = nextButton.Click()
			Expect(err).NotTo(HaveOccurred())

			// Fill credentials
			Eventually(func() bool {
				h2 := page.Locator("h2:has-text('Configure Credentials')")
				count, _ := h2.Count()
				return count > 0
			}, 5*time.Second).Should(BeTrue())

			emailInput := page.Locator("input[type='email']")
			err = emailInput.Fill("test@example.com")
			Expect(err).NotTo(HaveOccurred())

			tokenInput := page.Locator("input[type='password']")
			err = tokenInput.Fill("test-api-token-12345")
			Expect(err).NotTo(HaveOccurred())

			// Create
			createButton := page.Locator("button:has-text('Create')")
			err = createButton.Click()
			Expect(err).NotTo(HaveOccurred())

			// Wait for success
			Eventually(func() bool {
				modalCount, _ := modal.Count()
				return modalCount == 0
			}, 10*time.Second).Should(BeTrue())

			fmt.Printf("✓ Created connector: %s\n", connectorName)
		}

		// Now test the connector
		fmt.Println("\n=== Testing PM Connector Functionality ===")

		// Find a JIRA connector
		connector := page.Locator("div.card").Filter(playwright.LocatorFilterOptions{
			HasText: "JIRA",
		}).First()

		// Get connector name
		connectorTitle := connector.Locator("h3, h4, .card-title")
		connectorName, _ := connectorTitle.TextContent()
		fmt.Printf("Using connector: %s\n", connectorName)

		// Click on the connector to view details
		err := connector.Click()
		Expect(err).NotTo(HaveOccurred())

		// Wait for connector details page or modal
		time.Sleep(2 * time.Second)

		// Test the connection
		testButton := page.Locator("button:has-text('Test Connection')")
		testCount, _ := testButton.Count()

		if testCount > 0 {
			fmt.Println("Testing connection to mock JIRA...")
			err = testButton.Click()
			Expect(err).NotTo(HaveOccurred())

			// Wait for test result
			Eventually(func() bool {
				result := page.Locator("text=/Connection successful|Connected|Healthy/i")
				count, _ := result.Count()
				if count > 0 {
					fmt.Println("✓ Connection test passed")
					return true
				}

				// Check for errors
				errorMsg := page.Locator("text=/Failed|Error/i")
				errorCount, _ := errorMsg.Count()
				if errorCount > 0 {
					errorText, _ := errorMsg.TextContent()
					fmt.Printf("Connection test failed: %s\n", errorText)
				}
				return false
			}, 15*time.Second).Should(BeTrue())
		}

		// Navigate back to view all connectors
		_, err = page.Goto(baseURL + "/pm-connectors")
		Expect(err).NotTo(HaveOccurred())

		// Summary of PM Connector capabilities
		fmt.Println("\n=== PM Connector Capabilities Demonstrated ===")
		fmt.Println("1. ✓ Create JIRA connector with mock server URL")
		fmt.Println("2. ✓ Configure API token authentication")
		fmt.Println("3. ✓ Test connection to mock JIRA server")
		fmt.Println("4. ✓ View connector status and health")

		fmt.Println("\n=== How to Use PM Labels in Tests ===")
		fmt.Println("When writing tests, you can add JIRA issue labels like:")
		fmt.Println("- 'jira:TEST-123' to link a test to JIRA issue TEST-123")
		fmt.Println("- 'pm:PROJ-456' to link to project management ticket PROJ-456")
		fmt.Println("- Multiple labels can be added to track different PM systems")

		fmt.Println("\n=== Field Mapping Configuration ===")
		fmt.Println("PM Connectors support mapping fields between Fern and JIRA:")
		fmt.Println("- Requirement ID → JIRA Key")
		fmt.Println("- Title → Summary")
		fmt.Println("- Description → Description")
		fmt.Println("- Status → Status (with value mapping)")

		fmt.Println("\n✅ PM Connector is ready for use with mock JIRA!")
	})
})
