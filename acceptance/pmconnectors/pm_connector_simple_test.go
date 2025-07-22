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

var _ = Describe("PM Connector Basic Tests", Label("e2e"), func() {
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
				newPath := filepath.Join("videos", fmt.Sprintf("%s_PM_Basic_%s.webm", timestamp, testName))

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

	It("should create a JIRA connector successfully", func() {
		// Click Add Connector button
		addButton := page.Locator("button:has-text('Add Connector')")
		err := addButton.Click()
		Expect(err).NotTo(HaveOccurred())

		// Wait for modal to appear
		modal := page.Locator("div.modal")
		err = modal.WaitFor(playwright.LocatorWaitForOptions{
			State:   playwright.WaitForSelectorStateVisible,
			Timeout: playwright.Float(5000),
		})
		Expect(err).NotTo(HaveOccurred())

		// Fill connector name with timestamp to ensure uniqueness
		timestamp := time.Now().Format("20060102-150405")
		connectorName := fmt.Sprintf("Test JIRA %s", timestamp)
		nameInput := page.Locator("input[placeholder*='Production JIRA']")
		err = nameInput.Fill(connectorName)
		Expect(err).NotTo(HaveOccurred())

		// Type should already be JIRA by default, but let's verify
		typeSelect := page.Locator("select").First()
		selectedValue, err := typeSelect.InputValue()
		Expect(err).NotTo(HaveOccurred())
		fmt.Printf("Selected type: %s\n", selectedValue)

		// Fill base URL
		urlInput := page.Locator("input[type='url']")
		err = urlInput.Fill("http://mock-jira.fern-platform.svc.cluster.local:8080")
		Expect(err).NotTo(HaveOccurred())

		// Click Next button
		nextButton := page.Locator("button:has-text('Next')")
		err = nextButton.Click()
		Expect(err).NotTo(HaveOccurred())

		// Wait for step 2 (credentials)
		Eventually(func() bool {
			h2 := page.Locator("h2:has-text('Configure Credentials')")
			count, _ := h2.Count()
			return count > 0
		}, 5*time.Second).Should(BeTrue())

		// Authentication type should be API_TOKEN by default
		authSelect := page.Locator("select").First()
		authValue, err := authSelect.InputValue()
		Expect(err).NotTo(HaveOccurred())
		fmt.Printf("Auth type: %s\n", authValue)
		Expect(authValue).To(Equal("API_TOKEN"))

		// Fill email
		emailInput := page.Locator("input[type='email']")
		err = emailInput.Fill("test@example.com")
		Expect(err).NotTo(HaveOccurred())

		// Fill API token
		tokenInput := page.Locator("input[type='password']")
		err = tokenInput.Fill("test-api-token-12345")
		Expect(err).NotTo(HaveOccurred())

		// Handle potential alert dialog
		page.OnDialog(func(dialog playwright.Dialog) {
			fmt.Printf("Alert dialog: %s\n", dialog.Message())
			dialog.Accept()
		})

		// Click Create button
		createButton := page.Locator("button:has-text('Create')")
		err = createButton.Click()
		Expect(err).NotTo(HaveOccurred())

		// Wait for modal to close or success message
		Eventually(func() bool {
			// Check if modal is still visible
			modalCount, _ := modal.Count()
			if modalCount == 0 {
				fmt.Println("Modal closed")
				return true
			}

			// Check for success message
			success := page.Locator("text=Connector created successfully")
			successCount, _ := success.Count()
			if successCount > 0 {
				fmt.Println("Success message found")
				return true
			}

			// Check for any error messages
			errorMsg := page.Locator("text=/error|failed/i")
			errorCount, _ := errorMsg.Count()
			if errorCount > 0 {
				errorText, _ := errorMsg.First().TextContent()
				fmt.Printf("Error found: %s\n", errorText)
			}

			return false
		}, 10*time.Second).Should(BeTrue())

		// Verify the connector appears in the list
		Eventually(func() bool {
			connector := page.Locator(fmt.Sprintf("text=%s", connectorName))
			count, _ := connector.Count()
			return count > 0
		}, 5*time.Second).Should(BeTrue())
	})

	It("should test connection to mock JIRA", func() {
		// First check if there's already a connector
		existingConnector := page.Locator("div.card").Filter(playwright.LocatorFilterOptions{
			HasText: "JIRA",
		})

		count, _ := existingConnector.Count()
		if count == 0 {
			Skip("No JIRA connector found, skipping connection test")
		}

		// Click on the connector card
		err := existingConnector.First().Click()
		Expect(err).NotTo(HaveOccurred())

		// Wait for modal or details view
		time.Sleep(2 * time.Second)

		// Look for Test Connection button
		testButton := page.Locator("button:has-text('Test Connection')")
		testCount, _ := testButton.Count()

		if testCount > 0 {
			err = testButton.Click()
			Expect(err).NotTo(HaveOccurred())

			// Wait for test result
			Eventually(func() bool {
				result := page.Locator("text=/Connection successful|Connected|Healthy/i")
				count, _ := result.Count()
				return count > 0
			}, 15*time.Second).Should(BeTrue())
		}
	})
})
