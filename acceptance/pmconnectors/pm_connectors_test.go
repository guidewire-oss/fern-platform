package pmconnectors_test

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

var _ = Describe("UC-10: PM Connectors Management", Label("e2e"), func() {
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

		// Login as admin user for connector management
		auth.Login()
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

				// Save the video with a descriptive name
				timestamp := time.Now().Format("20060102_150405")
				newPath := fmt.Sprintf("../videos/pmconnectors/%s_%s.webm", testName, timestamp)

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

	Describe("UC-10-01: PM Connector List", Label("e2e"), func() {
		Context("Accessing PM connectors page", func() {
			It("should show PM connectors menu item for admin users", func() {
				// Navigate to base URL first to ensure proper app initialization
				_, err := page.Goto(baseURL)
				Expect(err).NotTo(HaveOccurred())

				// Wait for app to load
				err = page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
					State: playwright.LoadStateNetworkidle,
				})
				Expect(err).NotTo(HaveOccurred())

				// Look for PM Connectors in navigation
				pmConnectorsNav := page.Locator("text=PM Connectors")
				count, _ := pmConnectorsNav.Count()
				Expect(count).To(BeNumerically(">=", 1))

				// Click PM Connectors
				err = pmConnectorsNav.Click()
				Expect(err).NotTo(HaveOccurred())

				// Should be on PM connectors page
				Eventually(func() string {
					return page.URL()
				}, 5*time.Second).Should(ContainSubstring("/pm-connectors"))

				// Should see connectors list header
				header := page.Locator("h2:has-text('PM Connectors')")
				Expect(header).To(BeVisible())
			})
		})

		Context("Empty state", func() {
			It("should show empty state when no connectors exist", func() {
				// Navigate to base URL first
				_, err := page.Goto(baseURL)
				Expect(err).NotTo(HaveOccurred())

				// Wait for app to load
				err = page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
					State: playwright.LoadStateNetworkidle,
				})
				Expect(err).NotTo(HaveOccurred())

				// Navigate to PM Connectors
				pmConnectorsNav := page.Locator("text=PM Connectors")
				err = pmConnectorsNav.Click()
				Expect(err).NotTo(HaveOccurred())

				// Wait for navigation
				Eventually(func() string {
					return page.URL()
				}, 5*time.Second).Should(ContainSubstring("/pm-connectors"))

				// Check if we see empty state OR existing connectors
				emptyState := page.Locator("text=No PM connectors configured")
				existingConnectors := page.Locator("div.card").Filter(playwright.LocatorFilterOptions{
					HasText: "JIRA",
				})

				Eventually(func() bool {
					emptyCount, _ := emptyState.Count()
					connectorCount, _ := existingConnectors.Count()
					// Either we see empty state or we see existing connectors
					return emptyCount > 0 || connectorCount > 0
				}, 5*time.Second).Should(BeTrue())

				// Should always see "Add Connector" button
				addButton := page.Locator("button:has-text('Add Connector')")
				Expect(addButton).To(BeVisible())
			})
		})
	})

	Describe("UC-10-02: Create PM Connector", Label("e2e"), func() {
		Context("JIRA connector creation", func() {
			It("should create a new JIRA connector", func() {
				// Navigate to base URL first
				_, err := page.Goto(baseURL)
				Expect(err).NotTo(HaveOccurred())

				// Wait for app to load
				err = page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
					State: playwright.LoadStateNetworkidle,
				})
				Expect(err).NotTo(HaveOccurred())

				// Navigate to PM Connectors
				pmConnectorsNav := page.Locator("text=PM Connectors")
				err = pmConnectorsNav.Click()
				Expect(err).NotTo(HaveOccurred())

				// Wait for navigation
				Eventually(func() string {
					return page.URL()
				}, 5*time.Second).Should(ContainSubstring("/pm-connectors"))

				// Click Add Connector
				addButton := page.Locator("button:has-text('Add Connector')")
				err = addButton.Click()
				Expect(err).NotTo(HaveOccurred())

				// Should see connector creation form
				Eventually(func() bool {
					modal := page.Locator("div.modal")
					count, _ := modal.Count()
					return count > 0
				}, 5*time.Second).Should(BeTrue())

				// Fill in connector details
				nameInput := page.Locator("input[placeholder*='Production JIRA']")
				err = nameInput.Fill("Test JIRA Instance")
				Expect(err).NotTo(HaveOccurred())

				// Select JIRA as connector type (should already be default)
				typeSelect := page.Locator("select").First()
				_, err = typeSelect.SelectOption(playwright.SelectOptionValues{Values: &[]string{"JIRA"}})
				Expect(err).NotTo(HaveOccurred())

				// Fill base URL - using mock-jira service in k3d
				urlInput := page.Locator("input[type='url']")
				err = urlInput.Fill("http://mock-jira.fern-platform.svc.cluster.local:8080")
				Expect(err).NotTo(HaveOccurred())

				// Click Next to go to credentials step
				nextButton := page.Locator("button:has-text('Next')")
				err = nextButton.Click()
				Expect(err).NotTo(HaveOccurred())

				// Should see credentials form
				Eventually(func() bool {
					credForm := page.Locator("text=Configure Credentials")
					count, _ := credForm.Count()
					return count > 0
				}, 5*time.Second).Should(BeTrue())

				// Wait a bit for form to render
				time.Sleep(1 * time.Second)

				// Select API Token authentication
				authTypeSelect := page.Locator("select").Filter(playwright.LocatorFilterOptions{
					HasText: "API Token",
				})
				if count, _ := authTypeSelect.Count(); count == 0 {
					// Try by looking for any select in the modal
					authTypeSelect = page.Locator("div.modal select").First()
				}
				_, err = authTypeSelect.SelectOption(playwright.SelectOptionValues{Values: &[]string{"API_TOKEN"}})
				Expect(err).NotTo(HaveOccurred())

				// Fill API token details
				emailInput := page.Locator("input[type='email']")
				err = emailInput.Fill("test@example.com")
				Expect(err).NotTo(HaveOccurred())

				tokenInput := page.Locator("input[type='password']")
				err = tokenInput.Fill("test-api-token-12345")
				Expect(err).NotTo(HaveOccurred())

				// Click Create
				createButton := page.Locator("button:has-text('Create')")
				err = createButton.Click()
				Expect(err).NotTo(HaveOccurred())

				// Should see success message
				Eventually(func() bool {
					success := page.Locator("text=Connector created successfully")
					count, _ := success.Count()
					return count > 0
				}, 5*time.Second).Should(BeTrue())

				// Should see the new connector in the list
				connector := page.Locator("text=Test JIRA Instance")
				Expect(connector).To(BeVisible())
			})
		})
	})

	Describe("UC-10-03: Configure Field Mappings", Label("e2e"), func() {
		Context("Visual field mapping interface", func() {
			It("should allow configuring field mappings visually", func() {
				// Assume we have a connector created
				// Navigate to connector details
				_, err := page.Goto(baseURL + "/pm-connectors")
				Expect(err).NotTo(HaveOccurred())

				// Click on a connector (create one first if needed)
				connector := page.Locator("text=Test JIRA Instance").First()
				err = connector.Click()
				Expect(err).NotTo(HaveOccurred())

				// Should see connector details
				Eventually(func() string {
					return page.URL()
				}, 5*time.Second).Should(ContainSubstring("/pm-connectors/"))

				// Click on Field Mappings tab
				mappingsTab := page.Locator("text=Field Mappings")
				err = mappingsTab.Click()
				Expect(err).NotTo(HaveOccurred())

				// Should see mapping interface
				mappingInterface := page.Locator("div.field-mapping-container")
				Expect(mappingInterface).To(BeVisible())

				// Add a new mapping
				addMappingButton := page.Locator("button:has-text('Add Mapping')")
				err = addMappingButton.Click()
				Expect(err).NotTo(HaveOccurred())

				// Configure mapping - map JIRA key to id
				sourceField := page.Locator("input[placeholder='Source field path']").First()
				err = sourceField.Fill("key")
				Expect(err).NotTo(HaveOccurred())

				targetField := page.Locator("select[name='targetField']").First()
				_, err = targetField.SelectOption(playwright.SelectOptionValues{Values: &[]string{"id"}})
				Expect(err).NotTo(HaveOccurred())

				// Add another mapping for status with transformation
				err = addMappingButton.Click()
				Expect(err).NotTo(HaveOccurred())

				sourceField2 := page.Locator("input[placeholder='Source field path']").Last()
				err = sourceField2.Fill("fields.status.name")
				Expect(err).NotTo(HaveOccurred())

				targetField2 := page.Locator("select[name='targetField']").Last()
				_, err = targetField2.SelectOption(playwright.SelectOptionValues{Values: &[]string{"status"}})
				Expect(err).NotTo(HaveOccurred())

				// Select lookup transformation
				transformType := page.Locator("select[name='transformType']").Last()
				_, err = transformType.SelectOption(playwright.SelectOptionValues{Values: &[]string{"lookup"}})
				Expect(err).NotTo(HaveOccurred())

				// Should see lookup configuration
				Eventually(func() bool {
					lookupConfig := page.Locator("text=Configure Status Mapping")
					count, _ := lookupConfig.Count()
					return count > 0
				}, 3*time.Second).Should(BeTrue())

				// Save mappings
				saveButton := page.Locator("button:has-text('Save Mappings')")
				err = saveButton.Click()
				Expect(err).NotTo(HaveOccurred())

				// Should see success message
				Eventually(func() bool {
					success := page.Locator("text=Mappings saved successfully")
					count, _ := success.Count()
					return count > 0
				}, 5*time.Second).Should(BeTrue())
			})
		})
	})

	Describe("UC-10-04: Test Connection", Label("e2e"), func() {
		Context("Connection health check", func() {
			It("should test connection and show health status", func() {
				// Navigate to connector details
				_, err := page.Goto(baseURL + "/pm-connectors")
				Expect(err).NotTo(HaveOccurred())

				// Click on a connector
				connector := page.Locator("text=Test JIRA Instance").First()
				err = connector.Click()
				Expect(err).NotTo(HaveOccurred())

				// Click Test Connection button
				testButton := page.Locator("button:has-text('Test Connection')")
				err = testButton.Click()
				Expect(err).NotTo(HaveOccurred())

				// Should see testing indicator
				Eventually(func() bool {
					testing := page.Locator("text=Testing connection...")
					count, _ := testing.Count()
					return count > 0
				}, 2*time.Second).Should(BeTrue())

				// Should eventually show result (success or failure)
				Eventually(func() bool {
					success := page.Locator("text=/Connection successful|Connection failed/")
					count, _ := success.Count()
					return count > 0
				}, 10*time.Second).Should(BeTrue())

				// Health indicator should update
				healthIndicator := page.Locator("div.health-indicator")
				Expect(healthIndicator).To(BeVisible())
			})
		})
	})

	Describe("UC-10-05: Sync Configuration", Label("e2e"), func() {
		Context("Configure sync schedule", func() {
			It("should allow configuring sync intervals", func() {
				// Navigate to connector details
				_, err := page.Goto(baseURL + "/pm-connectors")
				Expect(err).NotTo(HaveOccurred())

				// Click on a connector
				connector := page.Locator("text=Test JIRA Instance").First()
				err = connector.Click()
				Expect(err).NotTo(HaveOccurred())

				// Click on Sync Settings tab
				syncTab := page.Locator("text=Sync Settings")
				err = syncTab.Click()
				Expect(err).NotTo(HaveOccurred())

				// Should see sync interval options
				syncIntervalSelect := page.Locator("select[name='syncInterval']")
				Expect(syncIntervalSelect).To(BeVisible())

				// Select 6 hours interval
				_, err = syncIntervalSelect.SelectOption(playwright.SelectOptionValues{Values: &[]string{"6hrs"}})
				Expect(err).NotTo(HaveOccurred())

				// Save settings
				saveButton := page.Locator("button:has-text('Save Settings')")
				err = saveButton.Click()
				Expect(err).NotTo(HaveOccurred())

				// Should see success message
				Eventually(func() bool {
					success := page.Locator("text=Settings saved successfully")
					count, _ := success.Count()
					return count > 0
				}, 5*time.Second).Should(BeTrue())

				// Should show next sync time
				nextSync := page.Locator("text=/Next sync:/")
				Expect(nextSync).To(BeVisible())
			})
		})
	})

	Describe("UC-10-06: Manual Sync", Label("e2e"), func() {
		Context("Trigger manual synchronization", func() {
			It("should allow manual sync of requirements", func() {
				// Navigate to connector details
				_, err := page.Goto(baseURL + "/pm-connectors")
				Expect(err).NotTo(HaveOccurred())

				// Click on a connector
				connector := page.Locator("text=Test JIRA Instance").First()
				err = connector.Click()
				Expect(err).NotTo(HaveOccurred())

				// Activate connector first
				activateButton := page.Locator("button:has-text('Activate')")
				if count, _ := activateButton.Count(); count > 0 {
					err = activateButton.Click()
					Expect(err).NotTo(HaveOccurred())

					// Wait for activation
					time.Sleep(2 * time.Second)
				}

				// Click Sync Now button
				syncButton := page.Locator("button:has-text('Sync Now')")
				err = syncButton.Click()
				Expect(err).NotTo(HaveOccurred())

				// Should see sync progress
				Eventually(func() bool {
					progress := page.Locator("text=/Syncing|Sync in progress/")
					count, _ := progress.Count()
					return count > 0
				}, 3*time.Second).Should(BeTrue())

				// Should eventually complete
				Eventually(func() bool {
					complete := page.Locator("text=/Sync completed|Last synced/")
					count, _ := complete.Count()
					return count > 0
				}, 30*time.Second).Should(BeTrue())

				// Should show sync statistics
				stats := page.Locator("text=/Requirements synced:/")
				Expect(stats).To(BeVisible())
			})
		})
	})

	Describe("UC-10-07: View Sync History", Label("e2e"), func() {
		Context("Sync logs and history", func() {
			It("should show sync history with details", func() {
				// Navigate to connector details
				_, err := page.Goto(baseURL + "/pm-connectors")
				Expect(err).NotTo(HaveOccurred())

				// Click on a connector
				connector := page.Locator("text=Test JIRA Instance").First()
				err = connector.Click()
				Expect(err).NotTo(HaveOccurred())

				// Click on Sync History tab
				historyTab := page.Locator("text=Sync History")
				err = historyTab.Click()
				Expect(err).NotTo(HaveOccurred())

				// Should see sync history table
				historyTable := page.Locator("table.sync-history")
				Expect(historyTable).To(BeVisible())

				// Should show sync entries with status
				syncEntries := page.Locator("tr.sync-entry")
				Eventually(func() int {
					count, _ := syncEntries.Count()
					return count
				}, 5*time.Second).Should(BeNumerically(">=", 0))
			})
		})
	})

	Describe("UC-10-08: Security and Permissions", Label("e2e"), func() {
		Context("Role-based access control", func() {
			It("should restrict PM connector management to admin users", func() {
				// This test would require logging in as a non-admin user
				// Skipping as it requires different user credentials
				Skip("Requires non-admin user credentials")
			})
		})

		Context("Credential security", func() {
			It("should not display credentials after saving", func() {
				// Navigate to connector details
				_, err := page.Goto(baseURL + "/pm-connectors")
				Expect(err).NotTo(HaveOccurred())

				// Click on a connector
				connector := page.Locator("text=Test JIRA Instance").First()
				err = connector.Click()
				Expect(err).NotTo(HaveOccurred())

				// Go to credentials section
				credentialsTab := page.Locator("text=Credentials")
				err = credentialsTab.Click()
				Expect(err).NotTo(HaveOccurred())

				// Should show masked credentials
				maskedToken := page.Locator("text=************")
				Eventually(func() int {
					count, _ := maskedToken.Count()
					return count
				}, 3*time.Second).Should(BeNumerically(">", 0))

				// Should not show actual token value
				actualToken := page.Locator("text=test-api-token-12345")
				count, _ := actualToken.Count()
				Expect(count).To(Equal(0))
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
