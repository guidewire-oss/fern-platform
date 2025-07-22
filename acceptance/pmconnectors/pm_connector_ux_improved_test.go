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

var _ = Describe("PM Connector Improved UX Tests", Label("e2e"), func() {
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
		// Handle cleanup and video saving
		SaveTestVideo(page, ctx, browser, recordVideo)
	})

	Describe("UC-10-01: Improved PM Connectors List View", Label("e2e"), func() {
		It("should display connectors in a table view with health indicators", func() {
			// Check for table view instead of cards
			connectorsTable := page.Locator("table.pm-connectors-table, [data-testid='pm-connectors-table']")
			Eventually(func() bool {
				count, _ := connectorsTable.Count()
				return count > 0
			}, 5*time.Second).Should(BeTrue(), "Should show table view for connectors")

			// Verify table columns
			expectedColumns := []string{"Name", "Type", "Status", "Last Sync", "Projects", "Actions"}
			for _, col := range expectedColumns {
				column := page.Locator(fmt.Sprintf("th:has-text('%s')", col))
				Expect(column).To(BeVisible())
			}

			// Check for health status indicators
			healthIndicators := page.Locator("[data-testid='health-indicator'], .health-status")
			count, _ := healthIndicators.Count()
			if count > 0 {
				// Verify health indicator shows proper status
				firstIndicator := healthIndicators.First()
				className, _ := firstIndicator.GetAttribute("class")
				Expect(className).To(SatisfyAny(
					ContainSubstring("active"),
					ContainSubstring("inactive"),
					ContainSubstring("error"),
					ContainSubstring("syncing"),
				))
			}

			// Check for relative time in Last Sync column
			lastSyncCells := page.Locator("td[data-column='last-sync'], .last-sync-time")
			if count, _ := lastSyncCells.Count(); count > 0 {
				syncText, _ := lastSyncCells.First().TextContent()
				Expect(syncText).To(SatisfyAny(
					ContainSubstring("ago"),
					ContainSubstring("Never"),
					ContainSubstring("Syncing"),
				))
			}
		})

		It("should show an engaging empty state when no connectors exist", func() {
			// This test would need to run in a clean environment
			// Check if we have connectors first
			rows := page.Locator("tbody tr")
			rowCount, _ := rows.Count()

			if rowCount == 0 {
				// Verify empty state
				emptyState := page.Locator("[data-testid='empty-state'], .empty-state")
				Expect(emptyState).To(BeVisible())

				// Check for engaging content
				Expect(page.Locator("text=Connect Your First PM Tool")).To(BeVisible())
				Expect(page.Locator("text=/Sync JIRA issues|GitHub PRs|Azure DevOps/")).To(BeVisible())

				// Verify help links
				Expect(page.Locator("a:has-text('Video Tutorial'), button:has-text('Video Tutorial')")).To(BeVisible())
				Expect(page.Locator("a:has-text('Documentation'), button:has-text('Documentation')")).To(BeVisible())
			}
		})

		It("should provide quick actions without navigation", func() {
			// Find a connector row
			connectorRow := page.Locator("tbody tr").First()

			// Check for quick action buttons
			quickActions := connectorRow.Locator("[data-testid='quick-actions'], .quick-actions")
			Expect(quickActions).To(BeVisible())

			// Verify action buttons
			testButton := connectorRow.Locator("button[title='Test Connection'], button:has-text('Test')")
			syncButton := connectorRow.Locator("button[title='Sync Now'], button:has-text('Sync')")

			// Actions should be visible without clicking menu
			Expect(testButton).To(BeVisible())
			Expect(syncButton).To(BeVisible())
		})
	})

	Describe("UC-10-02: Slide-out Panel for Connector Creation", Label("e2e"), func() {
		It("should open a slide-out panel instead of modal", func() {
			// Click Create Connection
			createButton := page.Locator("button:has-text('Create Connection'), button:has-text('Add Connector')")
			err := createButton.Click()
			Expect(err).NotTo(HaveOccurred())

			// Verify slide-out panel appears
			slidePanel := page.Locator("[data-testid='slide-panel'], .slide-panel, .drawer-right")
			Eventually(func() bool {
				count, _ := slidePanel.Count()
				return count > 0
			}, 5*time.Second).Should(BeTrue(), "Should show slide-out panel")

			// Verify panel is from the right
			Eventually(func() string {
				style, _ := slidePanel.GetAttribute("style")
				className, _ := slidePanel.GetAttribute("class")
				return style + className
			}, 2*time.Second).Should(SatisfyAny(
				ContainSubstring("right"),
				ContainSubstring("translateX"),
			))

			// Main content should still be visible but dimmed
			mainContent := page.Locator(".pm-connectors-table, [data-testid='pm-connectors-table']")
			Expect(mainContent).To(BeVisible())

			// Check for overlay/backdrop
			overlay := page.Locator(".overlay, .backdrop, [data-testid='panel-overlay']")
			Expect(overlay).To(BeVisible())

			// Verify close button
			closeButton := page.Locator(".slide-panel button[aria-label='Close'], .slide-panel button.close, .slide-panel button:has-text('✕')")
			Expect(closeButton).To(BeVisible())
		})

		It("should show connector type selection as visual cards", func() {
			// Open create panel
			createButton := page.Locator("button:has-text('Create Connection'), button:has-text('Add Connector')")
			err := createButton.Click()
			Expect(err).NotTo(HaveOccurred())

			// Wait for panel
			slidePanel := page.Locator("[data-testid='slide-panel'], .slide-panel")
			err = slidePanel.WaitFor(playwright.LocatorWaitForOptions{
				State: playwright.WaitForSelectorStateVisible,
			})
			Expect(err).NotTo(HaveOccurred())

			// Check for connector type cards
			typeCards := page.Locator("[data-testid='connector-type-card'], .connector-type-card")
			Eventually(func() int {
				count, _ := typeCards.Count()
				return count
			}, 5*time.Second).Should(BeNumerically(">=", 2), "Should show multiple connector type cards")

			// Verify JIRA card
			jiraCard := page.Locator("[data-testid='connector-type-card-jira'], .connector-type-card:has-text('JIRA')")
			Expect(jiraCard).To(BeVisible())

			// Cards should have icons and descriptions
			jiraIcon := jiraCard.Locator(".icon, [data-testid='type-icon']")
			jiraDesc := jiraCard.Locator(".description, [data-testid='type-description']")
			Expect(jiraIcon).To(BeVisible())
			Expect(jiraDesc).To(BeVisible())
		})

		It("should provide inline validation with immediate feedback", func() {
			// Open create panel
			createButton := page.Locator("button:has-text('Create Connection'), button:has-text('Add Connector')")
			err := createButton.Click()
			Expect(err).NotTo(HaveOccurred())

			// Select JIRA type
			jiraCard := page.Locator(".connector-type-card:has-text('JIRA'), [data-value='JIRA']")
			err = jiraCard.Click()
			Expect(err).NotTo(HaveOccurred())

			// Test name field validation
			nameInput := page.Locator("input[name='name'], input#connector-name")
			err = nameInput.Fill("Test")
			Expect(err).NotTo(HaveOccurred())

			// Should show validation in progress
			validationIndicator := page.Locator("[data-testid='validation-indicator'], .validation-indicator")
			Eventually(func() bool {
				count, _ := validationIndicator.Count()
				return count > 0
			}, 2*time.Second).Should(BeTrue())

			// Clear and enter valid name
			err = nameInput.Clear()
			Expect(err).NotTo(HaveOccurred())
			err = nameInput.Fill("Production JIRA Integration")
			Expect(err).NotTo(HaveOccurred())

			// Should show success indicator
			successIndicator := page.Locator("[data-testid='validation-success'], .validation-success, .field-valid")
			Eventually(func() bool {
				count, _ := successIndicator.Count()
				return count > 0
			}, 2*time.Second).Should(BeTrue())

			// Test URL validation
			urlInput := page.Locator("input[name='baseUrl'], input#base-url")
			err = urlInput.Fill("not-a-url")
			Expect(err).NotTo(HaveOccurred())

			// Should show inline error
			urlError := page.Locator("[data-testid='url-error'], .field-error:near(input[name='baseUrl'])")
			Eventually(func() bool {
				count, _ := urlError.Count()
				return count > 0
			}, 2*time.Second).Should(BeTrue())

			// Fix URL
			err = urlInput.Clear()
			Expect(err).NotTo(HaveOccurred())
			err = urlInput.Fill("https://company.atlassian.net")
			Expect(err).NotTo(HaveOccurred())

			// Error should disappear
			Eventually(func() int {
				count, _ := urlError.Count()
				return count
			}, 2*time.Second).Should(Equal(0))
		})

		It("should show live connection test with detailed progress", func() {
			// Create a connector and reach the test stage
			createButton := page.Locator("button:has-text('Create Connection'), button:has-text('Add Connector')")
			err := createButton.Click()
			Expect(err).NotTo(HaveOccurred())

			// Fill minimum required fields
			timestamp := time.Now().Format("20060102_150405")
			connectorName := fmt.Sprintf("UX Test JIRA %s", timestamp)

			// Select JIRA
			jiraOption := page.Locator("[data-value='JIRA'], .connector-type-card:has-text('JIRA')")
			err = jiraOption.Click()
			Expect(err).NotTo(HaveOccurred())

			// Fill details
			nameInput := page.Locator("input[name='name'], input#connector-name")
			err = nameInput.Fill(connectorName)
			Expect(err).NotTo(HaveOccurred())

			urlInput := page.Locator("input[name='baseUrl'], input#base-url")
			err = urlInput.Fill("http://mock-jira.fern-platform.svc.cluster.local:8080")
			Expect(err).NotTo(HaveOccurred())

			// Select auth method
			authSelect := page.Locator("select[name='authType'], [data-testid='auth-method']")
			_, err = authSelect.SelectOption(playwright.SelectOptionValues{Values: &[]string{"API_TOKEN"}})
			Expect(err).NotTo(HaveOccurred())

			// Fill credentials
			emailInput := page.Locator("input[type='email'], input[name='email']")
			err = emailInput.Fill("test@example.com")
			Expect(err).NotTo(HaveOccurred())

			tokenInput := page.Locator("input[type='password'], input[name='apiToken']")
			err = tokenInput.Fill("test-api-token-12345")
			Expect(err).NotTo(HaveOccurred())

			// Click Test Connection
			testButton := page.Locator("button:has-text('Test Connection')")
			err = testButton.Click()
			Expect(err).NotTo(HaveOccurred())

			// Should show live test progress
			testProgress := page.Locator("[data-testid='test-progress'], .connection-test-progress")
			Eventually(func() bool {
				count, _ := testProgress.Count()
				return count > 0
			}, 3*time.Second).Should(BeTrue(), "Should show test progress panel")

			// Verify progress stages
			stages := []string{"Connecting", "Authenticating", "Permissions", "Sample Data"}
			for _, stage := range stages {
				stageElement := page.Locator(fmt.Sprintf("[data-testid='test-stage-%s'], .test-stage:has-text('%s')",
					strings.ToLower(stage), stage))
				Eventually(func() bool {
					count, _ := stageElement.Count()
					return count > 0
				}, 5*time.Second).Should(BeTrue(), fmt.Sprintf("Should show %s stage", stage))
			}

			// Should show final result
			testResult := page.Locator("[data-testid='test-result'], .test-result")
			Eventually(func() bool {
				count, _ := testResult.Count()
				return count > 0
			}, 10*time.Second).Should(BeTrue(), "Should show test result")
		})
	})

	Describe("UC-10-03: Visual Field Mapping Interface", Label("e2e"), func() {
		It("should provide drag-and-drop field mapping", func() {
			// This test would need a connector to be created first
			// Find or create a connector
			connectorRow := page.Locator("tbody tr").First()

			// Click on configure field mappings
			mappingsButton := connectorRow.Locator("button:has-text('Field Mappings'), button[title='Configure Mappings']")
			if count, _ := mappingsButton.Count(); count == 0 {
				// Try from actions menu
				actionsMenu := connectorRow.Locator("button[data-testid='actions-menu'], button.actions-menu")
				err := actionsMenu.Click()
				Expect(err).NotTo(HaveOccurred())

				mappingsButton = page.Locator("button:has-text('Field Mappings'), [role='menuitem']:has-text('Field Mappings')")
			}

			err := mappingsButton.Click()
			Expect(err).NotTo(HaveOccurred())

			// Should show visual mapping interface
			mappingInterface := page.Locator("[data-testid='field-mapping-interface'], .field-mapping-visual")
			Eventually(func() bool {
				count, _ := mappingInterface.Count()
				return count > 0
			}, 5*time.Second).Should(BeTrue(), "Should show visual mapping interface")

			// Verify left panel (PM tool fields)
			leftPanel := page.Locator("[data-testid='source-fields'], .mapping-source-fields")
			Expect(leftPanel).To(BeVisible())

			// Verify right panel (Fern fields)
			rightPanel := page.Locator("[data-testid='target-fields'], .mapping-target-fields")
			Expect(rightPanel).To(BeVisible())

			// Check for draggable fields
			draggableFields := leftPanel.Locator("[draggable='true'], .draggable-field")
			Eventually(func() int {
				count, _ := draggableFields.Count()
				return count
			}, 3*time.Second).Should(BeNumerically(">", 0), "Should have draggable fields")

			// Check for auto-suggested mappings (dashed lines)
			suggestedMappings := page.Locator("[data-testid='suggested-mapping'], .mapping-line.suggested")
			count, _ := suggestedMappings.Count()
			if count > 0 {
				// Verify dashed line style
				firstMapping := suggestedMappings.First()
				style, _ := firstMapping.GetAttribute("style")
				className, _ := firstMapping.GetAttribute("class")
				Expect(style + className).To(SatisfyAny(
					ContainSubstring("dashed"),
					ContainSubstring("suggested"),
				))
			}
		})

		It("should show mapping preview with sample data", func() {
			// Assuming we're in the mapping interface
			// Look for any existing mapping
			mappingLine := page.Locator("[data-testid='field-mapping'], .mapping-line.active")
			if count, _ := mappingLine.Count(); count > 0 {
				// Click on the mapping line
				err := mappingLine.First().Click()
				Expect(err).NotTo(HaveOccurred())

				// Should show preview
				preview := page.Locator("[data-testid='mapping-preview'], .mapping-preview")
				Eventually(func() bool {
					count, _ := preview.Count()
					return count > 0
				}, 3*time.Second).Should(BeTrue(), "Should show mapping preview")

				// Preview should show transformation
				sourceValue := preview.Locator(".preview-source, [data-testid='preview-source']")
				targetValue := preview.Locator(".preview-target, [data-testid='preview-target']")
				arrow := preview.Locator(".preview-arrow, :has-text('→')")

				Expect(sourceValue).To(BeVisible())
				Expect(targetValue).To(BeVisible())
				Expect(arrow).To(BeVisible())
			}
		})
	})

	Describe("UC-12-01: Non-blocking Connection Testing", Label("e2e"), func() {
		It("should test connection without blocking the UI", func() {
			// Find a connector
			connectorRow := page.Locator("tbody tr").First()

			// Click quick test action
			testButton := connectorRow.Locator("button[title='Test Connection'], button:has-text('Test')")
			err := testButton.Click()
			Expect(err).NotTo(HaveOccurred())

			// UI should remain interactive
			// Try to hover over another row
			secondRow := page.Locator("tbody tr").Nth(1)
			err = secondRow.Hover()
			Expect(err).NotTo(HaveOccurred(), "UI should remain interactive during test")

			// Test indicator should update in place
			testIndicator := connectorRow.Locator("[data-testid='test-indicator'], .testing-indicator")
			Eventually(func() bool {
				count, _ := testIndicator.Count()
				return count > 0
			}, 2*time.Second).Should(BeTrue(), "Should show testing indicator")

			// Health status should update when done
			healthStatus := connectorRow.Locator("[data-testid='health-indicator'], .health-status")
			Eventually(func() string {
				className, _ := healthStatus.GetAttribute("class")
				return className
			}, 10*time.Second).Should(SatisfyAny(
				ContainSubstring("active"),
				ContainSubstring("error"),
			), "Health status should update after test")
		})
	})
})

// Helper function to save video with proper naming
func SaveTestVideo(page playwright.Page, ctx playwright.BrowserContext, browser playwright.Browser, recordVideo bool) {
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
			newPath := filepath.Join("videos", fmt.Sprintf("%s_PM_UX_%s.webm", timestamp, testName))

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
}
